## 1. 基线与可丢弃原型

- [x] 1.1 重建当前分支代码图并检查 `daily_report_assignment.go`、`daily_report_topic_repository.go`、`daily_report_cluster.go`、`dailyReportMagazine.ts`、`BoardDailyReportTimeline.vue` 的 affected/cycles，记录真实影响包与现有测试入口；验收：实施范围只覆盖日报分区、PersistentTopic 生命周期/选择器和必要的数据契约。
- [x] 1.2 基于只读真实数据快照做可丢弃分析，分别模拟 candidate 失活窗口 3/7/14 天、candidate 注入上限 8/12/20，输出每个 board 的保留率、被截断数、可复用候选数和潜在 auto_new 增量；原型脚本不得进入生产代码，结论记录到本 change 的 `design.md`。
- [x] 1.3 根据 1.2 校准结论确认默认 7 天/20 条，或先同步修改 proposal、design、两份 delta spec 后再进入编码；验收：实现常数与规格完全一致，不允许在代码阶段静默漂移。

## 2. 后端 TDD — 数据契约与状态机

- [x] 2.1 RED：在 `internal/topicgraph/repository` 为可锚定话题选择器补纯函数测试，覆盖“全部 active 保留”“窗口外 candidate 排除”“candidate 按 last_seen/hit_count/id 稳定排序”“上限边界”“报告日期归一化”；验收：新增测试先因功能缺失失败。
- [x] 2.2 RED：扩展 `planLifecycle` 测试，覆盖 candidate 在窗口内 miss 仅清零、`gap == window` 保留、`gap > window` 归档，以及 active 仍使用独立 30 天窗口；验收：新增 candidate 归档用例先红。
- [x] 2.3 RED：补归属/持久化测试，覆盖 anchor_hit 与 auto_new 写入对应 `topic_status_at_report`、unmatched 写 NULL、话题后续状态变化不改历史快照；验收：模型/迁移实现前测试失败原因准确。
- [x] 2.4 GREEN：新增幂等版本化迁移，为 `daily_report_sections` 增加可空 `topic_status_at_report`，为 ai_settings 增加 candidate 观察窗口与 prompt 上限默认配置；更新 GORM 模型和 API JSON 契约，不回填历史快照。
- [x] 2.5 GREEN：扩展 `PersistentTopicConfig` 及加载逻辑，加入 `CandidateDecayWindow` 和 `CandidatePromptLimit`，对非法非正值使用规格默认值并记录警告。
- [x] 2.6 GREEN：实现统一的可锚定话题选择器；ClusterTags 注入和 assignment 双重确认必须复用相同选择规则，生命周期更新仍加载全部非 archived 话题。
- [x] 2.7 GREEN：实现 candidate 超出观察窗口自动 archived，并在 section 归属事务中同步写入 `topic_status_at_report`；保持 active 人工确认、active decay 和 identity relation 行为不变。
- [x] 2.8 REFACTOR：收敛旧 `ListActiveTopicsByBoard` 的含混命名/调用点，增加 active 数、候选总数、窗口过滤数、上限截断数和 auto_new 数的结构化日志；不得输出正文、完整 prompt 或 embedding。

## 3. 前端 TDD — 阅读分区解耦

- [ ] 3.1 RED：扩展 `dailyReportMagazine.test.ts`，覆盖快照 active →“关心的话题”、candidate/null →“其他动态”、candidate 不获得排序加权，以及当前 topic.status 变化不改变历史分区；验收：旧三分区实现下测试先失败。
- [ ] 3.2 RED：扩展 `DailyReportTopicSection.test.ts` / `BoardDailyReportTimeline` 相关测试，断言页面不再出现“突发的新话题”“Developing”或 candidate 状态徽章，candidate 仍能在“其他动态”正常阅读；验收：旧文案与旧 zone 使测试先红。
- [ ] 3.3 GREEN：更新 `DailyReportSection` 类型和 normalizer，接收可空 `topic_status_at_report`，对缺失字段保持兼容。
- [ ] 3.4 GREEN：把 `QualityZone` 收敛为 active/briefs 两类，按报告时快照分区；candidate、archived 和旧数据 NULL 统一进入“其他动态”，保持 `(best_tier, avg_score)` 排序。
- [ ] 3.5 GREEN：更新日报侧栏、正文话题组和状态文案；仅 active 快照话题进入目录和自动展开逻辑，其他动态不得自动请求 topic lifeline。
- [ ] 3.6 REFACTOR：删除本次变更造成的废弃 candidate zone 分支、类型和样式，不改动相邻的主题、文章展开或 section 生命周期交互。

## 4. 兼容性与集成

- [x] 4.1 使用 `testutil.SetupTestDB` 增加 repository 集成测试：迁移重复执行成功，新报告写入快照，历史 NULL 可查询，candidate 自动归档后退出可锚定集合，active 不受 candidate 上限影响。
- [x] 4.2 增加 service/repository 协同测试，断言同一报告日期下 ClusterTags 收到的 topic id 集合与 assignment 接受集合一致，窗口外/被截断 candidate 不会单边参与双重确认。
- [x] 4.3 更新日报 API 契约测试与前端 fixture，覆盖 snake_case 字段、NULL 兼容和 current status 与 snapshot status 不一致的历史案例。
- [ ] 4.4 更新受影响的日报 magazine E2E fixture/断言，验证 candidate 内容可读但不形成独立注意力区；不新增脆弱的整页视觉快照。

## 5. 测试

- [x] 5.1 后端 RED/GREEN 过程只运行影响包：`go test ./internal/topicgraph/repository ./internal/topicgraph/service`；验收：新增测试先红，完成实现后全部 PASS。
- [ ] 5.2 前端 RED/GREEN 过程通过 Windows cmd 运行受影响 Vitest 文件：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit -- app/features/tags/components/daily-report/dailyReportMagazine.test.ts app/features/tags/components/daily-report/DailyReportTopicSection.test.ts"`；验收：新增测试先红，完成实现后 PASS。
- [x] 5.3 数据库集成测试在 Docker/testcontainer 可用时运行 `go test ./internal/topicgraph/repository -run "Test.*(PersistentTopic|TopicAssignment|Candidate)" -count=1`；验收：迁移幂等、NULL 兼容、快照稳定和 candidate 归档全部 PASS。

## 6. 文档

- [ ] 6.1 更新 `docs/reference/flow/` 日报生成/阅读流程：candidate 是内部叙事观察态，日报仅有“关心的话题/其他动态”两类主阅读区；补充报告时快照节点。
- [ ] 6.2 更新 `docs/reference/architecture/`：说明可锚定话题选择器由 ClusterTags 与 assignment 共享，并明确 PersistentTopic 身份系统与 topic watch 注意力系统的边界。
- [ ] 6.3 更新 `docs/reference/database/`、`api/` 和 `configuration.md`：记录 `topic_status_at_report`、两个 candidate 配置、NULL 兼容、排序与窗口边界。
- [ ] 6.4 对照实现更新本 change 的 design/spec/tasks 勾选与校准证据，检查 `docs/reference/` 不再把 candidate 描述为“突发的新话题”。

## 7. 验证

- [ ] 7.1 `cd backend-go && golangci-lint run ./...`；期望：退出码 0，无新增 lint 错误。
- [ ] 7.2 `cd backend-go && go vet ./...`；期望：退出码 0。
- [ ] 7.3 `cd backend-go && go test ./internal/topicgraph/repository ./internal/topicgraph/service`；期望：影响包全部 PASS，不运行无关 Go 全量测试。
- [ ] 7.4 `cd backend-go && go build ./...`；期望：退出码 0。
- [ ] 7.5 `cd front && pnpm lint`；期望：退出码 0。
- [ ] 7.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`；期望：退出码 0。
- [ ] 7.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`；期望：全部 Vitest 用例 PASS。
- [ ] 7.8 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`；期望：生产构建成功。
- [ ] 7.9 `rg -n -F "突发的新话题" front/app docs/reference`；期望：零命中，或仅存在明确说明该旧语义已废弃的迁移文档命中。
- [ ] 7.10 `openspec validate separate-topic-candidates-from-attention --type change --strict`；期望：change valid，无规格结构错误。
- [ ] 7.11 `git diff --check`；期望：退出码 0，无空白错误；随后确认 proposal/design/specs/tasks 与 `docs/reference/` 已同步、任务全部勾选，再执行归档流程。
