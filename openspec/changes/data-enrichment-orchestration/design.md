# Design — data-enrichment-orchestration

> 把 PoC（`tests/data_enrichment_poc/`）验证的三角色架构升级为**持久话题的认知闭环系统**。
>
> 核心原则：
> - 数据源是 agent 工具，不是订阅流；
> - **新闻记忆与分析认知分离**——两个独立循环，通过表 1 单向连接，互不污染；
> - 分析消费**分层上下文**（周/月/年/总 + 14 天详情），不是单篇新闻；
> - 持久话题分析是**不断自我修正的认知过程**，不是一次性快照。

## 设计目标

让持久话题的演进分析能按需补充实时数据（ETF 行情起步），判断"最新进展在演进中的意义"。系统维护两类记忆：**新闻事实记忆**（客观，只随新闻变）与**分析认知演进**（主观，随每次分析迭代），两者隔离。不制造第二个话题泳道，不污染持久话题主数据。

## 0. 架构总览：两个独立循环 + 三表认知闭环

```
═════════════════════════════════════════════════════════════════
循环 A：新闻记忆循环（独立 · 纯新闻 · 不碰分析）
═════════════════════════════════════════════════════════════════

   话题的历史 sections（新闻原文）
            │ 按 granularity 聚合（本周/本月/本年/全部）
            ▼
   ┌─────────────────────────────────────┐
   │ LLM 纯新闻汇总（发生了什么+数据波动） │  Operation: data_enrichment.summarize_context
   └────────────┬────────────────────────┘
                ▼
   ┌─────────────────────────────────────────────────────┐
   │ 表1  topic_lifeline_context                          │
   │   week/month/year/all + as_of_date                   │
   │   ★ 只随新闻更新，分析永远不回写                       │
   └─────────────────────────────────────────────────────┘
                │ 单向喂给（背景）
                ▼
═════════════════════════════════════════════════════════════════
循环 B：分析认知循环（增强驱动 · 自我迭代）
═════════════════════════════════════════════════════════════════

   表1 context  ─────┐
   14天实时详情 ──────┼──▶  三角色增强  ──▶  表2 result（新快照）
   历史表3 review ───┘                      │  (不可变)
                                            ▼
                                 LLM review judge（半自动）
                                 Operation: data_enrichment.review_judge
                                 输入: 上次result + 本次result
                                 输出: JSON {should_review, reason,
                                          deviation_summary, ...}
                                            │ 值得才生成（带理由，非字段diff）
                                            ▼
                                 ┌──────────────────────────┐
                                 │ 表3 review（追加）        │
                                 │  分析认知演进史            │
                                 │  ★ 不回写表1              │
                                 │  ★ applied=true 后被     │
                                 │    下次增强读取（避免重蹈）│
                                 └────────────┬─────────────┘
                                              │
                                              └─▶ 回到循环B入口（自我迭代）
```

**三表关注点分离**（生命周期/关联关系都不同，必须独立）：

| 表 | 角色 | 生命周期 | 可变？ |
|---|---|---|---|
| `topic_lifeline_context` | 新闻记忆（背景） | 滚动更新，按周期 | 可（循环A刷新/人工编辑） |
| `topic_enrichment_result` | 当下判断（快照） | 一次分析一行 | **不可变**（否则没法对比） |
| `topic_enrichment_review` | 两次快照间的增量（反思） | 追加 | deviation_summary 可人工调 |

类比人的认知：**记住过去（表1）→ 形成判断（表2）→ 反思对比（表3）→ 下次判断更准（读历史 review）**。三者缺一不可，但表 3 永远不污染表 1（新闻事实保持客观）。

## 1. 数据源与板块配置层

### 1.1 board_data_sources 表

```sql
CREATE TABLE board_data_sources (
  id           BIGSERIAL PRIMARY KEY,
  semantic_board_id BIGINT NOT NULL REFERENCES semantic_boards(id) ON DELETE CASCADE,
  source_type  VARCHAR(40) NOT NULL,   -- etf_quote / exchange_rate / gdelt_event
  config       JSONB NOT NULL DEFAULT '{}',  -- 板块级参数，如 {"keywords":["半导体"],"default_codes":["512480"]}
  enabled      BOOLEAN NOT NULL DEFAULT true,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (semantic_board_id, source_type)
);
```

`config` 的 schema 由 source_type 决定。同一 source_type 在不同 board 下 config 不同——这是"板块级数据源"的本质。

### 1.2 数据源工具注册表（tool registry）

`internal/dataenrichment/registry.go`，对标 PoC `tools.py` 的 TOOL_REGISTRY：

```go
type Tool struct {
    Name        string
    Description string
    InputSchema map[string]any
    Execute     func(ctx context.Context, args map[string]any) (string, error)
}
```

首批内置工具（对标 PoC，免费 HTTP，Go `net/http` 直调东方财富/新浪）：
- `list_etf_by_keyword`：全市场 ETF 按关键词过滤（全量缓存避免重复翻页）
- `get_etf_quote`：实时行情（对标新浪 `hq.sinajs.cn`）
- `list_sectors`：东财行业板块清单

工具返回**全部命中**（不截断条目，只精简字段）——PoC 踩过的坑：截断会导致 agent 误判"没拿全"而死循环。

### 1.3 板块级配置项（新增）

板块编辑扩展为 tab 式（基础/数据源/分析），「分析」tab 配置：

| 项 | 默认 | 说明 |
|---|---|---|
| `enrichment_enabled` | false | 循环B增强开关（耗资源，需先绑数据源） |
| `window_days` | 14 | 循环B实时详情窗口（消费 14 天 sections） |
| `context_layers` | `["week","month","year","all"]` | 解读员读哪些层（year/all 未生成则跳过；用户可关掉年/总以省 token） |

**循环 A（新闻汇总）不配板块级开关**——它是新闻汇总基础设施，全局定时跑。只有循环 B 的三项放板块配置。

## 2. 分层新闻汇总上下文（循环 A）

### 2.1 topic_lifeline_context 表（新增）

```sql
CREATE TABLE topic_lifeline_context (
  id                  BIGSERIAL PRIMARY KEY,
  persistent_topic_id BIGINT NOT NULL REFERENCES board_persistent_topics(id) ON DELETE CASCADE,
  granularity         VARCHAR(10) NOT NULL,  -- week / month / year / all
  content             TEXT NOT NULL,          -- 新闻叙事汇总 + 数据波动快照
  as_of_date          DATE NOT NULL,          -- 汇总截止日（时效判断 + 检查自愈依据）
  source              VARCHAR(12) NOT NULL DEFAULT 'manual', -- manual / llm_assisted
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (persistent_topic_id, granularity)
);
```

**滚动起步**（每类一条当前快照，`UNIQUE(topic_id, granularity)`）。`as_of_date` 标注汇总截止日，解读员/分析员据此判断时效（旧汇总可能滞后于最新进展）。

### 2.2 触发：定时 + 检查自愈 + 手动

- **定时**：week 每周一次、month 每月一次、year 每年一次。循环 B 实时已有 14 天详情，循环 A 只负责"沉淀"更早的历史，不必每天跑。
- **检查自愈**：定时任务跑时顺带扫描所有活跃 topic 的 context 缺口（topic 新建从没生成过、宕机漏跑的周/月、`as_of_date` 滞后超过阈值）→ 补生成。合并进定时逻辑，非独立巡检。
- **手动触发**：CRUD 界面能手动重生成任意 granularity（用户改完新闻想立即刷新）。

### 2.3 汇总算法

- **week**：直接读"最近 7 天 sections"重算。数据量小（实测单 topic 14 天 ≤13 sections），重算最稳，**不走增量合并**（例外）。
- **month / year / all**：**增量 + 旧汇总合并**——读「自上次汇总以来的增量 sections」+「该 granularity 旧汇总」→ LLM 合并生成新汇总。避免每月重读全年、每年重读多年。

不搞 week→month→year 层层金字塔合并（误差会累积）；各 granularity 平行维护自己的滚动窗口。

## 3. 三角色编排（循环 B 核心，演进版）

`internal/dataenrichment/service/orchestrator.go`，对标 PoC `roles_evolved.py`。**入口消费分层上下文，不是单 lifeline，更不是单篇 news**。

### 3.1 角色① 解读员：全层读

```
输入:
  - 表1 context（按板块 context_layers 配置取 week/month/year/all，未生成的层跳过）
  - 14天窗口详情（GetTopicLifeline 渲染：topic本体 + 逐日section + status + thread标题）
  - 历史 applied review（避免重蹈已知偏差，如"会谈=缓和"的线性错误）
全层读 ~2.5-3k token（为未来换大模型留余量，本地 Qwen3-9B 也能扛）
输出: JSON {topics: [{topic, reason}]}  —— 需补数据的产业方向
```

14 天详情渲染（对标 PoC `lifeline_mock.render_lifeline_for_agent`）：`GetTopicLifeline` 返回 sections + relations + 推导 status，补一次 join `daily_report_threads` 取每节 top-2 thread title（缺口见 `daily_report_repository.go:697-710`）。

### 3.2 角色② 查询员（agent loop，核心）

对标 PoC `research_topic_evolved`。极简 agent loop，**每主题独立跑**：

```
for step in 1..maxLoops:
    喂 (system含工具描述 + 主题背景, user含主题+历史调用) → airouter.Chat
        (Operation="data_enrichment.tool_use", SessionID=编排id)
    解析返回 JSON: {action: call_tool|finish, tool, args, thought}
    if call_tool:
        去重检查(相同tool+args直接拦截)  # PoC 死循环防御
        execute_tool → 结果加进历史(完整不截断)  # PoC 死循环防御
        命中0时换宽泛词重查（如"光刻机"→"半导体"，"避险"→"黄金"）
    if finish: 记 summary, break
```

**查询员不带分层上下文**（只带主题+工具描述）——省 token，6 轮循环不重复消耗 2.5k 背景。

**关键防御（PoC 验证过的三个坑，必须内置）**：
1. **Qwen3 thinking 处理**：`chat_template_kwargs: {enable_thinking: false}`，否则 thinking 烧光 token 导致 content 空。
2. **结果不截断**：历史累积给完整结果，否则 agent 误判"没拿全"死循环重查。
3. **去重拦截**：相同 tool+args 直接返回错误提示，避免无限重查。

### 3.3 角色③ 分析员：全层 + 数据

```
输入: 表1 context + 14天详情 + 查询员行情数据
输出: JSON {evolution_assessment, sectors:[{sector, evolution_role, current_signal,
         vs_history, judgment, confidence}], causal_chain, overall}
```

判断"最新进展在演进中的定位（强化/转折/扩散）"，引用表1对应层作对比基准（如"vs 6月冲突峰值仍低8%"）。`as_of_date` 滞后时以 14 天详情为准（近期优先）。

## 4. 分析认知循环：review judge

### 4.1 topic_enrichment_result 表（快照，不可变）

```sql
CREATE TABLE topic_enrichment_result (
  id              BIGSERIAL PRIMARY KEY,
  persistent_topic_id BIGINT NOT NULL REFERENCES board_persistent_topics(id) ON DELETE CASCADE,
  report_id       BIGINT REFERENCES board_daily_reports(id) ON DELETE CASCADE,
  evolution_assessment TEXT,
  sectors         JSONB,        -- 分析员结论
  causal_chain    TEXT,
  tool_calls      JSONB,        -- 工具调用记录（名/参数/返回摘要/耗时）
  session_id      VARCHAR(120), -- 关联 ai_call_logs，便于回放
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**不可变快照**——每次增强一行，存档不修改（否则 review 没法对比）。

### 4.2 topic_enrichment_review 表（新增）

```sql
CREATE TABLE topic_enrichment_review (
  id                  BIGSERIAL PRIMARY KEY,
  persistent_topic_id BIGINT NOT NULL REFERENCES board_persistent_topics(id) ON DELETE CASCADE,
  prev_result_id      BIGINT REFERENCES topic_enrichment_result(id) ON DELETE CASCADE,
  curr_result_id      BIGINT NOT NULL REFERENCES topic_enrichment_result(id) ON DELETE CASCADE,
  deviation_summary   TEXT NOT NULL,        -- 为什么变了（LLM基底+人工可调）
  affected_context    VARCHAR(10),          -- 建议关注哪层（week/month/year/all）
  confidence          REAL,
  applied             BOOLEAN NOT NULL DEFAULT false,  -- 用户采纳标记
  source              VARCHAR(12) NOT NULL DEFAULT 'llm_assisted',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.3 review judge：半自动对比

增强跑完产 result #N 后，触发一次额外 LLM 调用：

```
Operation: data_enrichment.review_judge
输入: result #(N-1)（上次）+ result #N（本次）
输出 JSON:
  { should_review: true|false,
    reason: "...",                 // 为什么值得/不值得 review（语义判断，非字段diff）
    deviation_summary: "...",      // 偏差说明（为什么变了）
    affected_context: "month",     // 建议关注哪层
    confidence: 0.8 }
should_review=true 才写 review 行（避免噪音）；false 跳过。
```

**第一次增强无 prev_result** → 跳过 review，等第二次才有对比。

### 4.4 applied 语义（关键：不回写表1）

`applied=true` **不触发回写表1**。表1 永远只随循环 A 新闻汇总变（保持新闻事实客观）。`applied` 仅标记"这条认知已被纳入"，下次增强（解读员）会读历史 applied review，避免重蹈已知偏差（如"上次因 X 误判黄金跌，这次别再犯"）。review 在自己的轨道上迭代，跟新闻记忆隔离。

## 5. 日报管线插入点 + 手动触发

遵循 TopicWatch 只读覆盖层范式（`daily_report_watch.go`）：

- 日报 `SaveReport` 事务**外**、报告存完后跑（失败只 Warnf 不阻断）。
- 对当期活跃 topic 跑演进版增强，结果写 `topic_enrichment_result`（独立表，不改 section.persistent_topic_id，不动 topic 状态）。
- **手动触发**：CRUD 界面支持"重新分析这个话题"——用户点按钮即跑一次增强（不依赖日报管线）。
- 调度：可配置开关（默认关，板块启用 `enrichment_enabled` 后才跑）。

## 6. 可观测性

**前置依赖已落地**（代码层 `ai_call_logs` 表 + `airouter/store.go` 写日志 + `SessionIDFromContext` 在 `daily_report_watch.go:29` 就绪），不阻塞开工。原 proposal 声明强依赖 ai-call-logging-schema 前置就绪，该声明已过时——功能已具备。

所有 LLM 调用经 airouter 带 `Operation` + `SessionID`：

| 调用 | Operation |
|---|---|
| 解读员 | `data_enrichment.interpret` |
| 查询员每轮 | `data_enrichment.tool_use` |
| 分析员 | `data_enrichment.analyze` |
| review judge | `data_enrichment.review_judge` |
| 循环A汇总 | `data_enrichment.summarize_context` |

- SessionID（循环B）：`data_enrichment_{topic_id}_{uuid8}`，一次增强内所有调用共享。
- SessionID（循环A）：`lifeline_context_{topic_id}_{granularity}_{uuid8}`。
- 工具调用（非 LLM）记 jsonb 存 `topic_enrichment_result.tool_calls`（跟增强结果绑定）。

## 7. 前端：板块 tab 下的 CRUD 界面（第一版）

板块详情页现有 tab（板块内容/日报/文章）旁新增「数据增强」栏，三表 CRUD：

| 表 | 查看 | 编辑 | 触发 |
|---|---|---|---|
| 表1 context | ✓ week/month/year/all | ✓ 人工修正 content | ✓ 手动重生成某 granularity |
| 表2 result | ✓ 只读（含 LLM 调用 trace） | ✗ 不可改 | ✓ 手动触发增强 |
| 表3 review | ✓ 认知演进史 | ✓ 人工调整 deviation_summary | （随增强自动产） |

**渐进策略**：第一版 CRUD 界面验证功能无误后，再重构侦探墙（`TopicDetectiveWall.client.vue`）为"话题增强分析中枢"（脉络/上下文/数据增强三 tab）。侦探墙重构单列后续 change，本 change 只交付 CRUD。

**契约稳定性**：CRUD 的数据契约（API 形状）设计成侦探墙能直接复用，避免替换时返工。

## 8. 风险与取舍

- **性能**：单 topic 增强 10-30s（PoC 实测），日报对 N 个活跃 topic 串行会慢。缓解：topic 间并发跑（无依赖）；本地 LLM 并发受限于模型实例，初期串行可接受。
- **token 量级（实测校准）**：14 天窗口单 topic 14天详情 ~1.5k token；解读员全层读（含 week/month/year/all + 14天 + 历史 review）~2.5-3k token。Qwen3-9B 上下文 32k+ 可扛；未来换大模型余量充足。原 proposal "5天1.5k token" 是 PoC mock 估算，已校准。
- **工具返回体积**：全市场 ETF 1500+ 条。缓解：只返精简字段（代码/名称/涨跌幅），PoC 实测可控。
- **本地模型能力边界**：Qwen3-9B 复杂编排（多主题多工具）可能不稳。缓解：max_loops=6 + 去重拦截兜底。目前本地验证为主，后续换大模型。
- **数据源稳定性**：东方财富/新浪非官方接口。缓解：工具层隔离，单接口失败返回 error 不阻断编排。
- **循环A时效裂缝**：month/year 汇总可能滞后于最新进展。缓解：所有 context 带 `as_of_date`，分析员据此判断时效，矛盾时以 14 天详情为准。

## 9. 不做什么

- **不做侦探墙重构**（第一版先 CRUD，验证后单列 change 重构侦探墙）。
- 不做 GDELT（金融数据起步，GDELT 与话题泳道定位重复）。
- 不改持久话题演进主链路（只读覆盖层，不动 split/merge/dual-confirmation）。
- 不把数据落 article 表（数据是 agent 查询上下文，不是订阅流）。
- **不让分析结果回写新闻记忆**（表1 纯新闻，表3 认知独立迭代）。

## 10. 完整流转实例（2026-07-05 topic 1「美伊协议」）

前置：表1 week（as_of 06-28）+ month（as_of 06-30）；表2 result #2（07-01，"局势暂时缓和，原油承压"）；表3 review #1（applied=true）。

```
日报管线触发增强 → session_id=data_enrichment_1_a1b2c3d4
[解读员] 全层读表1+14天详情+历史review → 提炼查"美伊停火进展""避险联动"
[查询员] 主题1: list_etf("原油")→get_quote; 主题2: list_etf("避险")命中0→换"黄金"→get_quote
[分析员] 输出 result #3: "再趋紧张,原油强化(+4.2%),黄金强化(+1.8%)"
[review_judge] 对比 #2(缓和/承压) vs #3(紧张/强化): should_review=true
  → "核心判断反转,上次'会谈=缓和'过于线性,本次以军威胁+海峡争议打破预期"
  → 写 review #2 (applied=false)

此时: 表1 没变(循环B不碰); 表2 +result#3; 表3 +review#2
用户在CRUD: 看 review#2 → 调 deviation_summary → 采纳(applied=true, 不回写表1)
07-07循环A定时: week重算(as_of→07-07), 表2/3不动
```
