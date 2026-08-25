# Tasks — fix-quality-audit-p0

> 三项互相独立，组内按「用例先行」执行：P0-1 是行为 bug，先写复现测试再修；P0-2 对账提取为可测小函数；P0-3 纯删除靠编译/静态检查兜底。

## 1. P0-1 AI 摘要开关读键修复（前端 bug）

- [x] 1.1 写复现测试：在 `front/app/stores/api.test.ts` 增加/修改用例——mock 后端返回 `article_summary_enabled: false`，断言映射后 feed 的 AI 摘要开关为 false；另加缺省字段用例（断言 false 而非 true）。先跑确认红（复现 bug）
- [x] 1.2 修 `front/app/stores/api.ts:148`：`aiSummaryEnabled` 映射改读 `feed.article_summary_enabled ?? false`，删除写死 true 的回退；跑 1.1 用例确认绿
- [x] 1.3 清死字段：`front/app/types/feed.ts` 删 `RssFeed.aiSummaryEnabled`（:53）与 `UpdateFeedData.ai_summary_enabled`（:88）；`front/app/stores/api.ts:22` 响应接口删 `ai_summary_enabled` 键。grep `aiSummaryEnabled` 全部消费点，确认仅映射处与 1.4 的 union type 引用（如有外部消费方按 design D3 风险预案处理并在此标注）
- [x] 1.4 清死 key：`front/app/composables/useGlobalSettings.ts:54` 与 `front/app/features/settings/components/FeedDetailEditor.vue:16` 的 setting union type 删 `'ai_summary_enabled'`；useGlobalSettings 内 legacy `aiSummaryEnabled` ref（:127,:136）若无外部消费一并移除

## 2. P0-2 feed_count 周期对账（后端）

- [x] 2.1 在 `backend-go/internal/admin/scheduler/job_tag_quality_score.go` 将对账实现为导出小函数 `RecalculateTopicTagFeedCounts(db *gorm.DB) error`（全量 UPDATE + COUNT DISTINCT，容错照 ref_count 模式由调用方记 warning），job 主函数在 quality score 计算后调用
- [x] 2.2 写集成测试（`job_tag_quality_score_feed_count_test.go`，用 `testutil.SetupTestDB`）：造 tag A 关联 2 个不同 feed 的文章、tag B 关联 1 个 feed、tag C 无关联且 feed_count 预置脏值 → 调 `RecalculateTopicTagFeedCounts` → 断言 A=2、B=1、C=0；覆盖 tagging-domain 三个 Scenario 中前两个
- [x] 2.3 对账失败容错用例：模拟 SQL 失败（如删表/断连）断言 job 不中断、记录 warning（覆盖 Scenario 3；实现成本高则允许以代码走查 + ref_count 同模式论证替代，在此说明）

## 3. P0-3 死组件删除（前端清理）——【结论反转：未执行】

> ⚠️ 实施中推翻：审计的「零引用」判断是误判。两个 grep 叠加 bug：① 复核时 `grep -v "components/"` 误杀了 `features/*/components/` 路径下的真实消费方（SettingsSectionFeeds 等）；② Nuxt 自动导入名按目录前缀（DialogXxx）计算，而模板实际用裸名（`<AddFeedDialog>`）。运行时验证（用户 dev 环境报 `Failed to resolve component`）确认 6 个组件全部活跃：AddFeedDialog/AddCategoryDialog/ImportOpmlDialog（SettingsSectionFeeds.vue:130/135/140）+ AddFeedDialog/EditCategoryDialog/EditFeedDialog（FeedLayoutShell.vue:506/513/521）+ TopicManageDialog（TopicDetectiveWall.client.vue:849）。已全部 git 恢复。教训：Nuxt 自动导入组件的引用判定不能靠静态 grep（build/typecheck 也不报，运行时才解析）——需运行时或 nuxt 组件清单验证。

- [x] 3.1 删除 6 个零引用组件 →【已回滚】组件全部活跃，不删
- [x] 3.2 删除后验证 → 教训：lint/typecheck/build 全绿不足以证明自动导入组件无引用

## 4. 文档

<!-- doc-impact: flow database -->
<!-- doc-impact-excuse: api=工作区 api 域改动属并行 change，本 change 未碰; architecture=同上，并行工作区脏改动; standard=同上，并行工作区脏改动; configuration=同上，并行工作区脏改动 -->

- [x] 4.1 `openspec/specs/unified-dialog/spec.md` Migration Map Pattern A 涉及清单标注 →【已回滚该标注】原标 5 个 dialog 已移除，结论反转后撤销（组件活跃，见组 3 结论反转说明）
- [x] 4.2 `docs/reference/flow/reading.md` 如有 AI 摘要开关默认值/字段名描述则核对更新（grep `ai_summary\|article_summary` 确认；无则仅记录核对结论）
- [x] 4.3 `docs/reference/flow/semantic-board.md` 或 topic-graph.md 的标签列表排序描述处，补充 feed_count 为周期对账维护的反规范化计数（grep `feed_count` 定位；无描述则跳过）

## 5. 测试

- 影响包（change-scope）：前端 stores/types/composables/components/dialog + 后端 internal/admin/scheduler
- 后端：`go test ./internal/admin/scheduler`（含新集成测试，需 Docker DB；-short 自动跳过走门禁）
- 前端：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"`（含 stores/api.test.ts 新用例）
- Scenario → 测试映射（验收基准）：

| Delta Scenario | 测试 |
| --- | --- |
| feed-settings-ui: feed 开关状态真实反映后端值 | stores/api.test.ts 新用例「开关反映 article_summary_enabled」 |
| feed-settings-ui: 字段缺省时按后端默认处理 | stores/api.test.ts 新用例「缺省字段回退 false」 |
| feed-settings-ui: 死字段不再存在于类型定义 | `nuxi typecheck` 编译期保证（类型删除后任何残留引用即报错） |
| tagging-domain: 打标漂移后被对账修正 | job_tag_quality_score_feed_count_test.go「A=2/B=1」断言 |
| tagging-domain: 无引用标签对账为零 | 同上「C=0」断言 |
| tagging-domain: 对账失败不中断 job | job_tag_quality_score_feed_count_test.go 容错用例（或 2.3 走查论证） |

## 6. 验证

- [x] 6.1 `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` → 期望零报错（实测：0 issues / vet 零输出 / build OK）
- [x] 6.2 `cd backend-go && go test ./internal/admin/scheduler` → 期望全绿（实测：ok 6.590s，含 feed_count 对账两个新集成测试）
- [x] 6.3 `cd front && pnpm lint` → 期望零报错（实测：0 errors，5 个 warning 均为无关文件存量问题）
- [x] 6.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 期望零报错（实测通过，含死字段删除后的全库引用检查）
- [x] 6.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` → 期望全绿（实测：523/523，含 1.1 复现用例）
- [x] 6.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 期望构建成功（实测：Build complete，1.73 MB）
- [x] 6.7 手动验证：已用 opencli（前台模式）实测 settings 页「添加订阅源/添加分类/导入」三个 dialog 均正常弹出（对话框标题分别为「添加订阅源」「添加分类」「导入 OPML」，console 零报错）；feed_count 对账由集成测试锁定。附注：用户反馈 dialog 样式观感待优化（kimi 视觉判断因额度不可用未执行），已记为后续待办，不阻塞归档
