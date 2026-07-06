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
  period              VARCHAR(12) NOT NULL,  -- 具体周期：'2026-W27' / '2026-06' / '2026' / 'all'（按粒度格式化）
  content             TEXT NOT NULL,          -- 新闻叙事汇总 + 数据波动快照
  as_of_date          DATE NOT NULL,          -- 汇总截止日（时效判断 + 检查自愈依据）
  source              VARCHAR(12) NOT NULL DEFAULT 'manual', -- manual / llm_assisted
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (persistent_topic_id, granularity, period)
);

-- 保留策略（定时任务清理超期行，非 UNIQUE 约束）：
--   week：近 8 周 ｜ month：近 12 月 ｜ year：全留 ｜ all：滚动一条（聚合压缩层）
```

**档案式存储（非滚动覆盖）**：每个 `(granularity, period)` 一条，历史周期独立保留可翻（如 2026-06、2026-05 各一条，不再被新周期合并覆盖）。`UNIQUE(topic_id, granularity, period)` 保证同周期不重复。`as_of_date` 标注汇总截止日，解读员/分析员据此判断时效。

> **设计修正（2026-07-06）**：原设计 `UNIQUE(topic_id, granularity)` 每类只留一条当前快照、新周期合并覆盖旧的——用户反馈要能翻历史（"5月可能也有信息"），改为 period 维度独立存档。

### 2.2 触发：定时 + 检查自愈 + 归档 + 手动

- **定时**：week 每周一次、month 每月一次、year 每年一次，**每个周期各产一条独立行**（新周期不覆盖旧周期）。循环 B 实时已有 14 天详情，循环 A 只负责"沉淀"历史，不必每天跑。
- **检查自愈**：定时任务扫描活跃 topic 缺失的历史 period（如该 topic 从未生成过 2026-05 的 month 汇总）→ **按 period 逐个补**，`as_of_date` 顺序推进。补的是遗漏的历史周期，不是覆盖当前周期。合并进定时逻辑，非独立巡检。
- **归档清理**：超保留策略的旧行（week > 8 周、month > 12 月）由定时任务归档/删除。
- **手动触发**：前端能重生成任意 period（用户改完新闻想立即刷新，或补某个历史周期）。

### 2.3 汇总算法

**每个 period 独立汇总成一条**（不再"增量合并覆盖旧汇总"）：

- **week / month / year**：读「该 period 范围内的 sections」→ LLM 汇总成一条。如 2026-06 的 month 汇总 = 6 月所有 sections 一次性汇总，5 月的 month 汇总独立存在不被覆盖。
- **all**：例外，滚动一条（`period='all'` 始终单行），读「全部历史 sections」+ 旧 all 汇总增量合并，作为全局压缩视图。
- **重生成某 period**：直接重读该 period 的 sections 重算（不依赖旧汇总）。

不搞 week→month→year 层层金字塔合并（误差累积）；各 period 平行独立维护。避免每月重读全年（只读当月 sections）。

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

### 3.3 角色③ 分析员：全层 + 数据 + 走向预测

```
输入: 表1 context（当前周期 + 历史 period）+ 14天详情 + 查询员行情数据 + 历史 applied review
输出: JSON {
  evolution_assessment,                                   // 一句话演进判断
  sectors: [{
    sector,                                               // 板块名
    direction: "up" | "down" | "flat" | "unknown",        // 走向（替 evolution_role）
    confidence: 0.0-1.0,
    horizon: "short" | "mid" | "long",                    // 短期1-2周 / 中期1-3月 / 长期
    reasoning: [{signal, mechanism}],                     // 凭什么：新闻信号 → 传导机制
    evidence: [{context_id, period, quote}],              // 原始依据：引用哪段 context 的原话（证据链）
    supporting_data,                                      // 支撑数据（涨跌幅等）
    trigger_up: [...], trigger_down: [...]                // 板块专属触发条件（给 review 兑现度对照）
  }],
  causal_chain, overall
}
```

**关键扩展（相对 PoC，2026-07-06）**：
- `direction + confidence + horizon + triggers`：从"描述发生了什么"升级为"判断往哪走 + 什么信号验证/证伪"，让 review 能做兑现度复盘（§4.3）。
- `reasoning + evidence`：每个判断带**可追溯证据链**——`evidence.context_id` 指回具体 context 行 + 原话摘录。前端 tooltip 悬停显示原话（**原地展示，不跳转**，避免分散注意力）。
- 边界：只到**板块方向 + 置信度 + 触发条件**，不下沉到个股买卖建议（合规 + 数据源只到 ETF 行情）。
- 判断"最新进展在演进中的定位（强化/转折/扩散）"，引用历史 period 作对比基准（如"vs 6月冲突峰值仍低8%"）。`as_of_date` 滞后时以 14 天详情为准（近期优先）。

### 3.4 个股深度辩论（外部 FinGenius，可选环节，2026-07-06 新增）

分析员输出 `sectors` 时顺带给每个板块的**代表标的池**（`symbols` 字段，不带买卖建议）。用户可对感兴趣的标的发起**个股深度辩论**——交给外部 FinGenius（6 角色 agent 多轮辩论 + 投票），Syntopica 作 HTTP 客户端调用，结果存库。

**分析员 prompt 扩展**（§3.3 的 sectors schema 加 `symbols`）：
```json
"sectors": [{
  "sector":"原油","direction":"up","confidence":0.8,"horizon":"short",
  "reasoning":[...],"evidence":[...],"trigger_up":[...],"trigger_down":[...],
  "symbols":[                              // 新增：板块代表标的池（2-4 个）
    {"code":"161129","name":"易方达原油","kind":"etf"},
    {"code":"601857","name":"中国石油","kind":"leader_stock"}
  ]
}]
```
- `kind`：`etf`（ETF，跟踪指数）/ `leader_stock`（龙头股）。只标"值得关注的代表标的"，**不判断买卖**（合规）。
- 解析透传（`sectors` 是 `[]map[string]any`，symbols 子字段自动进入，无需改解析逻辑）。

**编排接入点**：`EnrichTopic` 在分析员（§3.3）返回后、写 result 前，**不自动**调 FinGenius。个股辩论是**独立的可选步骤**，由前端④「开始辩论」按钮单独触发（见 §11 决策⑥触发时机），调独立端点 `POST .../results/:id/debates`。

**为什么独立而不串进主流程**：① FinGenius 辩论 2-3 分钟，串进主流程会拖慢循环B；② 辩论失败不应阻塞板块方向预测；③ 用户只对感兴趣的标的辩论，不必全跑。

**详见 §11 决策⑥**（GPL v3 合规边界 + 客户端契约 + 配置 + 降级策略）。

## 4. 分析认知循环：review judge

### 4.1 topic_enrichment_result 表（快照，不可变）

```sql
CREATE TABLE topic_enrichment_result (
  id              BIGSERIAL PRIMARY KEY,
  persistent_topic_id BIGINT NOT NULL REFERENCES board_persistent_topics(id) ON DELETE CASCADE,
  evolution_assessment TEXT,
  sectors         JSONB,        -- 分析员结论（见 §3.3：含 direction/confidence/horizon/reasoning/evidence/trigger）
  causal_chain    TEXT,
  tool_calls      JSONB,        -- 工具调用记录（名/参数/返回摘要/耗时）
  input_snapshot  JSONB,        -- 编排元数据（读的context层/period/as_of/section范围/引用review）可追溯
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
  verdict             JSONB,        -- 兑现度逐条结算：[{sector, predicted_dir, actual, mark: "hit"|"part"|"miss"}]
  deviation_summary   TEXT NOT NULL,        -- 复盘说明（预测为什么对/错，LLM基底+人工可调）
  affected_context    VARCHAR(12),          -- 建议关注哪个 period（如 '2026-06'）
  confidence          REAL,
  applied             BOOLEAN NOT NULL DEFAULT false,  -- 用户采纳标记（纳入下一轮解读）
  source              VARCHAR(12) NOT NULL DEFAULT 'llm_assisted',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.2b stock_debate_result 表（FinGenius 辩论结果，2026-07-06 新增）

存 FinGenius 多角色辩论的输出，按 `(result_id, sector, code)` 维度 append-only（同一 result 内可多次辩论，最新为准）。

```sql
CREATE TABLE stock_debate_result (
  id                      BIGSERIAL PRIMARY KEY,
  topic_enrichment_result_id BIGINT NOT NULL REFERENCES topic_enrichment_result(id) ON DELETE CASCADE,
  persistent_topic_id     BIGINT NOT NULL REFERENCES board_persistent_topics(id) ON DELETE CASCADE,  -- 冗余，便于按 topic 查
  sector                  VARCHAR(80) NOT NULL,        -- 关联 analyze 输出的 sector 名
  code                    VARCHAR(20) NOT NULL,        -- 标的代码（如 161129）
  name                    VARCHAR(60),                 -- 标的名称
  -- ↓↓ 提炼后字段（Syntopica LLM debate_distill 产出，前端④渲染用）↓↓
  verdict                 VARCHAR(8) NOT NULL,         -- up / down / flat（综合结论）
  consensus               VARCHAR(12),                 -- 共识度文本（如 "4/6"、"2/6 分歧"）
  agents                  JSONB,                       -- [{role, stance:up/down/flat, note, raw_vote:bullish/bearish}]
  votes                   JSONB,                       -- {up:N, flat:N, down:N} 提炼后三档统计
  -- ↓↓ FinGenius 原始输出（保底，提炼失败时降级展示原文）↓↓
  fingenius_research      JSONB,                       -- {sentiment:"...", risk:"...", ...} 6 段文本
  fingenius_battle        JSONB,                       -- {final_decision, vote_count, final_votes, debate_history, ...}
  fingenius_task_id       VARCHAR(120),                -- 异步任务 id（可追溯轮询过程）
  distill_status          VARCHAR(12) NOT NULL DEFAULT 'done',  -- done/failed/skipped（提炼状态）
  html_content            TEXT,                        -- FinGenius 完整 HTML 报告字符串（result.html_content，非 URL）
  created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_stock_debate_result_id ON stock_debate_result(topic_enrichment_result_id);
CREATE INDEX idx_stock_debate_topic ON stock_debate_result(persistent_topic_id);
```

**字段分工**：
- **提炼后字段**（`verdict`/`consensus`/`agents`/`votes`）：Syntopica LLM `debate_distill` 产出，前端④正常渲染用。
- **原始字段**（`fingenius_research`/`fingenius_battle`）：FinGenius 原始输出，提炼失败（`distill_status=failed`）时降级展示原文。
  - `fingenius_research`：7 段——6 段分析文本（`sentiment`/`risk`/`hot_money`/`technical`/`chip_analysis`/`big_deal`）+ `basic_info`（基本面快照）。
  - `fingenius_battle`：8 字段——`final_decision`/`vote_count`/`final_votes`/`vote_results`/`debate_history`/`battle_highlights`/`agent_order`/`debate_rounds`。
- **`html_content`**：完整 HTML 报告字符串（非 URL），前端④「查看完整报告」用 iframe `srcdoc` 渲染。

**不回写表1/表2/表3**——辩论结果独立存档，是循环B的可选增强产物，不污染新闻记忆、分析快照、认知演进史。前端④区块直接读此表渲染。

### 4.3 review judge：预测兑现度复盘（半自动）

增强跑完产 result #N 后，触发一次额外 LLM 调用，**对照上次预测 vs 期间实际走势**：

```
Operation: data_enrichment.review_judge
输入: result #(N-1)（上次，含 prev.sectors[].direction/triggers）+ result #N（本次）+ 期间实际行情
输出 JSON:
  { should_review: true|false,
    reason: "...",                 // 是否值得复盘（核心判断有无翻转/兑现）
    verdict: [{sector, predicted_dir, actual, mark}],   // 逐板块兑现度结算（hit/part/miss）
    deviation_summary: "...",      // 预测为什么对/错（如"会谈=缓和"线性错误）
    affected_context: "2026-06",   // 建议关注哪个 period
    confidence: 0.8 }
should_review=true 才写 review 行（避免噪音）；false 跳过。
```

**从"描述对比"升级为"兑现度复盘"（2026-07-06）**：不再比"上次描述 vs 本次描述"（语义漂浮），而是比"上次预测方向 vs 期间实际走势"，每个 sector 结算 hit/part/miss。这让认知闭环真正闭合——复盘结论（哪个信号导致误判）喂给下一轮解读。

**第一次增强无 prev_result** → 跳过自动 review。**prev 无 direction 字段**（旧格式 result）→ 降级为描述对比。

**用户手动批注**：用户可在前端手动写 review（`source='manual'`，`prev_result_id` 可空，`deviation_summary` 用户填，`applied` 默认 true）。

### 4.4 applied 语义（关键：不回写表1）

`applied=true` **不触发回写表1**。表1 永远只随循环 A 新闻汇总变（保持新闻事实客观）。`applied` 仅标记"这条认知已被纳入"，下次增强（解读员）会读历史 applied review，避免重蹈已知偏差（如"上次因 X 误判黄金跌，这次别再犯"）。review 在自己的轨道上迭代，跟新闻记忆隔离。

## 5. 手动触发（仅手动，不挂日报管线）

**循环 B 仅手动触发**——不是所有板块对金融数据产生影响（如"开发工具"板块），自动挂日报管线会产生无意义增强 + 浪费 LLM 成本。用户在 CRUD 界面点"重新分析某话题"才跑。

- 触发：CRUD 界面按钮，不依赖日报管线。
- 结果写 `topic_enrichment_result`（独立表，不含 report_id，不改 section.persistent_topic_id，不动 topic 状态）。
- 调度：板块级 `enrichment_enabled` 开关（默认关）。
- 失败处理：增强失败只记日志告警，手动触发天然隔离，无"阻断日报"问题。

> 设计余量：将来若要加回日报管线自动触发，可作为可选开关扩展，不影响当前手动路径。

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
| 个股辩论结果提炼 | `data_enrichment.debate_distill`（2026-07-06 新增，归 `data_enrichment_analysis` capability） |

- SessionID（循环B）：`data_enrichment_{topic_id}_{uuid8}`，一次增强内所有调用共享。
- SessionID（循环A）：`lifeline_context_{topic_id}_{granularity}_{uuid8}`。
- 工具调用（非 LLM）记 jsonb 存 `topic_enrichment_result.tool_calls`（跟增强结果绑定）。

**可追溯性（变更大，强化）**：所有切片的输入输出 SHALL 可查：
- 每次 LLM 调用的输入（messages）+ 输出（content）→ `ai_call_logs`（按 session_id 重建）。
- 每次工具调用的参数 + 返回摘要 + 耗时 → `topic_enrichment_result.tool_calls`。
- 编排元数据（本次增强读了哪些 context 层 + as_of、14 天详情的 section 范围、引用的历史 review id）→ `topic_enrichment_result.input_snapshot`（新增 jsonb 字段）。
- 三表本身（context/result/review）持久化全部中间结论。

排查路径：`result.session_id` → `GET /api/ai/call-logs?session_id=` 重建该次增强的全部 LLM 调用；`result.tool_calls` + `result.input_snapshot` 补齐工具与输入上下文。

## 7. 前端：板块 tab 下的「认知工作台」（第一版）

板块详情页新增「数据增强」tab，按**用户认知任务流**组织（不再按四张表平铺 CRUD）。原型见 `prototype/enrichment-workbench.html`（双主题 HTML 原型，已迭代多轮验证交互方向）。

### 7.1 四步认知循环结构

顶部 sticky 导航条 + 步骤间承上启下引导条，让用户看清这是一个循环（不是四个孤立板块）：

| 步骤 | 区块 | 对应表 | 核心交互 |
|---|---|---|---|
| ① 最近怎么了 | 新闻记忆 | context | **周期筛选器翻历史** + 结构化叙事段落 inline 编辑 |
| ② 会往哪走 | 走向预测 | result.sectors | **板块可展开卡片**（凭什么/数据/触发）+ tooltip 证据链 |
| ③ 猜得准吗 | 预测兑现复盘 | review | 时间轴 + **逐条兑现结算**（hit/part/miss）+ 采纳 |
| ④ 数据源/参数 | 板块配置 | board_data_sources | 折叠高级区 |

### 7.2 关键交互（原型验证过的方向）

- **周期筛选器（替四档缩放尺）**：粒度（周/月/年）下拉 + 具体 period 翻页（‹ 2026-06 ›），每周期独立可翻（吃 §2.1 的 period 字段）。
- **板块可展开卡片**：每个 sector 默认折叠，展开后含「凭什么判断（信号→机制）+ 支撑数据 + 板块专属触发条件」。对应 §3.3 的 reasoning/supporting_data/trigger。
- **证据链 tooltip（不跳转）**：判断信号/支撑数据悬停显示引用的新闻原话（来自 `evidence.quote`），**原地 tooltip 展示，不跳转**（跳转会分散注意力，用户反馈）。新闻段落标「被 N 个判断引用」徽章。
- **预测兑现复盘**：每条历史预测对照实际走势，hit/part/miss 一目了然（吃 §4.3 的 verdict），复盘结论标「已喂给下一轮解读」。
- **完整交互态**：loading/empty/error 都要有（不再只有成功态）。

### 7.3 术语翻译（用户友好）

界面禁用后端术语（granularity / session_id / evolution_assessment / deviation_summary / applied 等），统一用人话：周/月/年 + 具体周期、本次分析的调用记录、一句话结论、涉及的板块、来龙去脉、这次为什么跟上次不一样、采纳/未采纳。

**渐进策略**：第一版工作台验证功能后，再重构侦探墙（`TopicDetectiveWall.client.vue`）为"话题增强分析中枢"，侦探墙重构单列后续 change。

**契约稳定性**：API 形状设计成侦探墙能直接复用，避免替换时返工。

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
- **不复制/翻译 FinGenius 源码进 Syntopica**（GPL v3 合规边界，见 §11 决策⑥）：FinGenius 只作独立 HTTP 服务黑盒调用，Syntopica 仓库一行 FinGenius 代码都不引入。
- **不在本 change 实现 FinGenius 服务端改造**：本 change 只定义客户端契约（§11 决策⑥），FinGenius 的 FastAPI 服务壳由另起独立项目实现。
- **不把个股辩论挂自动管线**：手动触发（前端④按钮），避免 2-3 分钟辩论拖慢循环B主分析；辩论失败 non-fatal，不阻塞板块方向预测。

## 10. 完整流转实例（2026-07-05 topic 1「美伊协议」）

前置：表1 week（as_of 06-28）+ month（as_of 06-30）；表2 result #2（07-01，"局势暂时缓和，原油承压"）；表3 review #1（applied=true）。

```
手动触发增强（CRUD界面"重新分析"） → session_id=data_enrichment_1_a1b2c3d4
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

## 11. Apply 阶段决策（2026-07-05）

> 进入 apply 阶段，以下 5 个决策细化/修正了上述设计，**实现以本节为准**。

### 决策①：enable_thinking 走 DB provider 配置，不扩 ChatRequest

**修正 §3.5 / §6 / spec 的措辞**：原文"airouter 请求层带 `enable_thinking=false`"易被误读为 per-request 字段。实际机制（`openai_compatible.go:206` `buildPayload`）：payload **始终**发 `chat_template_kwargs.enable_thinking = provider.EnableThinking`，该值来自 `ai_providers` 表的 DB 配置，`ChatRequest` **无此字段**。

**结论**：domain 代码**不做任何特殊请求处理**，照常用 `airouter.Router.Chat`。由 ops 在 DB 里把 `data_enrichment_analysis` / `data_enrichment_news` 路由到的 provider 配 `enable_thinking=false`，并在 `docs/reference/configuration.md` 记一笔。spec 验证节的 `grep enable_thinking` 校验改为"airouter 层始终发送该字段"（已在 `openai_compatible.go:206` 满足）+ domain 代码无重复处理。

### 决策②：拆两条 airouter Capability

- `data_enrichment_news` → 循环 A（`summarize_context`，纯新闻汇总，量大，可配便宜模型）
- `data_enrichment_analysis` → 循环 B（`interpret` / `tool_use` / `analyze` / `review_judge`）

5 个 Operation 归这两条 capability。新增 capability 常量在 dataenrichment 内定义（值字符串），DB `ai_routes` 表需 seed 两条路由（运维任务，文档记一笔）。

### 决策③：循环 A 墙钟调度时刻（Asia/Shanghai，避开 daily_report）

- `week`：每周一 03:00
- `month`：每月 1 号 03:30
- `year`：每年 1 月 1 号 04:00

实现走 `scheduler.Config.NextRun func(now) time.Time`（参考 `NextDailyReportTime`）。在 `internal/dataenrichment/scheduler_next_run.go` 导出 `NextWeeklyLifelineTime` / `NextMonthlyLifelineTime` / `NextYearlyLifelineTime`。三个 scheduler 在 `runtime.go` 注册。检查自愈逻辑合并进每个 Job 的开头（扫 `as_of_date` 滞后 topic，逐周期补）。

### 决策④：板块配置字段加到 `SemanticLabel` model（即 `semantic_labels` 表）

> **修正 design §1.3 / proposal 的措辞**：原设计假设存在 `semantic_boards` 表，但本 codebase 里"板块"就是 `SemanticLabel`（表 `semantic_labels`，靠 `LabelType` 区分），不存在 `semantic_boards` 表。前端 `SemanticBoard` 类型对应后端 `models.SemanticLabel`，路由 `/semantic-boards` 在 `tagmanagement/handler/board_crud_handler.go`。

`EnrichmentEnabled bool`(默认 false) / `WindowDays int`(默认 14) / `ContextLayers []string`(默认 `["week","month","year","all"]`) 三列加到 `models.SemanticLabel`（`semantic_labels` 表）。GORM model 加 tag，AutoMigrate 自动加列。

### 决策⑤：工具层接口抽象，mock fetcher 走 TDD

三个数据源工具（`list_etf_by_keyword` / `get_etf_quote` / `list_sectors`）抽出一个 `HTTPFetcher` 接口（`Fetch(ctx, url, headers) (body []byte, err error)`），生产实现用 `net/http`，单测注入 mock fetcher。全量 ETF 缓存用 `sync.Once` 单例（进程级）。

**这样 TDD 全程不依赖外部网络/8085**，门禁绿。真实连通冒烟（调东方财富/新浪/本地 8085）等用户启动后单独跑。

### 决策⑥：FinGenius 个股辩论外部化 + GPL v3 合规边界（2026-07-06）

**背景**：循环B分析员只到板块方向（合规边界，不下沉个股买卖建议）。个股深度辩论交给外部 [FinGenius](https://github.com/HuaYaoAI/FinGenius)（6 角色 agent 多轮辩论 + 投票）。FinGenius 用 **GPL v3**，Syntopica 也是 **GPL v3**（见 `LICENSE` / README）。

**核心原则：进程隔离，绝不碰源码**。Syntopica 把 FinGenius 当作**独立的黑盒 HTTP 服务**调用，**不复制、不翻译、不静态/动态链接任何 FinGenius 源码进 Syntopica 仓库**。两边各自独立进程、独立仓库、独立 LICENSE。

**架构边界（GPL v3 传染性分析）**：

| 集成方式 | 是否构成 GPL 衍生作品 | 本 change 采用？ |
|---|---|---|
| HTTP 调用独立进程（SaaS/网络服务） | ❌ 不构成（松耦合，进程隔离） | ✅ **采用** |
| git submodule 引用（独立仓库） | ❌ 不构成（仓库隔离） | 不采用（多一层管理） |
| 复制/翻译 FinGenius 源码进 Syntopica | ⚠️ 构成衍生（强传染，整个 Syntopica 须继续 GPL v3） | ❌ **禁止** |

> GPL v3 的"强传染"针对"衍生作品"（结合成同一程序）。HTTP 调用独立服务是松耦合，**不传染**（FSF [FAQ](https://www.gnu.org/licenses/gpl-faq.html) + AGPL §13 均明确：网络调用不构成衍生）。Syntopica 的 Go 代码一行 FinGenius 代码都不引用。

**职责切分（两个独立开源项目）**：

| 项目 | 仓库 | 本 change 职责 | LICENSE |
|---|---|---|---|
| **Syntopica**（本仓库） | GPL v3 | 实现 `fingenius_client.go`（HTTP 客户端，调对方 `POST /analyze` + `GET /task/{id}` + `GET /health`）+ `stock_debate_result` 表 + orchestrator 接入 + 前端展示 | 维持 GPL v3 |
| **FinGenius 服务**（独立项目，已实现） | GPL v3 | **不在本 change 范围**——已在独立项目（`D:\project\FinGenius`）做 FastAPI 服务壳改造，暴露 `POST /analyze {symbols} → {tasks}` + `GET /task/{id}` + `GET /health`。本 change 只实现 Syntopica 侧**客户端**，不引入服务端代码 | 维持 GPL v3 |

**客户端契约（已与服务端实现对齐，2026-07-06）—— 异步任务 + 轮询**：

FinGenius 单次分析 6 agent ×（多步 LLM + sleep）+ 2 轮辩论 × 6 发言 + 报告生成，**实测数分钟**。同步 HTTP 必超时，故用**异步任务模式**：提交拿 task_id → 轮询拿结果。

**关键模型：一个 task_id 对应一个标的**（服务端按 symbol 拆成多个独立后台任务）。提交 N 个标的 → 返回 N 个 task_id；Syntopica 客户端可并发轮询多个 task_id，每个独立判完成/失败。

```
① 提交任务（立即返回，每个 symbol 一个 task）
POST {FINGENIUS_BASE_URL}/analyze
Request:  { "symbols": [{"code":"161129","name":"易方达原油","sector":"原油"},
                        {"code":"600519","name":"贵州茅台","sector":"白酒"}],
            "max_steps": 3,          // 可选，每 agent 最大步数（默认 3）
            "debate_rounds": 2,      // 可选，辩论轮数（默认 2）
            "agent_interval": 0 }    // 可选，research 阶段 agent 间隔秒（默认 0）
Response: { "tasks": [
             {"task_id":"fingenius_a1b2c3d4","stock_code":"161129","name":"易方达原油","sector":"原油"},
             {"task_id":"fingenius_e5f6g7h8","stock_code":"600519","name":"贵州茅台","sector":"白酒"} ] }
  // 注意：提交即 running（无 pending 态），任务在后台异步跑

② 轮询单个 task（Syntopica 每 FINGENIUS_POLL_INTERVAL 轮询一次）
GET {FINGENIUS_BASE_URL}/task/{task_id}
Response（运行中）: { "task_id":"...", "stock_code":"161129","name":"易方达原油","sector":"原油",
                     "status":"running",
                     "progress":{"current_agent":"舆情","done":2,"total":6},
                     "result":null, "error":null,
                     "created_at":1719900000.0, "updated_at":1719900012.3 }
Response（完成）:   { "task_id":"...", "stock_code":"161129","name":"易方达原油","sector":"原油",
                     "status":"done", "progress":null, "error":null,
                     "result": {
                       "stock_code":"161129", "analysis_time":187.5,
                       "research": {                 // 6 段文本 + basic_info（FinGenius 原始输出，非结构化）
                         "sentiment":"...舆情分析长文本...",
                         "risk":"...","hot_money":"...","technical":"...",
                         "chip_analysis":"...","big_deal":"...",
                         "basic_info":{...}          // 基本面快照
                       },
                       "battle": {                   // 辩论结构化输出（FinGenius 原生，8 字段）
                         "final_decision":"bullish", // bullish/bearish 二档（无中性）
                         "vote_count":{"bullish":4,"bearish":2},
                         "final_votes":{"sentiment_agent":"bullish", ...}, // 每 agent 票立场
                         "vote_results":{...},        // 投票明细聚合
                         "debate_history":[...],      // 辩论发言记录
                         "battle_highlights":[...],   // 关键观点
                         "agent_order":[...],         // agent 发言顺序
                         "debate_rounds":2            // 实际辩论轮数
                       },
                       "html_content":"<!DOCTYPE html>...", // 完整 HTML 报告字符串（非 URL）
                       "name":"易方达原油","sector":"原油"  // 服务端补到 result 顶层，便于客户端关联
                     },
                     "created_at":..., "updated_at":... }
Response（失败）:   { "task_id":"...", "status":"failed", "error":"错误信息", "result":null, ... }

③ 健康检查（客户端探活/降级判断用）
GET {FINGENIUS_BASE_URL}/health  →  { "status":"ok" }
```

**字段语义要点**：
- `result.html_content` 是**完整 HTML 字符串**（非 URL），Syntopica 落库时直接存 TEXT 字段；前端④「查看完整报告」按钮用 iframe `srcdoc` 渲染（或在详情弹窗里 v-html）。
- `result.research.basic_info` 是基本面快照（股票名/当前交易日等），提炼时只取 6 段文本（`sentiment`/`risk`/`hot_money`/`technical`/`chip_analysis`/`big_deal`），`basic_info` 落库但不参与 stance 提炼。
- `result.battle` 8 字段里，提炼环节主要消费 `final_decision`（整体倾向）+ `final_votes`（每 agent 票）；其余字段（`debate_history`/`battle_highlights` 等）原样落库供前端"展开辩论详情"用。
- `progress` 仅 `status=running` 时非空，结构恒为 `{current_agent, done, total}`，前端④加载态直接渲染。

**服务端持久化行为（影响客户端轮询策略）**：
服务端用 SQLite（`FINGENIUS_DB_PATH`，默认 `fingenius.db`）持久化任务。`done`/`failed` 结果**重启后仍可查**（GET /task 返回历史结果）；`running` 任务**重启后转 `failed`**（协程已死）。Syntopica 客户端可据此优化：用户重新打开页面时，对未完成的 task_id 重新轮询——若返回 `failed` 且 `error` 含"取消/中断"语义，提示用户重试而非死等。

> **关键：FinGenius 输出是「6 段文本 + 二档投票(bullish/bearish)」，不直接提供三档 stance**。FinGenius 服务端保持原始输出零加工（GPL 边界，不碰源码），**结构化提炼在 Syntopica 侧做**（见下"Syntopica 侧 LLM 提炼"）。

**Syntopica 侧 LLM 提炼（把 FinGenius 原始输出转成原型④的三档结构）**：

辩论结果拿到后，Syntopica 用一次轻量 LLM 调用（Operation=`data_enrichment.debate_distill`，归 `data_enrichment_analysis` capability）把每个 agent 的文本分析提炼成结构化字段：

```
输入（FinGenius 原始）: research.sentiment 文本 + battle.final_votes.sentiment_agent
输出（提炼后，存 stock_debate_result.agents）:
  { "role":"舆情", "stance":"up",      // up/down/flat（提炼：文本偏多→up，偏空→down，模糊→flat）
    "note":"热度上升",                 // 一句话理由（提炼自文本）
    "raw_vote":"bullish" }             // 保留 FinGenius 原始投票（bullish/bearish）
```

提炼规则（prompt 内置）：
- **stance 三档映射**：FinGenius `final_votes[agent]=bullish` → 倾向 `up`；`bearish` → `down`；但若文本语义与投票矛盾（如投票 bullish 但文本说"风险积聚"），以文本语义为准，stance 可降级为 `flat`。
- **note 一句话**：从该 agent 的 research 文本提炼核心结论（≤15 字）。
- **verdict 整体结论**：综合 6 agent stance + FinGenius `final_decision`：up 多 → `up`，down 多 → `down`，3:3 或矛盾 → `flat`。
- **consensus 共识度**：`{stance占多数计数}/6`（如 4 个 up → "4/6"；2:2:2 分歧 → "2/6 分歧"）。

> 这样原型④的"6 agent stance pill + 三档投票柱状图"就能基于提炼后的结构渲染，而 FinGenius 服务端一行不改。

**配置（不走主 config，仿 airouter 直接读 env，见 §决策⑤风格）**：

Syntopica 侧（`fingenius_client.go` 读）：
- `FINGENIUS_BASE_URL`（默认 `http://localhost:8000`）
- `FINGENIUS_API_KEY`（可选，服务端要求才传；当前服务端实现未启用鉴权，留接口）
- `FINGENIUS_TIMEOUT`（默认 `10s`——异步模式下每次 HTTP 调用都很快，长等待靠轮询）
- `FINGENIUS_POLL_INTERVAL`（默认 `8s`，轮询间隔）
- `FINGENIUS_MAX_WAIT`（默认 `10min`，单次辩论最大等待，超时判失败）

FinGenius 服务端侧（备忘，不在本 change 实现）：
- `FINGENIUS_DB_PATH`（默认 `fingenius.db`，SQLite 路径，done/failed 持久化、running 重启转 failed）

**合规义务清单（GPL v3，即使进程隔离也须遵守）**：
1. **致谢**：README / docs 标注"个股辩论能力由 [FinGenius](https://github.com/HuaYaoAI/FinGenius)（GPL v3）提供"。
2. **不修改 FinGenius 源码**：本 change 不碰 FinGenius 仓库；服务端改造另起项目时，须按 GPL v3 标注修改 + 保留原版权声明。
3. **链接提供**：文档提供 FinGenius 上游仓库链接，便于用户获取源码（GPL v3 对"下游分发"的要求；本仓库作为 GPL v3 开源项目，分发时一并满足）。
4. **LICENSE 不变**：Syntopica 维持 GPL v3（本就是），不因集成 GPL v3 的 FinGenius 而需要额外动作。

**降级策略（FinGenius 服务不可用时）**：
- 提交任务失败 / 轮询超时 → **non-fatal**，循环B主流程（板块方向预测）照常完成；个股辩论区块前端显示「连接 FinGenius 失败，检查服务状态」出错态（原型④已实现该态）。
- 不阻塞主分析，不影响板块走向判断——个股辩论是**可选增强**，不是核心闭环。
- 用户不开 FinGenius 服务时，Syntopica 完全可用（只是没有④个股辩论）。
- LLM 提炼失败（`debate_distill` 调用失败）→ 降级显示 FinGenius 原始文本（`research.*` 字段），不做三档渲染，保底可读。

**触发时机（原型④已验证方向）**：
- 不随循环B自动跑（FinGenius 辩论数分钟，会拖慢主分析）。
- 前端④区块「开始辩论」按钮**手动触发**：用户看完②板块方向后，选择性地对感兴趣的板块代表标的发起辩论。
- 提交后前端进入轮询加载态（原型④的"FinGenius 辩论中..."态展示 progress），轮询完成渲染结果。
- 辩论结果 append-only 存 `stock_debate_result` 表，同一 result 内不重复调。

**FinGenius 服务端（已实现，独立 GPL v3 项目，不在本 change 范围）**：

服务壳改造已在另起的独立项目（`D:\project\FinGenius`，GPL v3）完成，作为 Syntopica 个股辩论的外部依赖。本 change 的客户端契约（上文）即基于该实现落定。改造要点（备查）：
- 用 FastAPI（`uvicorn`）包原 `analyze_stock`，暴露 `POST /analyze`（提交）+ `GET /task/{id}`（轮询）+ `GET /health`（探活）。**一个 task_id 对应一个标的**——提交 N 个 symbol 返回 N 个 task_id，Syntopica 客户端并发轮询。
- 剥离原 `main.py` 的终端 UI 副作用（`visualizer.show_*` / `clear_screen`），写成 headless 版 `analyzer_service.py`（与原 `EnhancedFinGeniusAnalyzer` 并列，零改动原代码）。
- 报告从 `CreateHtmlTool.output["html_content"]` 取 HTML 字符串返进 `result.html_content`（原 `main.py` 没取，这里取出来走响应体，不强制写盘）。
- 参数化 `max_steps`/`debate_rounds`/`agent_interval`（原 `research.py:124` 的 `sleep(3)` 硬编码改为可配，HTTP 默认 0）。
- SQLite 持久化（`task_store.py`）：done/failed 重启可查，running 重启转 failed（协程已死）。
- **延迟导入**：`_run_one()` 内才 `from src.analyzer_service import headless_analyze_stock`，让 server 启动不被 openai/akshare/efinance 重依赖拖累——health/submit/task 端点零依赖、可单测（11 个 pytest 全绿，全 mock LLM，未配 key 也能跑）。
- 启动：`uvicorn src.server:app --port 8000`。
