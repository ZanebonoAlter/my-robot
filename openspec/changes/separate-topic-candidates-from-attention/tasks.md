## 1. 基线与可丢弃原型

- [x] 1.1 重建当前分支代码图并检查 `daily_report_assignment.go`、`daily_report_topic_repository.go`、`daily_report_cluster.go`、`dailyReportMagazine.ts`、`BoardDailyReportTimeline.vue` 的 affected/cycles，记录真实影响包与现有测试入口；验收：实施范围只覆盖日报分区、PersistentTopic 生命周期/选择器和必要的数据契约。
- [x] 1.2 基于只读真实数据快照做可丢弃分析，分别模拟 candidate 失活窗口 3/7/14 天、candidate 注入上限 8/12/20，输出每个 board 的保留率、被截断数、可复用候选数和潜在 auto_new 增量；原型脚本不得进入生产代码，结论记录到本 change 的 `design.md`。
- [x] 1.3 根据 1.2 校准结论确认默认 7 天/20 条，或先同步修改 proposal、design、两份 delta spec 后再进入编码；验收：实现常数与规格完全一致，不允许在代码阶段静默漂移。

## 2. 后端 TDD — 数据契约与状态机

- [x] 2.1 RED：在 `internal/topicgraph/repository` 为可锚定话题选择器补纯函数测试，覆盖“全部 active 保留”“窗口外 candidate 排除”“candidate 按 last_seen/hit_count/id 稳定排序”“上限边界”“报告日期归一化”；验收：新增测试先因功能缺失失败。
- [ ] 2.2 RED：扩展 `planLifecycle` 测试，覆盖 candidate 在窗口内 miss 仅清零、candidate/active 长期未命中也**不归档**（全人工）；验收：原自动归档用例被删除，新增"不自动归档"用例在现有自动归档实现下红。
> delta 重开：原 2.2 已按自动归档完成，现推翻为全人工归档（见 design Decision 3）。

- [ ] 2.7 GREEN：删除 `planLifecycle` 中 candidate 7 天与 active 30 天的自动归档分支，仅保留命中计数更新（命中累加、未命中清零），status 不变；同步删除 `TestPlanLifecycle_CandidateDecayBoundary`、`TestPlanLifecycle_ArchiveOnDecay`、`TestPlanLifecycle_KeepWithinDecayWindow` 等自动归档测试，替换为"长期未命中仍保持原 status"用例；保持 active 人工确认与 identity relation 行为不变。
> delta 重开：原 2.7 实现了自动归档，现删除。
- [x] 2.3 RED：补归属/持久化测试，覆盖 anchor_hit 与 auto_new 写入对应 `topic_status_at_report`、unmatched 写 NULL、话题后续状态变化不改历史快照；验收：模型/迁移实现前测试失败原因准确。
- [x] 2.4 GREEN：新增幂等版本化迁移，为 `daily_report_sections` 增加可空 `topic_status_at_report`，为 ai_settings 增加 candidate 观察窗口与 prompt 上限默认配置；更新 GORM 模型和 API JSON 契约，不回填历史快照。
- [x] 2.5 GREEN：扩展 `PersistentTopicConfig` 及加载逻辑，加入 `CandidateDecayWindow` 和 `CandidatePromptLimit`，对非法非正值使用规格默认值并记录警告。
- [x] 2.6 GREEN：实现统一的可锚定话题选择器；ClusterTags 注入和 assignment 双重确认必须复用相同选择规则，生命周期更新仍加载全部非 archived 话题。
- [x] 2.7 GREEN（初版，已被 delta 推翻）：实现 candidate 超出观察窗口自动 archived……见下方 delta 重开项。
- [x] 2.8 REFACTOR：收敛旧 `ListActiveTopicsByBoard` 的含混命名/调用点，增加 active 数、候选总数、窗口过滤数、上限截断数和 auto_new 数的结构化日志；不得输出正文、完整 prompt 或 embedding。

## 3. 前端 TDD — 阅读分区解耦

- [x] 3.1 RED：扩展 `dailyReportMagazine.test.ts`，覆盖快照 active →"关心的话题"、candidate/null →"其他动态"、candidate 不获得排序加权，以及当前 topic.status 变化不改变历史分区；验收：旧三分区实现下测试先失败。
- [x] 3.2 RED：扩展 `DailyReportTopicSection.test.ts` / `BoardDailyReportTimeline` 相关测试，断言页面不再出现"突发的新话题""Developing"或 candidate 状态徽章，candidate 仍能在"其他动态"正常阅读；验收：旧文案与旧 zone 使测试先红。
- [x] 3.3 GREEN：更新 `DailyReportSection` 类型和 normalizer，接收可空 `topic_status_at_report`，对缺失字段保持兼容。
- [x] 3.4 GREEN：把 `QualityZone` 收敛为 active/briefs 两类，按报告时快照分区；candidate、archived 和旧数据 NULL 统一进入"其他动态"，保持 `(best_tier, avg_score)` 排序。
- [x] 3.5 GREEN：更新日报侧栏、正文话题组和状态文案；仅 active 快照话题进入目录和自动展开逻辑，其他动态不得自动请求 topic lifeline。
- [x] 3.6 REFACTOR：删除本次变更造成的废弃 candidate zone 分支、类型和样式，不改动相邻的主题、文章展开或 section 生命周期交互。

## 4. 兼容性与集成

- [x] 4.1 使用 `testutil.SetupTestDB` 增加 repository 集成测试：迁移重复执行成功，新报告写入快照，历史 NULL 可查询，candidate 自动归档后退出可锚定集合，active 不受 candidate 上限影响。
- [x] 4.2 增加 service/repository 协同测试，断言同一报告日期下 ClusterTags 收到的 topic id 集合与 assignment 接受集合一致，窗口外/被截断 candidate 不会单边参与双重确认。
- [x] 4.3 更新日报 API 契约测试与前端 fixture，覆盖 snake_case 字段、NULL 兼容和 current status 与 snapshot status 不一致的历史案例。
- [x] 4.4 更新受影响的日报 magazine E2E fixture/断言，验证 candidate 内容可读但不形成独立注意力区；不新增脆弱的整页视觉快照。

## 5. 测试

- [x] 5.1 后端 RED/GREEN 过程只运行影响包：`go test ./internal/topicgraph/repository ./internal/topicgraph/service`；验收：新增测试先红，完成实现后全部 PASS。
- [x] 5.2 前端 RED/GREEN 过程通过 Windows cmd 运行受影响 Vitest 文件：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit -- app/features/tags/components/daily-report/dailyReportMagazine.test.ts app/features/tags/components/daily-report/DailyReportTopicSection.test.ts"`；验收：新增测试先红，完成实现后 PASS。
- [x] 5.3 数据库集成测试在 Docker/testcontainer 可用时运行 `go test ./internal/topicgraph/repository -run "Test.*(PersistentTopic|TopicAssignment|Candidate)" -count=1`；验收：迁移幂等、NULL 兼容、快照稳定和 candidate 归档全部 PASS。

## 6. 文档

- [x] 6.1 更新 `docs/reference/flow/` 日报生成/阅读流程：candidate 是内部叙事观察态，日报仅有"关心的话题/其他动态"两类主阅读区；补充报告时快照节点。
- [x] 6.2 更新 `docs/reference/architecture/`：说明可锚定话题选择器由 ClusterTags 与 assignment 共享，并明确 PersistentTopic 身份系统与 topic watch 注意力系统的边界。
- [x] 6.3 更新 `docs/reference/database/`、`api/` 和 `configuration.md`：记录 `topic_status_at_report`、两个 candidate 配置、NULL 兼容、排序与窗口边界。
- [x] 6.4 对照实现更新本 change 的 design/spec/tasks 勾选与校准证据，检查 `docs/reference/` 不再把 candidate 描述为"突发的新话题"。

## 7. 验证（初版，delta 返工后需重跑）

- [ ] 7.1 `cd backend-go && golangci-lint run ./...`；期望：退出码 0，无新增 lint 错误。
- [ ] 7.2 `cd backend-go && go vet ./...`；期望：退出码 0。
- [ ] 7.3 `cd backend-go && go test ./internal/topicgraph/repository ./internal/topicgraph/service`；期望：影响包全部 PASS。
- [ ] 7.4 `cd backend-go && go build ./...`；期望：退出码 0。
- [ ] 7.5 `cd front && pnpm lint`；期望：退出码 0。
- [ ] 7.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`；期望：退出码 0。
- [ ] 7.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`；期望：全部 Vitest 用例 PASS。
- [ ] 7.8 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`；期望：生产构建成功。
- [ ] 7.9 `rg -n -F "突发的新话题" front/app docs/reference`；期望：零命中，或仅存在明确说明该旧语义已废弃的迁移文档命中。
- [ ] 7.10 `openspec validate separate-topic-candidates-from-attention --type change --strict`；期望：change valid。
- [ ] 7.11 `git diff --check`；期望：退出码 0，无空白错误；随后确认 proposal/design/specs/tasks 与 `docs/reference/` 已同步、任务全部勾选，再执行归档流程。

## 8. Delta 返工 — 全人工归档 + 候选展示门槛 + 历史清理

> 本节推翻 design Decision 3 的初版自动归档设计（依据真实数据观察：222 条 candidate 中 139 条为一次性 candidate，自动归档门槛与本 change 核心目标不一致）。spec 已同步重写。

### 后端 — 删除自动归档

- [x] 8.1 RED：为 `planLifecycle` 新增"长期未命中不归档"用例——candidate/active 距 last_seen 超过任意天数（7/30/60 天），执行状态机更新后 status 保持不变；验收：在现有自动归档实现下这些用例红（会转 archived）。
- [x] 8.2 GREEN：删除 `planLifecycle`（`daily_report_assignment.go`）中 candidate 7 天自动归档分支与 active 30 天自动归档分支，仅保留命中计数逻辑（命中 +1、未命中清零、status 不变）；删除对应的 `TestPlanLifecycle_CandidateDecayBoundary`、`TestPlanLifecycle_ArchiveOnDecay`、`TestPlanLifecycle_KeepWithinDecayWindow`；确认 8.1 新用例转绿。
- [x] 8.3 顺手修正：`DeleteTopic`（`daily_report_topic_repository.go`）置 NULL 时连同 `topic_status_at_report` 一起置 NULL（当前漏了，会留下 snapshot≠NULL 但 topic_id=NULL 的脟脏数据）。

### 后端 — 候选展示门槛（observing 隐藏）

- [x] 8.4 RED：为 `listBoardTopics`（`handler/daily_report_handler.go`）补测试，断言 `status=candidate AND consecutive_hits < upgrade_threshold` 的 topic 不出现在响应中，而 active/archived 与达门槛 candidate 出现；验收：未加过滤前红。
- [x] 8.5 GREEN：在 `listBoardTopics` 组装响应前过滤掉 observing candidate（`t.Status==candidate && t.ConsecutiveHits < upgradeThreshold`）；active 与 archived 不过滤。

### 后端 — 一次性清理迁移

- [x] 8.6 新增幂等迁移（`platform/database/postgres_migrations.go`）：删除所有 `status='candidate' AND consecutive_hits < upgrade_threshold` 的 topic，采用与 `DeleteTopic` 一致语义——先 UPDATE 被引用 section 的 `persistent_topic_id`/`topic_match_distance`/`topic_match_confidence`/`topic_status_at_report` 置 NULL，再 DELETE topic 行，最后对每个受影响 board 重建 relations。
- [x] 8.7 RED→GREEN：新增集成测试 `TestCleanup_PruneUnderqualifiedCandidates`——插入 active、达门槛 candidate、未达门槛 candidate（含被 section 引用的），跑迁移后断言：未达门槛者硬删、section 的归属字段置 NULL 但内容/渲染正常、达门槛与 active/archived 保留、二次执行为 no-op。

### 文档

- [ ] 8.8 更新 `docs/reference/flow/topic-graph.md`：明确生命周期"仅命中计数、归档全人工"；补"候选展示门槛：observing 隐藏，达门槛自动可见"；说明 auto_new 创建门槛与清理迁移。
- [ ] 8.9 更新 `docs/reference/architecture/`、`configuration.md`：删除 active 30 天 decay_window 的自动归档描述，明确 candidate_decay_window 仅为 prompt 卫生过滤；记录展示门槛语义。
- [ ] 8.10 更新 `docs/reference/database/`：记录一次性清理迁移的行为与幂等性。

### AGENTS.md 汇报原则

- [ ] 8.11 在 `AGENTS.md` AI Behavior Rules 补一条汇报原则：完工或开工前必须主动汇报"部署后影响 + 需要的用户操作"，避免误会；规则表述见 design Open Questions。

### 验证

- [ ] 8.12 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/repository ./internal/topicgraph/service -count=1"`；期望全 PASS。
- [ ] 8.13 部署后人工核验（需用户操作）：重新生成一份日报，确认话题管理界面仅显示 active/archived/达门槛 candidate；确认 observing candidate 不可见；确认 section 渲染正常。
