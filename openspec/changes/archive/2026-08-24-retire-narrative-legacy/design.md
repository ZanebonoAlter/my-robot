# Design — retire-narrative-legacy

## Context

见 proposal.md Why。代码实态：`SaveNarrative`/`SaveBatchNarratives`（admin/repository/repository.go:286-296 邻域）零调用方；生成函数（`GenerateAndSaveForCategory`/`GenerateNarrativesForBoard`/`cleanEmptyBoards`/hotspot）已不存在；`GET /semantic-boards/:id/narratives`（board_crud_handler.go:136）前端零调用。20260522_0001 迁移已 DELETE `narrative_board_hotspot_threshold` 等 ai_settings 死键、并 DROP 过 narrative_boards 的部分列——ai_settings 无残留，DROP 迁移有现成范式。

## Goals / Non-Goals

**Goals**
- Phase A：死轨道全量下线（两表、模型、死方法、死路由、dump-sanitizer 条目、5 个死 specs、DATA_LIFECYCLE/ER_DIAGRAM 段落、组件改名）
- Phase B：活轨道词表收敛（flow/README 权威词表、板块→版块、叙事限指 thread/evolution narrative、「叙事工坊」登记为产品页专有名保留）

**Non-Goals**
- 不动 `/daily-reports` 族 API、BoardDailyReport/DailyReportThread 模型、日报生成管线
- 不动 evolution_narrative 活轨道（boardEnrichment.ts:346 及 BoardEnrichmentPanel/QAPanel/CausalAnalysisReport/AnalyzeRefChip/DebateSection 文案）
- 不做主 specs 全量「板块→版块」机械刷词（仅修 narrative 相关措辞；全量词表校正随后续文档 change）
- 不做任何数据迁移/导出（死数据直接 DROP）

## Decisions

### D1: DROP 两表走新 destructive 迁移，范式照抄 20260522_0001

新增 `Version: "20260824_0001"`：`DROP TABLE IF EXISTS narrative_summaries CASCADE` + `DROP TABLE IF EXISTS narrative_boards CASCADE`，自守卫 `IsDestructiveAllowed()`（部署时 `MIGRATIONS_ALLOW_DESTRUCTIVE=1`，门禁语义见 db-migration-safety spec）。CASCADE 顺带清掉历史迁移建的所有 narrative 索引。

**备选**：留孤儿表不 DROP——拒绝：单用户自部署库，无外部消费方，留着违背本次清算目的（ER 图/心智负担）。

### D2: AutoMigrate 注册同 change 删除（防重建）

migrator.go:98-99 注销两表。若只 DROP 不注销，下次启动 AutoMigrate 会按 model 重建空表——两表复活。删除顺序：迁移 DROP 与注册注销在同一 change 落地，先删 model 文件再跑迁移（model 已删则 AutoMigrate 无从重建）。

### D3: 历史迁移账本不动 → 修订：narrative 表引用处加 tableExists 守卫

postgres_migrations.go 中 narrative 索引创建（:218-243 邻域）等历史条目保留——迁移序列 append-only（无 checksum 机制但按 Version 顺序重放，改动历史条目对已应用库无收益）；DROP CASCADE 已覆盖其产物。

**实现期修订（2026-08-24）**：原判断「历史条目不动」的前提是 AutoMigrate 先建表、迁移只在表上加工。model 删除后该前提破产：空库重放（golden schema 测试 / 全新部署）在 20260420_0001 直接报 42P01（`CREATE INDEX ... ON narrative_summaries` 表不存在）。修复：对四个含 narrative 表裸引用的历史迁移加 `tableExists` 守卫（20260420_0001 / 20260430_0001 整体 skip；20260521_0001 拆出 narrative 索引行单独守卫；20260522_0001 拆出 narrative_boards 的四条 ALTER 单独守卫——PG 的 `DROP COLUMN IF EXISTS` 只在列级容错，表不存在照样炸）。表存在时行为完全不变，语义无损幂等强化，与 20260526_0001 起的守卫范式一致。

### D4: 前端组件改名走 Nuxt 自动导入裸名验证

`NarrativeGenerateDialog.vue` → `DailyReportGenerateDialog.vue`（含内部文案「生成叙事」→「生成日报」）；`BoardListSidebar.vue:133`「整理叙事」→「生成日报」。**P0 教训**：Nuxt 自动导入组件在模板中用裸名引用，build/typecheck/lint 不解析——改名后必须全库 grep `NarrativeGenerateDialog`/`narrative-generate-dialog` 双拼写验证零残留。

### D5: 词表落点与「叙事工坊」专有名

`docs/reference/flow/README.md` 新增权威中英词表节：版块=semantic board、话题=persistent topic、主题标签=topic tag、章节=section、叙事线=thread、订阅发现候选=feed recommendation。登记两条「叙事」合法用法：(a) 产品页专有名「叙事工坊」（tags 页，保留）；(b) evolution_narrative=结构演化叙述（data-enrichment 域）。其余「叙事」在 flow 文档中限指 daily_report_threads（叙事线）或改写为日报语境。

### D6: 主 specs 删除与措辞修正的归档路径

5 个整删 spec（narrative-board-generation/narrative-scope-query/board-narrative-timeline/empty-board-cleanup/daily-hotspot-board）delta 全量 REMOVED，归档 sync 时主 specs 目录整体移除；4 个措辞 spec（board-management-api/board-upgrade/tag-to-board-matching/tagging-domain）+ daily-report-system 走 MODIFIED。

## Risks / Trade-offs

- [DROP TABLE 不可逆，死数据永久丢失] → 单用户自部署 + 两表零生产写入方（审计三报告交叉验证）+ 归档 changes 可追溯设计；部署汇报中明确「旧叙事数据不可恢复」
- [MIGRATIONS_ALLOW_DESTRUCTIVE 忘开 → 迁移失败或静默跳过] → 迁移内自守卫返回明确错误信息提示环境变量；tasks 验证节含部署操作说明
- [组件改名漏引用 → 运行时 Failed to resolve component] → D4 双拼写 grep 验证；front `pnpm build` 兜底
- [词表批量替换误伤 evolution_narrative] → 禁用 sed 全局替换，逐文件人工判归属；boardEnrichment 链路文件列入白名单零改动
- [删除 admin/repository 死方法误删活跃方法] → 删除前逐方法 grep 调用方，仅删零调用者

## Migration Plan

1. 后端：删 model 文件 + AutoMigrate 注册 + admin 死方法 + 死路由 + dump-sanitizer 条目 → 编译绿
2. 新增 20260824_0001 destructive 迁移（DROP 两表）
3. 前端：组件改名 + 按钮文案 → lint/build 绿 + 裸名 grep 零残留
4. 部署：`MIGRATIONS_ALLOW_DESTRUCTIVE=1` 启动一次跑迁移（旧叙事数据永久删除，不可恢复）；后续重启无需该变量
5. 回滚策略：无（DROP 后不可逆；代码回滚会导致 AutoMigrate 重建空表但旧数据不复活——可接受，设计即不可逆清算）

## Open Questions

（无——「叙事工坊」保留已拍板；ai_settings 死键已被历史迁移清除，无需处理）
