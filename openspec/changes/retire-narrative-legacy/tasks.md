# Tasks — retire-narrative-legacy

## 1. Phase A 后端死轨道清算

- [ ] 1.1 删除 `backend-go/internal/models/narrative.go` + `narrative_board.go`，同步删除 `internal/platform/database/migrator.go:98-99` 的 AutoMigrate 注册 → `go build ./...` 绿（model 已删则 AutoMigrate 无从重建，DROP 后不复活）
- [ ] 1.2 删除 `admin/repository/repository.go` 中 SaveNarrative / SaveBatchNarratives / Delete 等死方法——删前逐方法 grep 调用方确认零引用（仅删零调用者，防误删活跃方法）→ `go build ./...` 绿
- [ ] 1.3 删除死路由：`tagmanagement/handler/board_crud_handler.go:136` 的 `boards.GET("/:id/narratives", ...)` 注册 + `getBoardNarratives` handler（:342 邻域）+ 其手写 JSON 数组解析（:383-389 邻域）→ `grep -rn 'narratives' internal/tagmanagement/` 零命中
- [ ] 1.4 `cmd/dump-sanitizer/tables.go` 删除 narrative_boards（:62）与 narrative_summaries（:161）两表条目 → `go build ./cmd/...` 绿 + `grep -n narrative tables.go` 零命中
- [ ] 1.5 新增 destructive 迁移 `20260824_0001`（DROP TABLE IF EXISTS narrative_summaries / narrative_boards CASCADE，`IsDestructiveAllowed()` 自守卫返回带环境变量提示的明确错误；范式照抄 20260522_0001）→ 迁移单测/本地 Docker 验证：allow=1 时两表消失，未设置时启动报明确错误且不崩库

## 2. Phase A 前端

- [ ] 2.1 `git mv` `NarrativeGenerateDialog.vue` → `DailyReportGenerateDialog.vue`，内部文案「叙事」→「日报」语境改写 → `grep -rn 'NarrativeGenerateDialog\|narrative-generate-dialog' front/app/` 零命中（双拼写，P0 教训：自动导入裸名运行时解析，静态全查）
- [ ] 2.2 定位并更新全部调用方模板引用（裸名 `<NarrativeGenerateDialog>` → `<DailyReportGenerateDialog>`，grep 定位消费组件逐一改）→ `pnpm build` 绿
- [ ] 2.3 `BoardListSidebar.vue:133`「整理叙事」→「生成日报」（emit 事件名 `open-generate` 不动）→ 目视/grep 确认

## 3. Phase A 数据库文档

- [ ] 3.1 `docs/reference/database/DATA_LIFECYCLE.md` 删除「叙事生成生命周期」整段（:231-278）+ :14 「进入叙事摘要」链路句改写为日报语境 → 段内 grep `narrative_` 零命中
- [ ] 3.2 `docs/reference/database/ER_DIAGRAM.md` 删除 narrative 两表节点（:65/:67）+ :85 逻辑引用句中的 narrative_boards → grep 零命中（user_preferences 属 P2 顺手项，此处不扩）

## 4. Phase B 词表收敛（严禁误伤活轨道）

- [ ] 4.1 `docs/reference/flow/README.md` 新增权威中英词表节：版块=semantic board、话题=persistent topic、主题标签=topic tag、章节=section、叙事线=thread、订阅发现候选=feed recommendation；登记「叙事」两条合法用法：(a) 产品页专有名「叙事工坊」（保留）、(b) evolution_narrative=结构演化叙述（data-enrichment 域）
- [ ] 4.2 flow 文档「板块」→「版块」：semantic-board.md（×29）、discovery.md（×1）逐处替换并目视复核语境（禁 sed 盲刷）
- [ ] 4.3 daily-report.md「叙事」×22、scheduler.md「叙事」×4 逐条判归属：限指 daily_report_threads（叙事线）的保留、日报语境的改写「日报/线索」、无归属的直接删 → 判定清单落 tasks 或 design 附录
- [ ] 4.4 白名单零改动验证：`git diff --stat` 确认 boardEnrichment.ts、BoardEnrichmentPanel.vue、QAPanel.vue、CausalAnalysisReport.vue、AnalyzeRefChip、DebateSection 不在改动清单

## 5. 文档

<!-- doc-impact: flow database -->

- [ ] 5.1 `docs/reference/database/DATABASE_FIELDS.md` 如仍含 narrative 两表字段表则同步删除（grep 定位；无则记录核对结论）
- [ ] 5.2 flow/README.md 词表节（4.1 交付）注册进文档索引（若有目录结构要求）
- [ ] 5.3 归档时主 specs 清理验证：5 个整删 capability 目录（narrative-board-generation / narrative-scope-query / board-narrative-timeline / empty-board-cleanup / daily-hotspot-board）在 sync/archive 后从 `openspec/specs/` 消失；若 sync 残留空壳 spec.md 则手动删目录

## 6. 测试

- 影响包（change-scope 机械判定）：后端 `internal/models` `internal/admin` `internal/tagmanagement` `internal/platform/database` `cmd/dump-sanitizer` + 前端 `features/tags` `features/shell`
- 后端 targeted：`go test ./internal/tagmanagement/... ./internal/platform/database ./internal/admin/... ./cmd/dump-sanitizer/...`（DB 集成测试需 Docker DB；-short 自动跳过走门禁）
- 前端：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"`
- 本 change 无新增测试义务（纯删除+文案；MODIFIED delta 均为措辞修正，行为不变，由既有测试锁定）

## 7. 验证

- Scenario → 测试映射（验收基准）：

| Scenario | 测试文件 |
| --- | --- |
| 手动创建板块 | backend-go/internal/tagmanagement/handler/semantic_board_handler_test.go |
| 列出板块 | backend-go/internal/tagmanagement/handler/semantic_board_handler_test.go |
| 冷启动无 board | backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go |
| 冷启动初始化建议 | backend-go/internal/tagmanagement/service/board/semantic_board_upgrade_test.go |
| 多视角挂载含降级标记 | backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go |
| 超过归属上限时截断 | backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go |
| 改 TopicTag 不触发 feed 重编译 | 人工（结构走查：narrative model 行删除后全库编译绿即证） |
| 改 Feed 不触发 tagging 重编译 | 人工（同上） |
| TopicTagEmbedding 无 Vector 字段 | 人工（struct 定义走查；本 change 不触碰该 struct） |
| 创建日报记录 | backend-go/internal/topicgraph/repository/daily_report_topic_integration_test.go |
| 日报记录唯一性 | 人工（SaveReport 同日 upsert 走查 daily_report_repository.go；无既有直接断言） |
| 日报关联昨日报告 | 人工（daily_report_orchestrator.go:206 prev 链接走查；无既有断言） |
| 线程存储在独立表中 | backend-go/internal/topicgraph/repository/daily_report_repository_test.go |

- [ ] 7.1 `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` → 全绿
- [ ] 7.2 `cd backend-go && go test ./internal/tagmanagement/... ./internal/platform/database ./internal/admin/... ./cmd/dump-sanitizer/...`（Docker DB up）→ 全绿
- [ ] 7.3 迁移演练：本地 Docker DB（`docker compose -f docker-compose.pg.yml up -d`）先以默认配置启动 → 期望 20260824_0001 报「需 MIGRATIONS_ALLOW_DESTRUCTIVE=1」明确错误；再以 `MIGRATIONS_ALLOW_DESTRUCTIVE=1` 启动 → 期望 `\dt` 确认 narrative_summaries / narrative_boards 两表消失、其余表无损
- [ ] 7.4 `cd front && pnpm lint` → 零报错
- [ ] 7.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 零报错
- [ ] 7.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` → 全绿
- [ ] 7.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 构建成功（组件改名后全库引用解析兜底）
- [ ] 7.8 残留一致性：`grep -rn 'narrative' backend-go/internal backend-go/cmd --include='*.go' | grep -v _test | grep -vi evolution | grep -v postgres_migrations.go` → 零命中（历史迁移账本按 design D3 保留）；`grep -rn 'NarrativeGenerateDialog\|narrative-generate-dialog' front/app/` → 零命中
- [ ] 7.9 `bash scripts/scenario-trace.sh openspec/changes/retire-narrative-legacy` → 退出码 0
- [ ] 7.10 归档门禁：`bash scripts/doc-impact.sh verify` + `bash scripts/check-standards.sh` → 通过
- [ ] 7.11 部署操作提示（完工汇报必含）：部署需一次 `MIGRATIONS_ALLOW_DESTRUCTIVE=1` 启动跑迁移，旧叙事数据（narrative_summaries/narrative_boards 全部行）永久删除不可恢复；之后重启无需该变量；前端「整理叙事」按钮与「叙事生成」对话框更名为「生成日报」，行为不变（实调日报生成 API）
