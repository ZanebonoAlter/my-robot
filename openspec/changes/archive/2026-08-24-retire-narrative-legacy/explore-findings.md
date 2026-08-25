
## retire-narrative-legacy 删除面核验结果（全部断言属实 + 3 处规格 bug）

## retire-narrative-legacy 实施前事实核验（2026-07 探索期逐条 grep 验证）

### 一、proposal 断言全部属实（可放心删）
- **SaveNarrative/SaveBatchNarratives 零调用**：全库 grep 仅定义处（admin/repository/repository.go:286/:290）。
- **死方法全家桶均零调用**（proposal"等死方法"覆盖但未点名，实测可全删）：ListNarrativesByDate(:250)、GetNarrativeByID(:261)、DeleteNarrativeByDate(:269)、ListNarrativeBoards(:297)、GetNarrativeBoardByID(:303)、ListNarrativeScopes(:317) + 类型 BoardNarrativeRow(:243)、NarrativeScopeItem(:311)。均引用 models.NarrativeSummary/NarrativeBoard，删 model 后编译强制清理。
- **生成函数已不存在**：GenerateAndSaveForCategory/GenerateNarrativesForBoard/cleanEmptyBoards 全库零命中。
- **死路由**：board_crud_handler.go:136 `boards.GET("/:id/narratives", handler.getBoardNarratives)` + handler :342；前端全库零调用 `/narratives` 端点。
- **组件误名属实**：NarrativeGenerateDialog.vue 实际调 `useDailyReportsApi().generateDailyReport`（import ~/api/dailyReports）——名为叙事实为日报。
- **migrator.go AutoMigrate 注册**在 :98-99（&models.NarrativeSummary{} / &models.NarrativeBoard{}）。
- **dump-sanitizer/tables.go** narrative_boards(:62)、narrative_summaries(:161)。
- **20260522_0001 范式**存在于 postgres_migrations.go:361，IsDestructiveAllowed 守卫多处可照抄。
- **flow 计数精确**：daily-report.md「叙事」×22、scheduler.md ×4。
- **主 specs 5 个待删目录**全部存在于 openspec/specs/。

### 二、发现的规格 bug / 误伤风险（实现前需修 tasks）
1. **7.8 验证标准写错（重要）**：`grep -rn 'narrative' | grep -v _test | grep -vi evolution | grep -v postgres_migrations.go` 期望零命中，但 topicgraph daily_report_* 活轨道有 ~33 处合法 narrative **注释**（narrative thread / durable narrative frame / new-narrative lane L3 / narrative chain，分布：daily_report_models.go×8、daily_report_lane.go×9、daily_report_topic_repository.go×4、orchestrator/matching/merge/cluster/llm 等）。这些是活概念必须保留。**7.8 需追加 `| grep -v topicgraph` 或改为按删除面白名单式检查**，否则验证永红或诱导改活轨道。
2. **tasks 1.1 漏列测试编译炸点**：internal/models/semantic_label_test.go:45-47 `TestNarrativeBoardHasSemanticBoardLink` 用 `reflect.TypeOf(NarrativeBoard{})`——删 narrative_board.go 后该测试编译失败，需同步删除此测试函数。
3. **ER_DIAGRAM.md 删除面被低估**：不止 :65/:67/:85，还有 :323-350「### Narrative（叙事摘要面）」整节、:588-589 关联表、:650-654 字段引用、:684 历史注记。3.2 的 grep 零命中的验收能兜住，行号枚举仅参考。
4. **DATABASE_FIELDS.md 确认含 16 处** narrative 两表命中 → task 5.1 必须执行（非"无则记录"分支）。

### 三、前端改名影响面（task 2.1/2.2 清单）
NarrativeGenerateDialog 引用点共 4 处：
- app/features/tags/components/TagsPage.vue:11（显式 import）+:300（模板）——注意是显式 import 非裸名自动导入，typecheck 可抓；D4 双拼写 grep 验证仍必要
- app/composables/useDailyReportProgress.ts:4（注释提及）
- app/features/tags/components/topic-landscape/TopicLandscapePanel.vue:332（注释提及）
BoardListSidebar.vue 实际路径 features/tags/components/，按钮 :133「整理叙事」，emit `open-generate` 不动。
「叙事工坊」保留拍板涉及：AppSidebarView.vue:173、TagsPage.vue:109、useOnboarding.ts×6、useOnboarding.test.ts:137/:173（测试断言文案，改名才会炸，保留则不动）。

### 四、白名单（零改动验证清单，task 4.4）
boardEnrichment.ts（:217 因果叙事形态注释、:346 evolution_narrative）、BoardEnrichmentPanel.vue、QAPanel.vue、CausalAnalysisReport.vue(+test.ts)、AnalyzeRefChip、DebateSection.vue、markdown.ts:6、dataenrichment/repository/models.go:71 注释、topicgraph 全部 daily_report_*（前述 ~33 处注释）。

### 五、其他注意
- db_unit_test.go:58/:85-86 引用历史迁移 SQL（narrative_boards 建索引/删列语句）——D3 保留历史账本则重放仍"先建后由 20260824_0001 DROP"，理论上不炸，实现时跑 go test 确认。
- semantic-board.md「板块」实测 grep -c 18 行（proposal 写 ×29，或按出现次数计），执行以实时 grep 为准；前端 UI 里另有大量「板块」（UpgradeSuggestionPanel×18 行、MatchingConfigDialog×8 行等）属后续词表 change 范围，本 change 不动。
- admin/repository 死方法区从 :239（"// Narrative operations"注释）到 ~:345 连片，可整段删。

<!-- pinned 2026-08-24T01:08:59Z -->
