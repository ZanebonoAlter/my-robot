## 1. L2 裁决 prompt 去历史 thread 注入

- [x] 1.1 [RED] 写失败测试 `TestBuildL2Prompt_ExcludesThreadTitles`：构造含 `ThreadTitles` 的 briefs 调用 `buildL2Prompt`，断言返回的 user prompt **不含**任何 thread title 文案
- [x] 1.2 [RED] 写失败测试 `TestBuildL2Prompt_KeepsSectionLabelAndMeta`：断言 user prompt **仍含** topic label、状态（正式/观察中）、最近命中日期、累计天数、质心距离、以及 section_label
- [x] 1.3 [GREEN] 改 `buildL2Prompt`（`daily_report_lane.go`）：移除 `briefs[t.ID]` 的 `ThreadTitles` 拼接循环，保留 `SectionLabel` + `PeriodDate` 框架信号
- [x] 1.4 [GREEN] 改 L2 内联 system 措辞：「判断须基于候选话题的**实际近期内容**」→「判断须基于候选话题的**标签语义与近期 section 框架**」，与实际注入内容一致

## 2. 文案生成事实锚约束

- [x] 2.1 [RED] 写失败测试 `TestThreadsSystemPrompt_HasFactAnchor`：断言 `threadsSystemPrompt` 含事实锚关键词（"禁止编造" 与 "基于…标签" 语义）
- [x] 2.2 [RED] 写失败测试 `TestHighlightsSystemPrompt_HasFactAnchor`：同上针对 `highlightsSystemPrompt`
- [x] 2.3 [GREEN] `threadsSystemPrompt` 追加事实锚段：title/summary 仅基于所列 tag 事实；禁止编造未列举事件/具体数字（涨跌幅/金额/连板数/跌停涨停）/市场情绪（恐慌/狂热/崩盘）/因果（引发/导致）；信息不足宁可不写
- [x] 2.4 [GREEN] `highlightsSystemPrompt` 追加同样事实锚段（针对 title/reason）
- [x] 2.5 [GREEN] JSON schema 的 `summary`（threads）/ `reason`（highlights）字段 `Description` 追加「须基于所列标签事实，禁止编造」

## 3. promptVersion 升级

- [x] 3.1 [RED] 写失败测试 `TestPromptVersion_Is4`：断言 `promptVersion == "4.0"`
- [x] 3.2 [GREEN] 改 `const promptVersion = "4.0"`（`daily_report_llm.go:16`）

## 测试

- 覆盖：L2 prompt 历史隔离（不含 thread title / 保留 section_label+元数据）、两处 system prompt 含事实锚关键词、promptVersion=4.0。均为纯字符串/逻辑断言，不依赖 LLM/DB。
- 影响包：`internal/topicgraph/service`（仅此包，遵循 AGENTS.md「只跑影响包」）。

## 文档

<!-- doc-impact: flow -->
<!-- doc-impact-excuse: api=命中文件属并行 change ai-model-health-gate 的脏工作区改动; database=同上（ai_models.go）; architecture=同上（app/runtime.go）; configuration=configs/config.yaml 属并行 change data-enrichment-structural-depth 的博查配置（本 change 未动 config） -->

- [x] `docs/reference/flow/daily-report.md`：链路设计 Step3.5「Slice D — lane context injection」描述更新——L2 裁决 prompt **不再注入历史 thread title**，仅注入 topic label + section 框架名 + 命中元数据（状态/最近命中/累计天数/距离）
- [x] `docs/reference/flow/daily-report.md`：mermaid 图修正 Step4「取昨日报告做连贯性参考」→ 标注 `findPreviousReportBrief` 仅写 `PrevReportID` 引用、**未进任何 LLM prompt**（修正文档↔代码不符）
- [x] `docs/reference/flow/daily-report.md`：「业务约束与不变量」节新增两条不变量——① L2 裁决 prompt 历史隔离 ② 日报文案（highlights/thread）事实锚约束（禁止编造事件/数字/情绪/因果）
- [x] `docs/reference/flow/daily-report.md`：「变更溯源」表补一行（daily-report-prompt-hygiene，prompt 卫生）

## 验证

- `cd backend-go && golangci-lint run ./internal/topicgraph/...` → 零失败
- `cd backend-go && go vet ./internal/topicgraph/...` → 零失败
- `cd backend-go && go test ./internal/topicgraph/service` → PASS（含新增 prompt 卫生测试）
- `cd backend-go && go build ./...` → 编译成功
- `bash scripts/doc-impact.sh verify openspec/changes/daily-report-prompt-hygiene` → 通过（flow 域声明↔文档更新对账）
- `bash scripts/check-standards.sh` → A–G 段零失败
