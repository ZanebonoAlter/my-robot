## Context

日报生成管线（`GenerateDailyReport`）有 **3 个会输出/消费叙事文案的 LLM 调用点**，构成"幽灵新闻"的渗透与脑补通道：

1. **L2 裁决**（`ClusterTagsLane` → `buildL2Prompt`，operation `daily_report.decide_l2_tags`）：对 L2 弱区 tag 做 keep/switch/new。当前 prompt 把每个候选话题近 7 天的 `section_label` + `thread_title`（`ListTopicRecentBriefs`，`briefsSinceDays=7`/`briefsPerTopicCap=5`/每 section 取 2 条 thread title）当"近期内容"原样注入，且 system 明确要求"判断须基于候选话题的**实际近期内容**"——等于逼 LLM 参考历史叙事。**主渗透通道**：昨天幻觉 thread 今天还在 7 天窗口内被喂回，引导 LLM keep 沾边 tag 并延续叙事。
2. **要闻**（`GenerateHighlights` → `highlightsSystemPrompt`，operation `daily_report.highlights`）：生成 2-3 条 title + reason。system 无任何事实锚约束，reason 是 LLM 自由脑补，可能写出"半导体跌停引发恐慌"这种当天事实撑不起的演绎。
3. **线程**（`GenerateClusterThreads` → `threadsSystemPrompt`，operation `daily_report.threads`）：每 cluster 生成 thread title + summary（100-200 字）。同样无事实锚约束。

三者叠加形成**累积闭环**：幻觉 thread → 存库 → 次日 L2 briefs 喂回 → 更多沾边 tag 被归入话题 → thread 生成素材更多 → 脑补更丰富。

**红鲱鱼**：`findPreviousReportBrief`（Step4）/ `PrevReportID` 只存引用 ID，从未进入任何 prompt，不背锅。

当前 `promptVersion = "3.0"`（`daily_report_llm.go:16`）。

## Goals / Non-Goals

**Goals**
- G1 切断 L2 历史叙事渗透：`buildL2Prompt` 不再注入历史 `thread_title`。
- G2 日报文案受事实锚约束：`highlightsSystemPrompt` + `threadsSystemPrompt` 加"基于所列 tag 事实、禁止编造"约束。
- G3 `promptVersion` 3.0 → 4.0。
- G4 `docs/reference/flow/daily-report.md` 同步（Slice D 描述 + 与代码不符的 mermaid Step4）。

**Non-Goals**
- 现有数据清洗（用户明确不做；历史 briefs 残留最多 7 天自然衰减）。
- Call A 是否真传"昨日日报"的 spec↔代码偏差（`daily-report-system` spec 仍写"输入…+昨日日报"，代码不传——另立议题，不在本次范围）。
- 改 lane 阈值（`lane_l1/l2_threshold`）/ embedding 算法 / topic 生命周期 / 吸尘器逻辑。
- 降 temperature（保持 0.3，避免范围蔓延；脑补靠 prompt 事实锚约束，不靠降温）。
- 改数据模型 / API / 配置 / 部署 / 前端。

## Decisions

### D1 — L2 briefs 保留范围：去 thread_title，留 section_label + 元数据

`buildL2Prompt` 改造后，每个候选话题注入：
- **保留**：topic `label`、状态（正式/观察中）、最近命中日期、累计天数、质心距离、`section_label`（话题近期 section 框架名，作为弱信号）。
- **移除**：`thread_title`（叙事文案，渗透性最强）。

**理由**：`thread_title` 是具体叙事句（"半导体链全线跌停"），注入后会被 LLM 当作"这个话题在讲什么"直接延续，是幽灵的主渗透源。`section_label` 是话题框架命名级别（"半导体产业链"），是话题跨天演进的**合理延续信号**（话题本就靠它串联），渗透性远弱于 thread 文案。

**备选 A（否决）**：连 `section_label` 一起去掉，只留 label + 命中元数据。否决理由：损失 L2 区分"标题字面相似但近期内容分属不同叙事"话题的能力（这正是 L2 system prompt 原本的设计目的，去掉会让近似话题误并）。
**备选 B（否决）**：完全去掉 briefs 注入。同备选 A 理由，且 L2 裁决退化到只看 label 字面。

### D2 — L2 system prompt 措辞同步修正

L2 system 当前写"判断须基于候选话题的**实际近期内容**"。注入内容改为 section_label 后，这句话要同步改为"判断须基于候选话题的**标签语义与近期 section 框架**，而非仅凭标题字面沾边"，避免 prompt 自相矛盾误导 LLM。

### D3 — 事实锚约束统一加到 highlights + thread 的 system prompt

`highlightsSystemPrompt` 与 `threadsSystemPrompt` 各加一段"事实锚"约束，核心：

> title / summary / reason 必须仅基于所提供的标签（label / description / 代表文章）中的事实。禁止编造未在标签中出现的：具体事件、具体数字（涨跌幅 / 金额 / 连板数 / 百分比 / 跌停涨停）、市场情绪判断（恐慌 / 狂热 / 崩盘 / 抛售）、因果推断（"引发""导致""因此"）。若标签信息不足以支撑某条叙事，宁可不写，也不要补全。

同时在 JSON schema 的 `summary` / `reason` 字段 `Description` 追加一句"须基于所列标签事实，禁止编造"作双重强化。

### D4 — promptVersion 3.0 → 4.0

`const promptVersion = "4.0"`。仅作版本标记（`BoardDailyReport.GenerationPromptVersion`），无 schema 变更。

## Risks / Trade-offs

- **[L2 归属变保守，candidate 话题增多]** → 可接受。candidate 有 `CandidateDecayWindow`（7 天）自然衰减；减少错误归属正是本 change 的预期效果，"宁可新开 candidate 也不要把沾边 tag 硬塞进既有话题"。
- **[highlights/thread 文案变干，可读性略降]** → 可接受。准确性优先于文采；temperature 不降（0.3），只靠 prompt 约束。
- **[section_label 仍是历史产物]** → 弱于 thread 文案，且话题框架延续是设计意图。若 section_label 本身是幻觉，属上游聚类问题，不在本次范围。
- **[LLM 可能绕过事实锚约束]** → 事后从 `ai_call_logs`（`operation` 过滤）抽查 prompt + response 验证；现有 `fit_distance`（thread↔section，System 3）可作辅助观测。约束是"显著降低"而非"绝对消除"脑补。
- **[历史日报残留幽灵]** → 不回改历史；新 prompt 生效后，残留 briefs 在 7 天窗口内自然衰减。若急需可对个别污染话题手动 backfill，但本次不做批量清洗。

## Migration Plan

- **无 schema 迁移、无配置变更、无依赖变更**。
- `promptVersion` 升级仅标记，新生成日报写 "4.0"，历史日报保持 "3.0"。
- 部署后下一次日报生成（手动触发或定时）即按新 prompt。
- **回滚**：还原 3 处 prompt 文本 + `promptVersion`，无数据副作用。

## Open Questions

- **Q1**：`section_label` 是否保留？（决策 D1 倾向**保留**，apply 时若实测发现 section_label 仍带强幻觉可再收紧为只留 label + 元数据。）
- **Q2**：是否需要在 highlights/thread 的 JSON schema 字段 description 也加约束？（决策 D3 倾向**加**，与 system prompt 双重强化。）
