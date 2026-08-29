# Tasks: board-level-deep-analysis

> 2026-08-28 用户验收推翻了原“自动立命题→支持性检索→论文式长文”主线。本清单保留可复用的已完成基础设施，并以未完成任务跟踪“版块简报→用户选题→多假设调查”的修订实现；旧任务历史由 git 保留。

## 0. 已完成且继续复用的基础设施

- [x] 0.1 版块 scope result、semantic_board_id、nullable topic id、lane source_type/kind 与旧枚举兼容已落地；已有数据库与 repository 测试可作回归基线
- [x] 0.2 态势卡（week→month→实质 section 指纹）、month/year 补全门（缺行首建/72h 重算/40 次上限）、质量排序与失败降级已由现有实现落地；已有 service/DB 测试可作回归基线
- [x] 0.3 版块触发/结果 API、历史列表、lane 下钻、单泳道 prefill_lens 与 legacy 报告组件已落地；新实现须增量演进而非删除兼容路径
- [x] 0.4 append-only result、QA 追问、review 隔离与 ai_call_logs SessionID 链路已存在；简报和调查继续复用
- [x] 0.5 修订版白盒用例已记录于 `test-cases.md`；实现时按 M1-M10 用例先行

## 1. 结果种类与父子数据模型

- [x] 1.1 先新增迁移测试：旧 topic/board 行分别回填 `topic_analysis`/`legacy_board_analysis`，调查 parent 必须指向同版块 brief且带 question_key，回滚不改旧 sectors；验证 `go test ./internal/platform/database -run 'ResultKind|BoardInvestigationParent|QuestionKey'` 先红后绿
- [x] 1.2 新增 `result_kind`、nullable `parent_result_id` 与 nullable `question_key` 迁移/model/约束；question_key 由 trim+空白折叠后的问题文本 hash 生成，旧 JSON 原样保留；验证 1.1 测试通过并抽查 legacy 行数量不变
- [x] 1.3 repository 增加按 kind 查询、简报父子调查写入与同版块父结果校验；验证 repository 单测覆盖 kind 隔离、跨版块 parent 拒绝、一 brief 多 investigation
- [x] 1.4 result 序列化增加 kind/parent，保持无新字段的历史行可读；验证 handler/repository 兼容测试通过

## 2. 分析方法卡库

- [ ] 2.1 先写方法选择与阶段隔离单测：0 张可正常调查、最多 2 张、avoid_when 优先、简报 prompt 零方法正文、超预算按整卡舍弃；验证 `go test ./internal/dataenrichment/service -run 'AnalysisMethod|BoardBriefNoMethod'` 先红后绿
- [ ] 2.2 新建 `analysis_methods` model/repository/migration，字段含 summary/selection_meta/content/enabled/deleted_at；实现 `/analysis-methods` CRUD/启停/软删除；验证 repository + handler 创建/更新/停用/软删除测试通过
- [ ] 2.3 将旧 `reference_roles` 非破坏复制为 disabled legacy 方法记录，保留旧表与原文；《内部看美国》升级后不得自动注入；验证迁移测试比较原文 hash、enabled=false、旧表仍存在
- [ ] 2.4 实现方法选择器：仅按用户问题+父简报元数据选 0-2 张，再用选中方法辅助生成假设；method_refs 固化 id/title/content_hash，理由、正文与预算舍弃写入 input_snapshot/ai_call_logs；验证 2.1 与“方法删除后历史可回放”测试通过
- [ ] 2.5 移除旧参考角色对 board/topic interpret/tool/analyze 的全局注入，保留只读兼容 API 一个版本；验证 grep 仅兼容层引用旧注入函数，简报、调查及单泳道 prompt snapshot 均不含作者画像
- [ ] 2.6 前端设置页由“参考角色”改为“分析方法”，表单覆盖适用/禁用/证据/失败模式，legacy 项默认停用并提示人工整理；验证组件单测覆盖 CRUD、启停和 legacy 提示

## 3. 版块简报链

- [ ] 3.1 先写 `board_brief` parser/prompt 用例：无统一关系、并行趋势、全 sparse、幽灵 lane、非法关系枚举、坏 JSON 重试与机械降级、输出数量上限；验证 `go test ./internal/dataenrichment/service -run 'BoardBrief'` 先红后绿
- [ ] 3.2 新增 `data_enrichment.board_brief` Operation 与简报 schema，输入仅为补齐后的态势卡 + 同 kind 历史 review，禁止 web/fetch 工具和方法卡全文；验证 prompt snapshot 与 3.1 测试通过
- [ ] 3.3 将现有版块 trigger 编排改为 freshness→cards→brief→persist，不再自动运行 board_interpret/研究循环/论文 analyze；失败重试后机械降级只产观察，不造关系；验证 mock 断言默认触发 LLM/工具调用次数符合契约
- [ ] 3.4 实现简报关系校验与 lane 白名单清理，质量信号只影响排序不充当关系证据；验证 context_only/possible_causal/unclear 边界测试通过
- [ ] 3.5 review judge 按 `board_brief` 比较 observations/relationships/uncertainties，跳过 legacy thesis；验证第二份简报 review 测试且 lifeline 前后快照一致

## 4. 用户选题与多假设调查链

- [ ] 4.1 先写 hypothesis parser 用例：2-4 假设、零假设必有、全宏大假设自动重试/补 H0、question generated/custom、所有假设可证据不足；验证 `go test ./internal/dataenrichment/service -run 'BoardHypothesis'` 先红后绿
- [ ] 4.2 先写研究计划用例：单一共享循环、至少一个中性查询与一个反证/替代查询、重复工具拦截、外部工具失败记录 gap、不得只用结论词搜索；验证 `go test ./internal/dataenrichment/service -run 'BoardInvestigationResearch'` 先红后绿
- [ ] 4.3 按“方法选择→假设生成”顺序实现 `data_enrichment.board_hypothesize`：方法只基于父简报+问题选择，随后产竞争假设与证据需求而不预选赢家；验证 4.1、无选择循环与方法引用快照测试通过
- [ ] 4.4 实现一个共享 investigation tool loop，统一服务全部假设并区分 support/counter/gap；内部 lane 优先但允许按问题调用 web_search/fetch_page；验证 4.2 与 max_loops 测试通过
- [ ] 4.5 实现 `data_enrichment.board_synthesize`：assessment 五态、可改写/合并/推翻假设、允许 H0 最可信或全部 insufficient；取消固定机制层/历史类比/system_reframe；验证 synthesis schema 与反证保留测试通过
- [ ] 4.6 持久化 `board_investigation` 子结果、tool_calls、method_refs 与 input_snapshot；中途失败不得留半成品；验证一 brief 多调查、跨版块 parent 拒绝、原子性测试通过
- [ ] 4.7 调查 review 仅比较相同 `parent_result_id + question_key` 的重跑；generated/custom 问题使用同一规范化 hash，不同问题不互比；验证 review kind/question 隔离测试通过
- [ ] 4.8 调整单泳道 prompt：去掉旧作者画像全局注入和“至少三类证据”配额，不引入 board hypotheses schema，冲突材料留在既有 evidence/boundary；验证 orchestrator prompt 与旧结果解析回归测试通过

## 5. API 与前端认知工作台

- [ ] 5.1 先写 handler/API 契约测试：原 trigger 返回 board_brief job、新 investigation trigger 支持 generated/custom 问题、job_id/job_kind 轮询、同 board 跨 kind 409、防断连、kind 过滤、父结果校验与旧详情兼容；验证目标 handler 包测试先红后绿
- [ ] 5.2 扩展 analysis runner：任务记录唯一 job_id/job_kind，按 job_id 查询，active key 仍按 board 串行；409 返回当前任务身份，按 board 状态入口保留用于重进恢复；验证 runner 并发/完成/错误/进程内恢复测试通过
- [ ] 5.3 实现 `POST .../analysis/investigations/trigger` 与 `GET .../results?kind=`，现有 trigger 切换简报语义并返回 board_brief job；验证 5.1 通过
- [ ] 5.4 先写 `BoardBriefReport` 组件测试：summary/观察/关系/不确定项/0-4 问题、无关系正常态、lane 下钻、深入调查 payload、自填问题；验证 Windows cmd 目标 unit 测试先红后绿
- [ ] 5.5 实现简报主视图与按 job_id 的状态恢复；board_investigation 完成不得误刷新为新 brief，“分析板块”不再出现自动论文分析文案；验证组件测试和 API mock 通过
- [ ] 5.6 先写 `BoardInvestigationReport` 测试：首屏有限结论、支持/反证/gap 分区、hypothesis assessment、证据展开、lane 下钻；验证目标 unit 测试先红后绿
- [ ] 5.7 实现调查渐进展示，移除新结果的 argument/depth 重复长文；legacy 结果继续路由旧组件并显示“旧版分析”；验证新旧三种 result_kind 分派测试通过
- [ ] 5.8 下钻 prefill 改为具体 observation/question/evidence note 且允许修改；验证前端 payload 与后端透传测试通过

## 6. 旧行为退场与兼容

- [ ] 6.1 停止新调用 `board_interpret`、旧 board agent directions 与 boardAnalyze 论文分支，但暂留 parser/组件用于 legacy 读取；验证新 trigger 调用链测试中旧 Operation 次数为 0
- [ ] 6.2 历史 `legacy_board_analysis` 列表/详情/QA/渲染回归；验证 fixture 覆盖 thesis/argument/depth 旧 JSON，无数据改写
- [ ] 6.3 删除默认启用的作者画像 seed 行为或将后续 seed 迁移为 disabled legacy；验证全新数据库初始化后不会有 enabled 作者风格 profile

## 7. 测试

- [ ] 7.1 后端影响包：`cd backend-go && go test ./internal/dataenrichment/... ./internal/platform/database/...`，期望全部通过（DB 集成测试使用 Docker PostgreSQL）
- [ ] 7.2 后端静态门禁：`cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...`，期望零错误
- [ ] 7.3 前端 lint：`cd front && pnpm lint`，期望零 error
- [ ] 7.4 前端 Windows 验证：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build"`，期望 typecheck、unit、build 全绿
- [ ] 7.5 UI 手动验收：真实板块依次走“生成简报→选择问题→深入调查→查看反证→lane 下钻→打开 legacy”，记录截图/操作结果；期望无自动调查、无强制反转标题、旧数据可读

## 8. 文档

<!-- doc-impact: flow api database architecture standard configuration deployment -->

- [ ] 8.1 更新 `docs/reference/flow/data-enrichment.md`：默认简报、显式调查、多假设/反证、方法卡阶段隔离、旧行为退场与业务约束；验证术语与 specs 一致
- [ ] 8.2 更新 `docs/reference/api/dataenrichment.md`：result_kind、parent_result_id、调查 trigger、kind filter、analysis-methods CRUD 与旧 reference endpoint 兼容期；验证请求/响应示例可对照 handler
- [ ] 8.3 更新 `docs/reference/database/DATABASE_FIELDS.md`：result_kind/parent 自关联、analysis_methods、legacy 迁移与回滚；验证迁移字段逐项对账
- [ ] 8.4 更新 `docs/reference/architecture/map.md`：版块简报和调查双入口；验证代码入口指向真实 symbol
- [ ] 8.5 运行 `bash scripts/doc-impact.sh verify openspec/changes/board-level-deep-analysis` 与 `bash scripts/check-standards.sh`，期望均退出 0

## 9. 验证

- [ ] 9.1 `openspec validate board-level-deep-analysis --strict`，期望 change 与三份 capability delta 全部有效
- [ ] 9.2 `bash scripts/scenario-trace.sh openspec/changes/board-level-deep-analysis`，期望所有 Scenario 均映射到实际测试文件
- [ ] 9.3 `bash scripts/change-scope.sh $(git diff --name-only)`，按输出复核只运行受影响包且无遗漏
- [ ] 9.4 执行第 7 节全部命令，期望后端/前端/构建全绿；不得以旧实现时期的测试结果代替本轮证据
- [ ] 9.5 部署前抽查数据库：旧 board 行均为 legacy、新 brief/investigation parent 合法、旧 reference 原文仍在且不注入；期望无需用户清理或重新生成旧数据

### Scenario → 计划测试文件映射（实现后由 scenario-trace 对账）

| Scenario 组 | 计划测试文件 |
| --- | --- |
| 简报触发/无统一关系/并行趋势/sparse/数量上限 | `backend-go/internal/dataenrichment/service/board_brief_test.go`, `enrich_board_test.go` |
| 关系枚举/幽灵 lane/因果证据不足 | `backend-go/internal/dataenrichment/service/board_brief_test.go`, `evidence_test.go` |
| 研究问题 generated/custom/空问题集/异步 job 身份 | `backend-go/internal/dataenrichment/service/board_brief_test.go`, `handler/board_enrichment_handler_test.go`, `handler/analysis_runner_test.go` |
| 零假设/竞争假设/全部不足/推翻 | `backend-go/internal/dataenrichment/service/board_investigation_test.go` |
| 支持与反证/中性搜索/外部失败 | `backend-go/internal/dataenrichment/service/board_investigation_research_test.go` |
| 父子快照/result kind/question_key/review 隔离 | `backend-go/internal/dataenrichment/repository/repository_test.go`, `service/enrich_board_test.go` |
| 方法卡 CRUD/选择/阶段隔离/legacy 迁移 | `backend-go/internal/dataenrichment/service/analysis_methods_test.go`, `handler/analysis_method_handler_test.go`, `internal/platform/database/analysis_method_migration_test.go` |
| lane/kind/新鲜度/单泳道下钻 | 既有 `evidence_test.go`, `freshness_gate_test.go`, `orchestrator_test.go` 增量用例 |
| 简报/调查/legacy 前端分派 | `front/app/features/tags/components/BoardBriefReport.test.ts`, `BoardInvestigationReport.test.ts`, `BoardAnalysisReport.test.ts` |
| 分析方法设置页 | `front/app/features/settings/components/AnalysisMethodPanel.test.ts` |
