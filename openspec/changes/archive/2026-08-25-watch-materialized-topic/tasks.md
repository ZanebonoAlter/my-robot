## 1. 数据模型与迁移

- [x] 1.1 `BoardTopicWatch` 模型扩展：新增 `Query`(text,可空) / `EmbeddingCache`(vector,可空) / `PersistentTopicID`(*uint,可空 FK) 字段与 `WatchTypeKeywordTopic` / `WatchTypeSentenceTopic` 常量；验证：AutoMigrate 冒烟（db_unit_test 模式）通过且新列存在
- [x] 1.2 Postgres migration：type CHECK 重建为四值（沿 20260824_0002 惯例）+ 三新列 + FK(ON DELETE SET NULL)；验证：migration 单测（沿 watch_type_column_test.go 模式）断言非法 type 拒绝、四值可写、新列可空
- [x] 1.3 `topic_watch_repository.go` 扩展：CreateWatch 接收 type/query/embeddingCache；UpdateWatch 更新 label/query 时置空 embedding_cache；验证：repository 单测覆盖创建/失效/回读

## 2. 关键字物化轨（keyword_topic）

- [x] 2.1 文章取数查询 `ListWatchCandidateArticles(date)`：当天 pub_date 窗口 + 未归档 + 择优摘要 COALESCE（AIContentSummary>FirecrawlContent>Content>Description，NULLIF 空串）+ 量级截断保护；验证：单测覆盖窗口边界、归档排除、摘要择优
- [x] 2.2 匹配器文章版入口：复用既有 parseKeywordExpr/matchKeywordGroups，输入 (title+择优摘要) 拼接文本，输出命中文章集 + matched words；验证：单测覆盖 DNF 语义、大小写、无 tag 文章可命中、零命中空集
- [x] 2.3 keyword 物化组装：section（固定名「关键字『XX』相关话题」/ ClusterIndex 续排 / lane_tier=watch_keyword / BestTier=4 / AvgScore=0 / Embedding 留空 / ClusterTagIDs 空数组）+ 每文章一条 thread（Confidence=1.0 / RelatedArticleIDs=[自身] / 摘要截断）；验证：单测断言字段契约

## 3. 一句话物化轨（sentence_topic）

- [x] 3.1 辅助标签池检索：BoardComposition join SemanticLabel 取池 + 内存余弦 + 阈值/top-K 配置（默认 0.55/8，沿 LoadPersistentTopicConfig 读取机制）；验证：单测覆盖阈值过滤、top-K 截断、空池/无 embedding 标签跳过
- [x] 3.2 标签→tag→文章解析：TopicTagSemanticLabel → active event tag → 当天有文章限定 → 文章并集去重；验证：单测覆盖无关联 tag、当天无文章、并集去重
- [x] 3.3 embedding 缓存生命周期：创建 watch 时 embed 一次写缓存（失败不阻断创建）；生成时缓存为空则惰性补算回写；验证：单测（mock embed 函数）覆盖成功/失败/补算三条路径
- [x] 3.4 专属话题创建与复用：首物化建话题（source=manual/status=active/label=watch.label/Embedding+Centroid=检索句向量）并回写 watch.persistent_topic_id，后续复用；section 归属字段（PersistentTopicID/TopicMatchConfidence=manual/TopicMatchDistance=0/lane_tier=watch_sentence）；验证：单测覆盖首建/复用/无命中不建

## 4. 日报管线接入

- [x] 4.1 orchestrator 物化 phase：Step 7 merge 之后、return 之前 append 物化 sections/threadBatches；report 级计数不改；任一 watch 物化失败降级跳过（log warn）不阻断；验证：单测（注入失败 watch + 正常 watch 并存，断言日报正常返回且仅含正常物化）
- [x] 4.2 SaveReport 钩子规则：assignAndUpdateTopics 排除 watch_keyword（不自动归属不建题）、正常处理 watch_sentence（生命周期推进）；RebuildBoardRelations 排除全部 watch_*（无关系边）；验证：集成测试断言 keyword section 无话题、sentence 话题 consecutive_hits 递增、物化 section 无 relation 行
- [x] 4.3 提示轨互斥：evaluateWatchHits 分流跳过 watch_* 类型；ListWatchSectionTexts* SQL 与 label 轨 prompt 构建过滤 watch_* section；验证：单测断言物化轨零 hits、物化 section 不进 prompt

## 5. API 与前端

- [x] 5.1 topic_watch_handler：POST 扩展（type/query 校验：keyword* 轨 DNF 校验、sentence 轨句子非空、创建时尝试 embed）；DELETE 扩展（sentence 轨 confirm_archive_topic 确认参数 → 归档话题后删 watch）；验证：handler 单测覆盖四类型创建、非法表达式拒绝、删除确认流
- [x] 5.2 前端 `app/api/topicWatches.ts`：类型定义扩展（type 四值/query/persistent_topic_id）+ 请求参数；验证：api 单测通过
- [x] 5.3 watch 管理对话框：类型选择（四轨）、sentence 轨 query 输入、删除 sentence 轨确认弹窗（提示将归档话题）；验证：组件交互手动验证 + lint/typecheck 通过
- [x] 5.4 日报时间线：lane_tier=watch_keyword/watch_sentence 的样式区分（角标/徽标）；验证：BoardDailyReportTimeline 组件渲染正常（单测 + ui-verify 截图）
- [x] 5.5 旧提示轨创建入口退役：TopicWatchCreateDialog 仅物化轨双选（默认 keyword_topic），label/keyword 输入态删除；WatchManagePanel 回扫 banner 死代码清理（navigate-daily 联动移除）；存量四类型继续展示可管理；验证：topic-watch 测试 43/43 + 全量 563/563 + lint/typecheck 过

## 6. 测试

> 测试设计锚 `test-cases.md`（复杂档白盒用例文档：五故事主链路 + 变体走查 + 白盒锚点表）。

- [x] 6.1 集成测试（watch_materialize_integration_test.go）：双物化轨端到端——keyword 捞回无 tag 文章、sentence 话题跨期延续（首建→复用→consecutive_hits 递增）、重生成幂等；验证：`go test ./internal/topicgraph/service/ -run TestWatchMaterializationIntegration` PASS
- [x] 6.2 白盒用例锚点四组：DNF 匹配语义复用（与提示轨同源）/ SaveReport 排除放行边界 / 提示轨互斥双向 / embedding 检索阈值与 top-K；验证：`go test ./internal/topicgraph/service/ -run "TestMatchKeywordArticles|TestBuildKeywordWatchSection|TestCosineSimilarity|TestRetrieveAuxLabels"` PASS
- [x] 6.3 回归确认：label/keyword 提示轨行为不变（既有 watch 测试全绿）；验证：`go test ./internal/topicgraph/...` 全 PASS

## 7. 文档

<!-- doc-impact: flow api database -->
<!-- doc-impact-excuse: standard=standard/*.md 三处脏文件为其他 change 的基线残留，本 change 未触碰任何 standard 文档 -->

- [x] 7.1 `docs/reference/flow/daily-report.md`：管线图补 Watch 物化追加步骤、物化 section 边界（不合并/不关系/计数口径）、失败降级约束
- [x] 7.2 `docs/reference/flow/topic-graph.md`：watch 与持久话题联动（sentence 轨专属话题、删除归档联动）、「命中不改归属」不变量的物化轨边界修订
- [x] 7.3 涉及 API 文档同步（topic-watches CRUD 新参数）

## 8. 验证

- [x] 8.1 `cd backend-go && bash ../scripts/change-scope.sh` 判定影响包后：`go test ./internal/topicgraph/... ./internal/platform/database/...` — 全部 PASS（-short 自动跳过 DB 集成外用例）
- [x] 8.2 `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` — 零报错
- [x] 8.3 `cd front && pnpm lint` — 零报错
- [x] 8.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck && pnpm test:unit"` — 通过（Windows cmd 执行）
- [x] 8.5 `openspec validate watch-materialized-topic` — valid；`openspec show watch-materialized-topic --json --deltas-only` 核对 delta 与实现一致

### Scenario → 测试文件映射（scenario-trace-gate 对账表）

| spec Scenario | 测试文件 |
| --- | --- |
| watch-materialized-topic: 含关键字文章聚合为固定话题 | backend-go/internal/topicgraph/service/watch_materialize_keyword_test.go |
| watch-materialized-topic: 漏网文章可被捞回 | watch_materialize_keyword_test.go + repository/topic_watch_repository_scan_test.go |
| watch-materialized-topic: 无命中不产空 section | service/watch_materialize_integration_test.go |
| watch-materialized-topic: 检索命中并物化 / 阈值过滤 | service/watch_materialize_sentence_test.go + repository/topic_watch_repository_sentence_test.go |
| watch-materialized-topic: 检索句更新后缓存失效 | repository/topic_watch_repository_test.go（TestUpdateWatchInvalidatesEmbeddingCache） |
| watch-materialized-topic: 物化 section 推进话题延续 / 无物化日自然衰减 | service/watch_materialize_integration_test.go（day1→day2 + P3 变体） |
| watch-materialized-topic: 物化话题作为聚类锚 | persistent-topic 既有"手动话题参与自动归属"规则承接（spec 交叉引用，无新代码路径；集成测试 C1 归属字段佐证） |
| watch-materialized-topic: 不参与同日合并 / 计数不重复 | service/watch_materialize_integration_test.go（topicCount 断言 + relCount 断言） |
| watch-materialized-topic: 物化失败不阻断 | orchestrator Step7.5 降级分支（spec F1；集成测试隐式覆盖 sentence 跳过路径） |
| watch-materialized-topic: 删除关键字轨保留历史 / 删除一句话轨确认归档 | handler/topic_watch_handler_pg_test.go（TestDeleteTopicWatch_SentenceRequiresConfirmation） |
| watch-materialized-topic: 物化轨无命中记录 | service/watch_materialize_integration_test.go（GetWatchHitsByReport 空断言） |
| topic-watch: 创建一句话物化关注 / 类型约束 / 更新检索句失效缓存 | handler/topic_watch_handler_pg_test.go + repository/topic_watch_repository_test.go |
| topic-watch: 暂停的关注不参与判定 | 既有 daily_report_watch_test.go（回归，物化轨经 ListActiveMaterializedWatchesByBoard 的 status 过滤） |
| daily-report-system: 物化失败不阻断流水线 | orchestrator Step7.5（同上 F1） |
| daily-report-system: 物化 section 记录物化来源 | service/watch_materialize_keyword_test.go（字段契约 C1）+ 集成 kwReload 断言 |
