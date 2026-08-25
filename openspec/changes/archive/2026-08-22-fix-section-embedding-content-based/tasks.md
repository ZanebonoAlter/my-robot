# Tasks: fix-section-embedding-content-based

## 1. 内容化文本组装（纯函数 + 单测）

- [x] 1.1 `internal/topicgraph/service/` 新增 `buildSectionEmbedText(clusterTags []repository.TagInput, threads []repository.DailyReportThread, fallbackLabel string) string` 纯函数：per tag 拼接 label +「：」+ description +「；代表文章：」+ 截断(ArticleContext, 100 runes)，换行连接，总长截断 480 runes；兜底链 无 tags→thread 标题拼接→cluster_label
- [x] 1.2 单测覆盖：正常多 tag 拼接、空 description/ArticleContext 省略、单 tag 100 runes 截断、总长 480 runes 截断、空 tags 走 thread 标题、全空走 cluster_label、纯空格输入
- [x] 1.3 跑 `go test ./internal/topicgraph/service/ -run TestBuildSectionEmbedText` → PASS

## 2. 流水线接入（orchestrator 替换 embedding 输入）

- [x] 2.1 `daily_report_orchestrator.go` Step 6：embedding 输入从 `sec.ClusterLabel` 改为 `buildSectionEmbedText(clusterTags, threads, clusterLabel)`（clusterTags 由 `filterTagsByIDs(tags, cluster.TagIDs)` 得到；无 tags 的 section 沿用兜底链）
- [x] 2.2 确认 embedding 生成时机仍在 threads 收集后、`computeThreadFitDistances` / `MergeSimilarSections` 前；空文本 section 跳过嵌入（保持现状）
- [x] 2.3 更新 orchestrator 现有相关单测（若有断言 embedding 输入为 cluster_label 的用例，改为断言内容文本）——排查结果：无断言旧口径的用例，无需改动

## 3. 回刷端点扩展

- [x] 3.1 `BackfillSectionEmbeddings` 扩展签名：新增 `recompute bool / boardID *uint / sinceDays int` 参数；补缺模式文本改用内容化规则（从 DB 按 cluster_tag_ids 反查 tag label/description，ArticleContext 复用 `buildArticleContextForTag` 口径按 section 日期窗口取），重算模式处理范围内全部 section
- [x] 3.2 两种模式完成后：受影响 topic 集合重算 centroid（`ComputeTopicCentroid`）→ `BackfillRelations`；单条嵌入失败跳过 + 计数，日志输出 embedded/skipped 统计
- [x] 3.3 `triggerBackfillEmbeddings` handler 解析 `recompute` / `board_id` / `since_days` query 参数并透传；保持异步 goroutine + 立即返回 processing
- [x] 3.4 repository 层单测：范围过滤（board/date 窗口）正确、重算模式覆盖已有 embedding 的 section、失败跳过不影响其他 section（skip-and-continue 契约用 ai route not found 场景验证）

## 4. 观测与前端语义注释

- [x] 4.1 `front/app/api/dailyReports.ts` 中 `DailyReportThread.fit_distance` 注释从「Thread 标题 ↔ 所属 section 标题」更新为「Thread 标题 ↔ 所属 section 内容 embedding」（仅注释，无逻辑改动）

## 5. 测试

- [x] 5.1 `go test ./internal/topicgraph/service/... ./internal/topicgraph/repository/...` → PASS（影响包）
- [x] 5.2 `golangci-lint run ./... && go vet ./... && go build ./...` → 零失败

## 文档

<!-- doc-impact: flow api -->
<!-- doc-impact-excuse: database=工作区其他 active change 脏文件命中（本 change 未实现，0/20 任务），非本 change 改动; architecture=同上，其他 change 脏文件; configuration=同上 -->

- [x] `docs/reference/flow/` 日报域活文档：补充 section embedding 内容化规则与质心漂移不变量（打破标题回声闭环），§12.2 归档后补变更溯源链接
- [x] `docs/reference/api/`：`POST /daily-reports/backfill-embeddings` 新增 `recompute` / `board_id` / `since_days` 参数说明

## 验证

- [x] `cd backend-go && go test ./internal/topicgraph/service/... ./internal/topicgraph/repository/...` → 全部 PASS
- [x] `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` → 零输出零失败
- [x] `grep -rn "buildSectionEmbedText" backend-go/internal/topicgraph/service/daily_report_orchestrator.go` → 至少 1 处调用（流水线已接入，实测 2 处）
- [x] 内容化 embedding 生效验证（回刷实测替代次日观察）：board1974 抽样 8 个 section md5 全部换血；section 3159 到自身内容 tag 距离 0.0058（旧标题向量时代为 0.24+，回声 ≈0.00002 消失）
- [x] 部署后回刷实测：全量 12 板块 30 天 1071 sections 全部重嵌入（当日 53 次 embed 调用仅 1 次失败为修复前 512-token 案），350 个话题质心刷新，关系重建完成
