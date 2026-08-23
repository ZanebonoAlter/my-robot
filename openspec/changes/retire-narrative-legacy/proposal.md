<!-- constraint-domains: semantic-board, daily-report, data-enrichment -->

## Why

质量审计三份报告（DB 冗余 H1 / 术语漂移 #1 / 前端耦合）独立收敛到同一块最大纯负担：narrative 遗留双轨。`narrative_summaries`/`narrative_boards` 两表零生产写入方（`SaveNarrative` 零调用），生成函数（`GenerateAndSaveForCategory`/`GenerateNarrativesForBoard`/`cleanEmptyBoards`/hotspot）早已全部删除；死路由 `GET /semantic-boards/:id/narratives` 前端零调用；前端 `NarrativeGenerateDialog.vue` 名为「叙事生成」实际调用日报生成 API，「整理叙事」按钮触发的是日报——UI 语义误导；`DATA_LIFECYCLE.md:231-278` 仍在描述已删除的生成流程；5 个 legacy specs 仍在要求已死行为。本次一并收敛词表（「版块」统一、「叙事」限指 thread / evolution narrative 活概念）。

## What Changes

### Phase A — 死轨道清算

- **BREAKING**：DROP TABLE `narrative_summaries` + `narrative_boards`（destructive 迁移，走 `MIGRATIONS_ALLOW_DESTRUCTIVE` 门禁；两表已无生产写入方，数据为死数据，不迁移不降级）
- 删除 `models/narrative.go`、`models/narrative_board.go` 与 migrator.go 的 AutoMigrate 注册
- 删除 `admin/repository` 的 SaveNarrative / SaveBatchNarratives / Delete 等死方法
- 删除死路由 `GET /semantic-boards/:id/narratives`（board_crud_handler.go 的路由注册 + getBoardNarratives handler 及其手写 JSON 解析）
- `dump-sanitizer/tables.go` 删除两表条目（:62/:161）
- 历史迁移中 narrative 索引创建与 hotspot seed 键的处置（保留不动 vs 清理）→ design 决策点
- 前端 `NarrativeGenerateDialog.vue` → `DailyReportGenerateDialog.vue`（改名+文案改「生成日报」），「整理叙事」按钮（BoardListSidebar.vue:133）文案同步
- 删除 5 个死轨道主 specs（见 Capabilities/REMOVED，其中 empty-board-cleanup、daily-hotspot-board 经验证其全部 Requirement 描述的生成函数在代码中已不存在）
- `DATA_LIFECYCLE.md` 删除「叙事生成生命周期」整段（:231-278）及 :14 的「叙事摘要」链路句；`ER_DIAGRAM.md` 删除 narrative 两表（:65/:67/:85）

### Phase B — 活轨道词表与文案（严禁误伤）

- `docs/reference/flow/README.md` 落权威中英词表（版块=semantic board、话题=persistent topic、主题标签=topic tag、章节=section、叙事线=thread、订阅发现候选=feed recommendation 等）
- flow 文档「板块」→「版块」统一（semantic-board.md 板块×29、discovery.md 板块×1 等）；daily-report.md「叙事」×22 / scheduler.md「叙事」×4 逐条判归属：限指 thread 或改写为日报语境
- UI「叙事」文案逐条判归属：「叙事工坊」页面名（AppSidebarView/TagsPage/useOnboarding）改名待用户拍板；`evolution_narrative`（结构演化叙述）属 data-enrichment 活概念，boardEnrichment.ts:346、BoardEnrichmentPanel/QAPanel/CausalAnalysisReport/AnalyzeRefChip/DebateSection 中的相关文案**保留不动**
- 4 个活 specs 的过时措辞 delta 修正（见 Capabilities/MODIFIED）

## Capabilities

### New Capabilities

（无——本 change 只做下线与收敛，无新能力）

### Modified Capabilities

- `board-management-api` — Purpose 中「避免与每日叙事板 /api/narratives/boards 混淆」的历史对比句删除
- `board-upgrade` — 冷启动 Requirement 中「NarrativeBoard 生成 SHALL 跳过」措辞过时，改为日报语境
- `tag-to-board-matching` — 「同一 event tag 及其文章在多个 NarrativeBoard 中重复出现」措辞更新
- `tagging-domain` — 模型清单中「narrative/model.go 持有 NarrativeSummary、NarrativeBoard」行删除
- `daily-report-system` — 「独立于旧 narrative_boards/narrative_summaries」对比句删除；「板块动态」→「版块动态」

### Removed Capabilities（整册删除，归档时从主 specs 移除）

- `narrative-board-generation` — 生成函数已全部不存在
- `narrative-scope-query` — 死端点查询行为
- `board-narrative-timeline` — 死端点 `GET /semantic-boards/:id/narratives` 行为
- `empty-board-cleanup` — `GenerateAndSaveForCategory` 等函数已全部不存在
- `daily-hotspot-board` — hotspot NarrativeBoard 生成已删除（`narrative_board_hotspot_threshold` 仅剩迁移 seed 残留）

## Impact

- **后端**：models、admin/repository、tagmanagement/handler、platform/database（migrator + 新 destructive 迁移）、cmd/dump-sanitizer
- **前端**：features/tags（NarrativeGenerateDialog 改名、BoardListSidebar 按钮）、features/shell（AppSidebarView）、composables/useOnboarding、docs 文案
- **数据库**：DROP 两表（需 `MIGRATIONS_ALLOW_DESTRUCTIVE=1` 部署时手动开启；表内为死数据，无降级路径、无回填需求）
- **文档**：DATA_LIFECYCLE.md、ER_DIAGRAM.md、flow/（README + semantic-board/discovery/daily-report/scheduler 词表校正）
- **API**：删除 `GET /api/semantic-boards/:id/narratives`（前端零调用，无兼容层）
- **风险边界**：evolution_narrative 活轨道组件与 data-enrichment 行为零改动；`/daily-reports` 族 API 零改动；BoardDailyReport/DailyReportThread 模型零改动
