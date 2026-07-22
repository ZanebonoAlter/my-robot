# Tasks — causal-analysis-agent

> 参考：`design.md`（D1-D9 决策，D9 报告追问）、`specs/data-enrichment/spec.md`（9 ADDED Requirements）、`prototype/causal-chain-report.html`（见解层 v4）。
> 复用 data-enrichment-orchestration 骨架（三表/agent loop 三防御/可观测/循环A/tool registry），本 change 换"分析目标"+ 视角机制 + 见解层 + 工具集 + 报告追问 + 前端报告。

## 1. 数据迁移与模型适配

- [ ] 1.1 迁移脚本：清空旧演进定位数据（`topic_enrichment_result` / `topic_enrichment_review` TRUNCATE + RESTART IDENTITY，旧 position/signals/verdict 语义不可复用）—— `postgres_migrations.go` 新版本
- [ ] 1.2 `analyzeOutput` struct 重写：`{form, lens, analysis}`，analysis 按 form 多态（event_chain={fact_layer,timeline,insight_layer} / theme_vein={veins,cross_insight} / single_point={impact} / sparse={notice}）—— `service/orchestrator.go`
- [ ] 1.3 `topic_enrichment_result.Sectors` jsonb 存复合对象 `{form,lens,analysis}`（免 DDL 复用列），解析适配
- [ ] 1.4 `ReviewJudgeOutput` struct 重写：`{new_findings[], overturned[], confidence_shift[]}` 替 `{position_change, change_summary}`
- [ ] 1.5 新表 `topic_enrichment_qa`（result_id + question/answer/tool_calls + created_at，多轮 append-only）—— `repository/models.go` + migration

## 2. 解读员重构：形态判断 + 视角候选

- [ ] 2.1 形态判断：解读员 prompt 扩展，输出 `form ∈ {event_chain,theme_vein,single_point,sparse}`，判据含 hit_count/section 数/cluster_label 发散度/内容语义（`service/orchestrator.go` interpretPrompt）
- [ ] 2.2 视角候选：解读员输出 `lens_candidates[]`（具体问题式，如"美国为何反复横跳"，非抽象标签），每个带视角名+一句话说明
- [ ] 2.3 `LensSource` 接口抽象（`service/lens_source.go`）：`Propose(topic, form) []Lens`；首批 `AgentLensSource`（LLM 生成），预留外部源（VideoCommentator/Report）
- [ ] 2.4 形态判断测试：`TestInterpret_FormClassification`（4 形态各一 case）
- [ ] 2.5 视角候选测试：`TestInterpret_LensCandidates`（输出具体问题式，非抽象标签）

## 3. 探索工具集：多级入口 + web_search

- [ ] 3.1 `list_boards()` 工具：返版块全景 `{id,name,活跃度}`（`service/tool_registry.go`）
- [ ] 3.2 `list_lanes(board_id)` 工具：返版块下持久话题泳道（查 `board_persistent_topics`）
- [ ] 3.3 `get_lane_detail(lane_id, window)` 工具：返泳道详情（复用 `lifeline_renderer`）
- [ ] 3.4 `web_search(query)` 工具：网页结果（验证事实 + 支撑推演中间环节）
- [ ] 3.5 金融工具降级为可选：仅金融话题注册（沿用 §11.1.2 动态注册）
- [ ] 3.6 工具集测试：`TestToolsForExploration`

## 4. 探索 agent 循环（分析主引擎）

- [ ] 4.1 `runAgentLoop` 扩展为分析主引擎：注册多级入口+web_search，agent 自主决定调哪个工具/查多深/何时停（`service/orchestrator.go`）
- [ ] 4.2 按形态控深：event_chain 深挖 / sparse 浅出（form 传入约束）
- [ ] 4.3 按视角聚焦：仅推演与选定 `lens` 相关的链（lens 传入 system prompt）
- [ ] 4.4 沿用三防御（去重/不截断/thinking 关闭），max_loops 上限兜底
- [ ] 4.5 探索 agent 测试：`TestRunAgentLoop_FormDepthControl` / `TestRunAgentLoop_LensFocus`

## 5. 见解层产出：分层 + 确定性 + 依据

- [ ] 5.1 分析员 prompt 重写：产出分层（fact_layer + insight_layer），按形态+视角产出（`service/orchestrator.go` analyzePrompt）
- [ ] 5.2 确定性分级：每条 insight 标 `high/medium/low/question` + logic 字段
- [ ] 5.3 依据强制：每条 insight 必须挂文章依据 + 时间线节点，无依据拒绝（解析校验）
- [ ] 5.4 提问式见解约束：`question` 级指出条件非预言成败
- [ ] 5.5 见解层测试：`TestAnalyze_LayeredInsight` / `TestAnalyze_InsightMustHaveEvidence` / `TestAnalyze_CertaintyGrading`

## 6. review 重定义：新发现/推翻对比

- [ ] 6.1 review judge prompt 重写：对照上次 insight_layer，输出 new_findings/overturned/confidence_shift（`service/review_judge.go`）
- [ ] 6.2 存储：ReviewJudgeOutput 存 `Verdict` jsonb（免 DDL），不回写表1
- [ ] 6.3 review 测试：`TestRunReviewJudge_NewFindingsOverturned`

## 7. 前端：多形态分析报告

- [ ] 7.1 视角选择 UI：展示 agent 候选视角（具体问题式），用户选/自填
- [ ] 7.2 时间线依据轴：渲染 section 时序节点 + 关键节点高亮（吃 analysis.timeline）
- [ ] 7.3 见解层渲染：事实层/见解层分区，确定性 4 级视觉区分
- [ ] 7.4 按形态渲染：event_chain=因果链+见解 / theme_vein=平行线索+跨线索洞察（不画因果箭头）/ single_point=影响评估 / sparse=信息不足
- [ ] 7.5 双类引用：📰新闻/🔧工具 hover tooltip（沿用 evidence.source_type）
- [ ] 7.6 探索过程 trace：文末轻量交代
- [ ] 7.7 前端测试：typecheck/build/test:unit 全绿（boardEnrichment.test.ts 对齐新 schema）

## 8. 触发接线

- [ ] 8.1 沿用 EnrichTopic 话题级入口（手动触发，不挂日报管线）
- [ ] 8.2 触发流程：解读员(形态+视角候选) → 前端展示候选 → 用户选视角 → 探索 agent → 产出 → review
- [ ] 8.3 增强失败只记日志，天然隔离（沿用）

## 9. 报告追问交互层（复用探索 agent）

- [ ] 9.1 追问 agent：复用 `runAgentLoop`，输入用户问题 + 报告上下文（analysis+依据+视角+形态），调探索工具（新 `service/qa_agent.go`）
- [ ] 9.2 追问 API：`POST .../results/:id/qa`（提问，返 answer + tool_calls）、`GET .../results/:id/qa`（多轮历史）
- [ ] 9.3 沉淀 API：`POST .../qa/:id/sediment`（手动沉淀追问依据回报告，标记 source=qa，不自动改报告）
- [ ] 9.4 前端追问框 + chat 渲染：报告页底部追问，多轮对话，回答带双类引用 + 确定性（复用报告渲染）
- [ ] 9.5 沉淀交互：追问回答中有价值的依据，用户可点「沉淀到报告」
- [ ] 9.6 追问 agent 测试：`TestQAAgent_ReuseExplorationLoop`（复用探索工具）/ `TestQAAgent_ReportContext`（带报告上下文不跑题）

## 10. 测试

<!-- doc-impact: 数据增强认知闭环 -->

- [ ] 10.1 后端单元测试：`go test ./internal/dataenrichment/...`（形态判断/视角候选/探索agent/见解层/review/追问 全覆盖）
- [ ] 10.2 前端单元测试：`boardEnrichment.test.ts` 对齐 {form,lens,analysis} schema + 追问交互
- [ ] 10.3 骨感型诚实测试：sparse 形态不产出推演见解
- [ ] 10.4 见解依据强制测试：无依据 insight 被拒绝
- [ ] 10.5 追问沉淀测试：手动沉淀标记 source=qa，不自动改报告

## 11. 文档

- [ ] 11.1 `docs/reference/database/DATABASE_FIELDS.md`：`topic_enrichment_result.Sectors` 语义变更 + `topic_enrichment_review.Verdict` 语义变更 + 新表 `topic_enrichment_qa`
- [ ] 11.2 `docs/reference/flow/`：数据增强认知闭环流程更新（形态判断→视角选择→探索→分层见解→新发现对比→报告追问），archive 后补 §12 变更溯源
- [ ] 11.3 `docs/reference/standard/backend/ai-logging.md`：补 `data_enrichment.interpret`(含形态+视角) / `data_enrichment.analyze`(分层见解) / `data_enrichment.qa`(报告追问) Operation 跟踪
- [ ] 11.4 `backend-go/AGENTS.md`：domain 白名单已含 dataenrichment（确认无需改）
- [ ] 11.5 与 data-enrichment-orchestration 关系：在其 tasks.md 标注 `[~]` 演进定位任务（3.6/4.2/5.2/7.4-7.5）作废，由本 change 接管

## 12. 验证

- [ ] 12.1 后端门禁：`cd backend-go && golangci-lint run ./... && go vet ./... && go test ./internal/dataenrichment/... && go build ./...` → 零警告/全 PASS/构建成功
- [ ] 12.2 前端门禁：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build"` → 全绿（typecheck/build/test:unit 必须走 Windows cmd）
- [ ] 12.3 形态/视角落地校验：`grep -rn "form\|lens_candidates\|insight_layer\|cert" backend-go/internal/dataenrichment/` → 命中
- [ ] 12.4 演进定位清除校验：`grep -rn "position\|signals\|position_change" backend-go/internal/dataenrichment/` → 仅遗留兼容守卫或零命中
- [ ] 12.5 前端 schema 对齐校验：`grep -rn "form\|lens\|insight_layer\|cert" front/app/` → 命中
- [ ] 12.6 数据清空校验：迁移后 `docker exec syntopica-postgres psql -U postgres -d syntopica -t -A -c "SELECT count(*) FROM topic_enrichment_result"` → 0
- [ ] 12.7 追问落地校验：`grep -rn "qa_agent\|topic_enrichment_qa\|sediment" backend-go/internal/dataenrichment/` → 命中（追问 agent + qa 表 + 沉淀落地）
- [ ] 12.8 doc-impact 对账：`bash docs/reference/开发执行规范.md` §11.4 doc-impact.sh verify → 通过（check-standards F/G 段）
