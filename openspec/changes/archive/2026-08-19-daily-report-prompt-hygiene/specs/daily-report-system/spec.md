## ADDED Requirements

### Requirement: 日报聚类裁决 prompt 历史隔离

L2 泳道裁决（`buildL2Prompt`，operation `daily_report.decide_l2_tags`）的 LLM prompt SHALL NOT 注入候选话题的历史叙事文案（`daily_report_threads` 的 title / summary），切断「昨天幻觉 thread → 今天作为 briefs 喂回 → LLM 延续叙事」的渗透闭环。

L2 裁决 prompt MAY 注入以下**非叙事**信号辅助裁决：候选 topic 的 `label`、状态（active / candidate）、最近命中日期、累计命中天数、质心余弦距离、近期 section 的框架名（`cluster_label` / `section_label`，话题命名级别，**非** thread 文案）。

L2 system prompt 的裁决依据措辞 SHALL 与实际注入内容保持一致——基于「标签语义与近期 section 框架」，而非「实际近期内容（thread 文案）」，避免 prompt 自相矛盾误导 LLM。

本约束随 `promptVersion` 由 "3.0" 升至 "4.0" 一并生效。

#### Scenario: L2 prompt 不含历史 thread 文案

- **GIVEN** board 的某 active 话题近 7 天生成过 thread（title="半导体链全线跌停"）
- **WHEN** 当天 L2 裁决为某沾边 tag 构建 prompt
- **THEN** prompt SHALL NOT 出现该 thread 的 title 或 summary 文案
- **AND** prompt MAY 出现该话题的 label、状态、最近命中日期、质心距离、近期 section_label

#### Scenario: L2 prompt 保留话题框架信号以区分近似话题

- **GIVEN** 两个 active 话题 label 字面相近但近期 section 框架不同
- **WHEN** L2 裁决为某 tag 构建 prompt
- **THEN** prompt SHALL 提供两者的 section 框架信号以供区分
- **AND** SHALL NOT 提供任一话题的 thread title / summary 文案

#### Scenario: promptVersion 升级

- **WHEN** 日报按本约束生成
- **THEN** `board_daily_reports.generation_prompt_version` SHALL 为 "4.0"

### Requirement: 日报文案生成事实锚约束

日报要闻（`GenerateHighlights`，operation `daily_report.highlights`）与叙事线程（`GenerateClusterThreads`，operation `daily_report.threads`）的 system prompt SHALL 包含「事实锚」约束，要求生成的 title / reason / summary 仅基于所提供标签的事实（`label` / `description` / `代表文章`）。

事实锚约束 SHALL 明确禁止以下编造行为（当对应信息未在所列标签中出现时）：

1. 编造未列举的具体事件
2. 编造具体数字（涨跌幅 / 金额 / 连板数 / 百分比 / 跌停涨停）
3. 编造市场情绪判断（恐慌 / 狂热 / 崩盘 / 抛售）
4. 编造因果推断（「引发」/「导致」/「因此」）

当标签信息不足以支撑某条叙事时，系统 SHALL 选择不生成该条（如返回 `{"threads":[]}`），而非补全编造。

JSON schema 中 `summary` / `reason` 字段的 description SHALL 追加「须基于所列标签事实，禁止编造」作双重强化。

本约束随 `promptVersion` "4.0" 一并生效。

#### Scenario: thread summary 不编造数字与情绪

- **GIVEN** 某 cluster 的 tag 仅为几个半导体公司名（label），无任何涨跌 / 情绪描述
- **WHEN** `GenerateClusterThreads` 生成 thread
- **THEN** thread 的 title / summary SHALL NOT 出现「全线跌停」「引发恐慌」等未由标签支撑的数字或情绪判断

#### Scenario: highlights reason 不编造因果

- **GIVEN** 当天 tag 无任何关于事件因果的描述
- **WHEN** `GenerateHighlights` 生成要闻
- **THEN** reason SHALL NOT 出现「引发」「导致」等未由标签支撑的因果推断

#### Scenario: 信息不足时宁缺毋滥

- **GIVEN** 某 cluster 仅含 1 个 tag 且描述匮乏
- **WHEN** 生成 thread
- **THEN** 系统 MAY 返回空 threads（`{"threads":[]}`），SHALL NOT 编造内容凑数
