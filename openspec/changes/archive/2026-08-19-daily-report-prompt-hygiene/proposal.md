## Why

日报生成出现"前一日幽灵新闻"：当天根本没有的事件（如"A股半导体链全线跌停引发市场恐慌"）被写进日报。根因是日报 prompt **沿用了近 7 天的历史叙事**——L2 裁决 prompt（`buildL2Prompt`）把每个候选话题近 7 天的 `section_label` + `thread_title` 当作"近期内容"原样注入，引导 LLM 把当天沾边 tag 强行归入该话题并延续历史叙事；加上 thread 生成（`buildThreadsPrompt`）没有任何"事实锚"约束，LLM 基于少量沾边 tag 即可脑补出当天事实撑不起的演绎文案。两者叠加形成**累积闭环**：今天的幻觉 thread 明天又作为 briefs 被喂回，越滚越大。

> 注：用户最初怀疑的"前一日报告"（`findPreviousReportBrief` / `PrevReportID`）是**红鲱鱼**——它只存了引用 ID，从未进入任何 LLM prompt。

## What Changes

- **L2 裁决 prompt 去历史渗透**（`buildL2Prompt`）：移除近 7 天话题 `thread_title` 的注入，**只保留** topic label + 命中元数据（状态/最近命中日期/累计天数/质心距离）。可选保留 `section_label`（话题框架名）作为弱信号，但**绝不再注入历史 thread 文案**。切断"昨天幻觉 → 今天延续"的主渗透通道。
- **日报文案生成统一加事实锚约束**（`highlightsSystemPrompt` + `threadsSystemPrompt`）：system prompt 明确要求——title / summary / reason SHALL 仅基于所列 tag 的 `label`/`description`/`代表文章`，SHALL NOT 编造未列举的事件、具体数字（涨跌幅/金额/连板数/百分比）、市场情绪判断（恐慌/狂热/崩盘）或因果推断。统一压住 highlights reason 与 thread summary 两个文案点的脑补。
- **prompt version 升级**：`generation_prompt_version` 由 "3.0" 升至 "4.0"，标记 prompt 卫生版本。
- **文档同步**：修正 `docs/reference/flow/daily-report.md`——更新 Slice D（lane context injection）描述反映"不再注入历史 thread 文案"；修正 mermaid 图与代码不符的 `Step4 取昨日报告做连贯性参考`（实际仅存 `PrevReportID` 引用，未进 prompt）。
- **不改**：数据模型、API、配置、部署、前端、lane 分桶阈值、embedding 算法、topic 生命周期。
- **不纳入**（避免范围蔓延）：现有数据清洗（用户明确不做）、Call A 是否真传昨日日报的 spec↔代码偏差（另立议题）。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `daily-report-system`：聚类 L2 裁决 prompt 与日报文案生成（highlights/thread）prompt 新增“prompt 卫生”不变量——L2 裁决 SHALL NOT 注入历史 thread 文案（仅可注入 topic 框架信号 + 命中元数据）；highlights/thread 文案 SHALL 受事实锚约束（禁止编造未列举的事件/数字/情绪/因果）。新增对应 requirement，防回退。

## Impact

- **后端代码**：`backend-go/internal/topicgraph/service/daily_report_lane.go`（`buildL2Prompt` + 内联 system）、`backend-go/internal/topicgraph/service/daily_report_llm.go`（`highlightsSystemPrompt` + `threadsSystemPrompt` + `buildThreadsPrompt` + `buildHighlightsPrompt`）、`promptVersion` 常量（3.0→4.0）。
- **文档**：`docs/reference/flow/daily-report.md`（flow 域——Slice D 描述、mermaid Step4）。
- **无影响**：数据模型 / API / 配置 / 部署 / 前端 / 依赖。
- **可观测**：`ai_call_logs.prompt` 字段（`operation='daily_report.decide_l2_tags'` / `'daily_report.threads'`）可用于事后验证 prompt 不再含历史 thread 文案、thread summary 是否贴事实。
- **行为变化**：日报的 L2 归属更保守（沾边 tag 更可能判 `new` 而非 keep 进既有话题）；thread 文案更干、更贴当天事实，不再出现"全线跌停/引发恐慌"等演绎。
