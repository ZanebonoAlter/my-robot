# 概要设计 — data-enrichment-orchestration

> 持久话题的**认知闭环系统**：两个独立循环 + 三表关注点分离。
> 本文用 mermaid 辅助说明**顺序、状态、表结构依赖**。完整决策见 `design.md`，字段约束见 `specs/data-enrichment/spec.md`。

---

## 1. 架构总览

两个循环通过 `topic_lifeline_context`（表1）**单向连接**——循环 A 只产新闻记忆，循环 B 消费它并自我迭代，但 review 永远不回写表1（保持新闻事实客观）。

```mermaid
flowchart TB
    subgraph CycleA[循环 A · 新闻记忆循环 纯新闻 定时+自愈]
        A1[(话题 sections<br/>新闻原文)] --> A2[LLM 汇总<br/>summarize_context]
        A2 --> T1[(topic_lifeline_context<br/>week / month / year / all)]
    end

    subgraph CycleB[循环 B · 分析认知循环 仅手动触发]
        B0[手动触发<br/>CRUD界面 重新分析]
        B0 --> B1[三角色增强<br/>解读→查询→分析]
        B1 --> T2[(topic_enrichment_result<br/>快照 不可变)]
        T2 --> B2[review judge<br/>半自动对比]
        B2 --> T3[(topic_enrichment_review<br/>认知演进史)]
        T3 -. 读历史 applied .-> B1
    end

    T1 -. 单向喂给 背景 .-> B1
    T3 -.->|❌ 永不回写| T1

    style T1 fill:#e8f5e9,stroke:#2e7d32
    style T2 fill:#fff3e0,stroke:#ef6c00
    style T3 fill:#fce4ec,stroke:#c62828
```

**为什么循环 B 仅手动**：不是所有板块对金融数据有影响（如"开发工具"板块），自动挂日报管线会产生无意义增强 + 浪费 LLM 成本。用户判断需要时手动点。

**三表关注点分离**：

| 表 | 角色 | 可变？ | 谁写 |
|---|---|---|---|
| `topic_lifeline_context` | 新闻记忆（背景） | 可（滚动/人工） | 循环 A |
| `topic_enrichment_result` | 当下判断（快照） | **不可变** | 循环 B 分析员 |
| `topic_enrichment_review` | 两次快照间增量（反思） | deviation_summary 可调 | review judge / 用户手动 |

---

## 2. 表结构依赖（ER 图）

4 张新表 + 关联的 3 张已有表。`topic_enrichment_result` 新增 `input_snapshot` 字段（记录本次增强读了哪些输入，可追溯）。

```mermaid
erDiagram
    semantic_boards ||--o{ board_data_sources : "1:N 绑定数据源"
    board_persistent_topics ||--o{ topic_lifeline_context : "1:N 按 granularity"
    board_persistent_topics ||--o{ topic_enrichment_result : "1:N 快照"
    board_persistent_topics ||--o{ topic_enrichment_review : "1:N 反思"
    topic_enrichment_result ||--o{ topic_enrichment_review : "prev_result 可空"
    topic_enrichment_result ||--o{ topic_enrichment_review : "curr_result"

    board_data_sources {
        bigserial id PK
        bigint semantic_board_id FK
        varchar source_type "etf_quote等 CHECK"
        jsonb config "板块级参数"
        boolean enabled "默认true"
    }
    topic_lifeline_context {
        bigserial id PK
        bigint persistent_topic_id FK
        varchar granularity "week_month_year_all"
        text content "新闻叙事+数据波动"
        date as_of_date "截止日 自愈依据"
        varchar source "manual_llm_assisted"
    }
    topic_enrichment_result {
        bigserial id PK
        bigint persistent_topic_id FK
        text evolution_assessment
        jsonb sectors
        jsonb tool_calls "工具调用记录"
        jsonb input_snapshot "编排元数据 可追溯"
        varchar session_id "关联ai_call_logs"
    }
    topic_enrichment_review {
        bigserial id PK
        bigint persistent_topic_id FK
        bigint prev_result_id FK "可空 手动批注时"
        bigint curr_result_id FK
        text deviation_summary
        varchar source "llm_assisted_manual"
        boolean applied "默认false"
    }
```

**注意**：`topic_enrichment_result` 移除了 `report_id`（循环B不再挂日报管线）；`prev_result_id` 可空（支持用户手动批注，无 prev 对比）。

---

## 3. 循环 A：新闻汇总（顺序图）

定时触发（周/月/年）+ 检查自愈（**补遗漏的周期，非覆盖当前**）+ 手动重生成。

```mermaid
sequenceDiagram
    participant Cron as 定时任务
    participant Svc as 汇总 Service
    participant DB as DB
    participant AI as airouter

    Cron->>Svc: 触发(周/月/年) + 检查自愈扫描
    Svc->>DB: 查 as_of_date 滞后的 topic
    DB-->>Svc: 待补清单(含 as_of_date)

    loop 每个 topic 的每个 granularity
        Note over Svc: 从 as_of_date 次周期起逐块补<br/>补的是遗漏周期 非覆盖当前
        alt 首次(无旧汇总)
            Svc->>DB: 读该周期全部 sections
        else 已有旧汇总(增量合并)
            Svc->>DB: 读该周期 sections + 旧汇总
        end
        DB-->>Svc: sections 或 增量+旧汇总
        Svc->>AI: Chat(Operation=summarize_context)
        AI-->>Svc: 汇总文本
        Svc->>DB: UPSERT context<br/>as_of_date 推进到该周期末
    end
```

**关键**：循环 A 只读 sections，**不读** result/review；表1 永远只随它变。隔周再触发时，自愈会先补上周遗漏的 week，再补本周，`as_of_date` 顺序推进。

---

## 4. 循环 B：分析认知（顺序图，含 2026-07-05 topic 1 实例）

```mermaid
sequenceDiagram
    autonumber
    participant Trig as 手动触发<br/>CRUD界面
    participant Interp as ①解读员
    participant AgentLoop as ②查询员<br/>agent loop  <-- 这里修改了名称
    participant Tool as 数据源工具
    participant Ana as ③分析员
    participant Rev as review_judge
    participant AI as airouter
    participant DB as DB

    Trig->>Interp: EnrichTopic topicID=1<br/>session=data_enrichment_1_a1b2c3d4

    Interp->>DB: 读表1 context week+month<br/>+ 14天详情 + 历史 applied review
    DB-->>Interp: 分层上下文 约2.5k token
    Interp->>AI: Chat Operation=interpret
    AI-->>Interp: topics = 美伊停火进展, 避险联动

    loop 每主题 max_loops=6
        AgentLoop->>AI: Chat Operation=tool_use<br/>enable_thinking=false <-- 这里同步修改引用
        AI-->>AgentLoop: action=call_tool tool+args
        alt call_tool
            AgentLoop->>AgentLoop: 去重检查 相同tool+args拦截
            AgentLoop->>Tool: execute args
            Tool-->>AgentLoop: 完整结果 不截断
            Note over AgentLoop: 命中0则换宽泛词<br/>避险 换 黄金
        end
    end

    Ana->>AI: Chat Operation=analyze<br/>输入 分层上下文+行情
    AI-->>Ana: assessment=再趋紧张<br/>原油强化 黄金强化
    Ana->>DB: INSERT result #3<br/>含 tool_calls+input_snapshot

    Rev->>DB: 读上次 result #2 07-01 缓和原油承压
    DB-->>Rev: prev_result
    Rev->>AI: Chat Operation=review_judge<br/>对比 #2 vs #3
    AI-->>Rev: should_review=true<br/>reason=核心判断反转<br/>deviation=上次会谈等于缓和过于线性
    Rev->>DB: INSERT review #2 applied=false

    Note over DB: 表1 没变 循环B不碰<br/>用户CRUD采纳后 applied=true 仍不回写表1<br/>用户也可手动写review批注 source=manual
```

---

## 5. 状态机

### 5.1 review 的 applied 生命周期（含手动批注）

```mermaid
stateDiagram-v2
    [*] --> 无对比基础 : 第一次增强<br/>无 prev_result
    无对比基础 --> [*] : 跳过自动 review

    [*] --> 待采纳 : review_judge 判定<br/>should_review=true
    待采纳 --> 已采纳 : 用户点采纳<br/>applied=true
    待采纳 --> 已忽略 : 用户不采纳 / 噪音

    [*] --> 已采纳 : 用户手动批注<br/>source=manual<br/>applied默认true
    已采纳 --> [*]
    已忽略 --> [*]

    note right of 已采纳
        ★ 不回写 topic_lifeline_context
        ★ 仅标记认知已纳入
        ★ 下次增强解读员会读 applied review
    end note
```

### 5.2 context 的滚动状态

```mermaid
stateDiagram-v2
    [*] --> 缺失 : 新 topic
    缺失 --> 最新 : 首次生成(定时/手动)
    最新 --> 滞后 : 时间推移<br/>as_of_date 超阈值
    滞后 --> 最新 : 自愈逐周期补齐 / 手动
    最新 --> 最新 : 手动重生成<br/>人工修正 content
    滞后 --> [*] : topic 归档
```

**`as_of_date` 的双重作用**：解读员/分析员据此判断时效（滞后时以 14 天详情为准）；检查自愈据此扫描缺口并逐周期补齐。

---

## 6. 关键约束速查

| 约束 | 说明 | 落地点 |
|---|---|---|
| **单向不回写** | review 永远不写 `topic_lifeline_context` | review_judge.go |
| **快照不可变** | `topic_enrichment_result` 写入后不修改 | repository |
| **循环B仅手动** | 不挂日报管线，CRUD 手动触发 | handler |
| **自愈补遗漏** | 补 as_of_date 之后的遗漏周期，非覆盖当前 | lifeline_context.go |
| **thinking 关闭** | 本地 Qwen3 请求带 `enable_thinking=false` | airouter 请求层 |
| **历史不截断** | agent loop 累积完整工具结果 | orchestrator |
| **去重拦截** | 相同 tool+args 直接挡 | orchestrator |
| **可追溯** | 每次切片输入输出都可查（见 §7） | ai_call_logs + result |

---

## 7. 可追溯性（变更大，强化）

每次增强的所有切片 SHALL 可查、可重建：

| 切片 | 输入 | 输出 | 存哪 |
|---|---|---|---|
| 解读员 | 分层上下文（rendered） | topics JSON | `ai_call_logs`（messages+content） |
| 查询员每轮 | 主题+工具+历史 | action JSON | `ai_call_logs` |
| 工具调用 | args | 返回结果+耗时 | `result.tool_calls` jsonb |
| 分析员 | 分层+行情 | assessment/sectors | `ai_call_logs` + `result` |
| review_judge | prev+curr result | should_review JSON | `ai_call_logs` + `review` |
| 编排元数据 | 读的context层/as_of/section范围 | — | `result.input_snapshot` jsonb |

**排查路径**：`result.session_id` → `GET /api/ai/call-logs?session_id=` 重建该次增强的全部 LLM 调用（输入+输出）；`result.tool_calls` + `result.input_snapshot` 补齐工具与输入上下文；三表持久化全部中间结论。

---

## 8. 5 个 Operation 速查

| Operation | 循环 | 角色 | 触发 |
|---|---|---|---|
| `data_enrichment.summarize_context` | A | 汇总 | 定时/自愈/手动 |
| `data_enrichment.interpret` | B | 解读员 | 手动增强 |
| `data_enrichment.tool_use` | B | 查询员每轮 | 手动增强 |
| `data_enrichment.analyze` | B | 分析员 | 手动增强 |
| `data_enrichment.review_judge` | B | review 对比 | 增强后 |

SessionID 规则：循环 B = `data_enrichment_{topic_id}_{uuid8}`；循环 A = `lifeline_context_{topic_id}_{granularity}_{uuid8}`。
