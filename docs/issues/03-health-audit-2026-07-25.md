# 03 · 全仓健康体检报告（2026-07-25）

> 范围：文档完整性 / 后端代码 / 数据库 DDL / 前端代码 四维审计。
> 证据格式：`file:line`。所有结论均来自实读代码，不做无依据猜测。
> 配套目录：`docs/issues/02-quality-audit-2026-07-23/`（前次质量审计）。
> 优先级：**P0**（必修，正确性/数据风险）／**P1**（强烈建议）／**P2**（性价比优化）。

---

## 0. 总览（先看这张表）

| 维度 | 红线数 | 最值得先动的 1 件事 |
|------|-------|---------------------|
| 📚 文档 | 17 处过时 + 8 处缺失 | 补 `DATABASE_FIELDS.md` 4 张新表 + 删 `user_preferences` |
| 🗄️ 数据库 DDL | 4 P0 + 8 P1 | 给 `preference_vectors.embedding` / `route_embeddings.embedding` 加 HNSW 索引 |
| 🦫 后端代码 | 20 个上帝文件 + 8+ 处 fire-and-forget goroutine | `recommendation_service.go` Accept/SyncAll 包事务 + 不吞 err |
| 🎨 前端代码 | 15 个上帝组件（2 个 >1500 行） | 拆 `BoardThreadBrowser.vue`（2458 行）+ 抽 AI ctx 类型 |

**当前进行中 change（`preference-vector-feed-discovery`）影响放大**：代码已合并，但 APPLY_TODO §D1-D6 文档节全部未完成，导致文档大面积漂移；该 change 尚未 archive，按执行规范 §11 归档门禁，**archive 前必须补齐本文 P0 文档项**，否则 `doc-impact.sh verify` 必失败。

---

## 1. 文档完整性

### P0 · 代码已变文档未跟（17 处，会误导）

| # | 文档 | 问题 | 实际代码证据 |
|---|------|------|--------------|
| 1 | `docs/reference/database/DATABASE_FIELDS.md:3,19,21` | 表数自相矛盾（43/44），实际 47 张；漏 4 张新表（`preference_vectors` / `rsshub_routes` / `route_embeddings` / `feed_recommendations`） | `backend-go/internal/models/discovery.go:29-92` |
| 2 | 同上 §11.2 + line 67 | `user_preferences` 已由迁移 `20260725_0001` DROP，仍完整列字段 | `postgres_migrations.go:1428-1439` |
| 3 | `docs/reference/api/reading.md:51-71` | `/api/user-preferences` 端点已删，仍写 GET/POST | `admin/routes.go` 已无该组；`preferences_handler.go:15` 注释「已废弃」 |
| 4 | `docs/reference/api/reading.md` / `_index.md:18` | discovery 全套 7 个端点零文档（`/api/discovery/catalog/{sync,status}` / `/recommendations` / refresh/accept/dismiss / `/ask`） | `admin/routes.go:47-56` |
| 5 | `docs/reference/flow/scheduler.md:18,25,99` | 调度器数错（写 13 实际 15）；仍列已删 `preference_update`；代码入口仍写 `job_preference_update.go` | `app/runtime.go:92-247`（15 个）；新文件 `job_preference_profile_update.go` |
| 6 | `docs/reference/architecture/runtime.md:56,87,117,132,151` | 9 处引用已删 `preference_update` / `/api/user-preferences`；调度器数写 9 实际 15 | 同上 |
| 7 | `docs/reference/architecture/backend.md:229,440,92` | 调度器清单过时；列已删路由；目录注释未反映 discovery 子域 | 同上 |
| 8 | `docs/reference/flow/reading.md:90-91,101` | 偏好段描述旧实现（`UpdateAllPreferences` + `DELETE FROM user_preferences`），代码入口列已删文件 | `preference_profile_service.go` / `job_preference_profile_update.go` |

### P1 · 覆盖缺口（8 处缺失文档）

- **flow 文档**：`discovery flow` / `recommendation-preference-profile flow` / `rsshub-route-catalog flow` 全无（APPLY_TODO §D1-D6 明列未做）
- **api 文档**：`api/preference-profile.md` / `api/discovery.md` 缺失；`api/_index.md` 表未登记
- **架构索引**：`architecture/map.md` 业务域→代码入口表缺 discovery / preference-profile / rsshub-catalog 三域（违反执行规范 §0.6 map 同步要求）
- **配置**：`configuration.md` 缺 4 项新配置（`DefaultRSSHubBaseURL` / `DismissCooldownDaysDefault` / `RecommendationTopNDefault` / preference 权重阈值）

### P2 · 冗余 / 可归档

- `development.md` / `testing.md` 开头自承「规范已迁入 standard/」，仍残留失效引用（`tests/workflow/` / `tests/firecrawl/` 已不存在）
- 根 `AGENTS.md:3` 项目路径仍写 `D:\project\my-robot`（旧名）；`:19,59` 引用失效 Python `uv` + tests 目录
- `README.md` 端口描述前后矛盾（line 292 写「启动后访问 :5000」与 line 325 前端 :3000 冲突）；`configuration.md:17` `CORS_ORIGINS` 默认值重复同值
- `docs/issues/02-quality-audit-2026-07-23` 是目录而非 `.md`，与 `01-*.md` 命名不一致（小瑕疵）

---

## 2. 数据库 DDL

### P0 · 数据完整性 / 正确性风险

1. **向量列缺 HNSW 索引 → KNN 全表扫**
   `recommendation_service.go:180,191` 用 `<=>` 粗筛，但 `preference_vectors.embedding` / `route_embeddings.embedding` **无任何 vector 索引**。对比 `topic_tag_embeddings` 等已建 HNSW（`embedding.go:482`）。
   → 每次推荐 exact KNN，数据量上来直接拖垮。
   **建议**：`CREATE INDEX ... USING hnsw (embedding vector_cosine_ops)`（注意 pgvector HNSW >2000 维不支持，需保留 Go-side cosine fallback 通道）。

2. **ReadingBehavior 无 FK 级联 → 删文章产生孤儿**
   `reading_behavior.go:17-18` 仅 `foreignKey:ArticleID`，无 `constraint:OnDelete:CASCADE`；对比 `ArticleTopicTag`（`topic_graph.go:166-167`）已正确声明。
   → 删文章后行为记录残留，偏好向量重算读到脏数据。
   **建议**：补 `constraint:OnDelete:CASCADE` + 迁移建 FK。

3. **核心表缺 UpdatedAt → 状态变更无审计**
   `article.go:7-35`（25 列宽表）只有 `CreatedAt`，但 `SummaryStatus`/`FirecrawlStatus`/`CompletionAttempts` 等被反复更新。同病：`feed.go:17` / `category.go:16` / `narrative_board.go:17`。
   → 并发更新无新鲜度判断，增量同步失效。
   **建议**：统一加 `UpdatedAt time.Time`。

4. **`topic_tags.Kind` 标 deprecated 仍被填默认值**
   `topic_graph.go:67-69` 注释「will be removed with a DB migration」，但 `postgres_migrations.go:1320` 仍 `SET DEFAULT 'keyword'`，无 DROP COLUMN 计划。
   → dead column 残留 + 新行仍填。
   **建议**：补 DROP COLUMN 迁移 + 删字段。

### P1 · 性能 / 可维护性

5. **8+ 个 status 列无 CHECK 约束** — `topic_tags.status` / `articles.summary_status` / `firecrawl_status` / `feeds.refresh_status` / `semantic_labels.status` / `firecrawl_jobs.status` / `tag_jobs.status` / `embedding_queues.status` / `narrative_summaries.status` 全是裸 `varchar`，任意字符串可入库。历史已踩过坑（`postgres_migrations.go:1404` 描述提到空串致 board match 失效）。**建议**：每列加 `CHECK (status IN (...))`。

6. **ID 列表塞 text 列无法 JOIN/索引** — `narrative.go:14-15`（`ParentIDs` / `RelatedTagIDs` / `RelatedArticleIDs` `gorm:"type:text"`）、`narrative_board.go:14-15`（`EventTagIDs` / `PrevBoardIDs`）。**建议**：改 JSONB 或拆 junction 表。

7. **`articles` 25 列宽表混三领域** — RSS 元数据 + summary pipeline + firecrawl pipeline 混在一起（`article.go:7-35`）。**建议**：拆 `article_completion_jobs` 1:1 子表存流水线状态，主表回归 RSS 元数据。

8. **`TopicTag.Aliases` text 存 JSON 数组** — `topic_graph.go:53`，同文件 `Metadata` 已用 `jsonb;serializer:json`，应统一。

9. **`AICallLog` 无分区/保留策略** — `ai_models.go:122-141` 每次模型调用写一行，text 字段可达 KB，单用户长期运行无限增长。**建议**：按月分区或挂保留期清理 task（已有 SchedulerTask 框架）。

10. **`ai_providers.api_key` 明文存 text** — `ai_models.go:71`。单用户自托管风险有限，但备份/日志泄露即丢密。**建议**：应用层加密或引用 secret store。

11. **向量字段用 `string gorm:"type:vector"`** — `topic_graph.go:123` / `discovery.go:18,59` / `semantic_label.go:12`。Go 侧无类型保护，写入需手动格式化 `"[1,2,3]"`。**建议**：引入 `pgvector` Go driver 类型或自定义 Valuer/Scanner。

### P2 · 可选优化

12. **bool 命名不一致** — `Read`/`Favorite`（无前缀）vs `IsCanonical`/`IsWatched`/`IsSystem`/`IsFallback`（有前缀）vs `EnrichmentEnabled`/`Protected`（描述型）。
13. **低基数 bool 索引选择率差** — `idx_articles_read` / `idx_articles_favorite`（`postgres_migrations.go:185-186`）几乎全 false，改 partial `WHERE read=true` 更优。
14. **时间戳硬编码 CST** — `utils.go:7` 写死东八区，DB 列大概率 `timestamp without time zone`。单用户可接受，多租户/跨时区踩坑。建议列改 `timestamptz`。
15. **`topic_tag_embeddings` 维度硬编码 4096** — `postgres_migrations.go:119,126` `vector(4096)`，但 runtime 从 `embedding_config.embedding_dimension` 读（默认 1024，`:146`）。DDL 与配置不一致，换模型必须 ALTER。其他表已用无维度 `vector`，建议对齐。

### 宽表 / JSON 滥用汇总

| 表 | 问题 | 建议 |
|----|------|------|
| `articles` | 25 列混 RSS+summary+firecrawl | 拆 1:1 子表 |
| `NarrativeSummary` / `NarrativeBoard` | 5 个 text 列塞 ID 列表 | 改 JSONB 或拆 junction |
| `TopicTag.Aliases` | text 存 JSON 数组 | 改 JSONB + serializer |
| `SchedulerTask` | 18 列含 6 统计 + 4 时间 | 轻度，可拆 `scheduler_task_stats` |
| `AICallLog` | 16 列大 text 日志宽表 | 靠分区/保留治理 |

### 索引缺失/冗余

- **缺**：2 个 vector 列（P0）；`reading_behaviors` 缺 `(article_id,event_type)` / `(feed_id,created_at)` 复合；`firecrawl_jobs`/`tag_jobs` lease 扫描缺 `(status,available_at,priority)` 复合。
- **冗余/低价值**：`idx_articles_read`/`idx_articles_favorite`（改 partial）。

### 迁移治理现状（OK，有长期债）

- ✅ Up 幂等（普遍 `IF NOT EXISTS`）、destructive 有守卫（`migrator.go:41-47,113-119`）、outside-tx 路径存在（`migrator.go:148-167`）。
- ⚠️ **Down 仅有声明无执行器**（`migrator.go:29-34` 注释明说），不可逆迁移只能靠 Description 标注 — 长期债。

---

## 3. 后端代码质量

### 3.1 admin/（含进行中 change 新代码）

| 级别 | 位置 | 问题 |
|------|------|------|
| **P0** | `service/recommendation_service.go:394-406` | `AcceptRecommendation` 跨表写 `feeds` INSERT + `feed_recommendations.markAccepted` UPDATE **不在事务里**；对比同文件 `preference_profile_service.go:89` 已正确用事务 — 不一致 |
| **P0** | `service/recommendation_service.go:454,458,466,467` | `Ask()` 用户问答主路径连续吞 4 个写/查 err（`blocked, _ := countDismissedInCooldown` / `_, _ = insertPending` / `boardVecs, _ := loadBoardVectors` / `_ = prefSvc.WriteSeed`），DB 故障时静默返空无任何 log |
| **P0** | `service/catalog_sync_service.go:58-135` | `SyncAll` 逐条 err 已处理，但**整批无外层事务**：fetch→diff→N×Save/Create/Updates→mark gone 串联，中途失败 `CatalogSyncSummary` 数字与实际不符 |
| P1 | `service/recommendation_service.go:228` | `filterByFeedsURL` 内 `Pluck("url", &urls)` 忽略 `.Error`（误报为 catalog_sync:228，实为 recommendation_service） |
| P1 | `service/catalog_sync_service.go:297-302` | `GetStatus` 内 `countStatus(...)` ×5 与 `Count(&st.Embedded)` 全忽略 `.Error`，统计接口 DB 抖动静默返 0 |
| P1 | `service/catalog_sync_service.go:27` + `service/recommendation_service.go:236,532` + `discovery_helpers.go:26` | `DefaultRSSHubBaseURL = "http://47.110.71.194:1200"` 写死公网 IP + http，跨 3 文件复用；`discovery_helpers.go:26` 注释声称「可由 ai_settings 覆盖」但 **grep 全仓无任何代码读取 ai_settings 覆盖** — 注释撒谎，配置化未实现 |
| P1 | `service/preference_profile_service.go:557` | `err == gorm.ErrRecordNotFound` 裸 ==，应 `errors.Is` 防 wrap |
| P2 | `handler/discovery_handler.go:18-20` | 每请求 `newRecommendationService()` 重建 `airouter.NewRouter()`，浪费内部并发 map |
| P2 | `handler/discovery_handler.go:85` | `_ = c.ShouldBindJSON(&req)` 吞 bind err |
| P2 | `scheduler/base.go:483` | `s.cfg.Job(context.Background())` 切断调度/tracing ctx，job 无法响应 graceful shutdown |

> **核实纠正（2026-07-25 二次复核）**：subagent 首版报告称 `generateAndPersist` (line 96-115) 与 `catalog_sync SyncAll` 逐条 err 处理有问题，实测**均已正确 return err**。真正待修的吞 err 仅集中在 `Ask()` 路径 + `filterByFeedsURL` Pluck + `GetStatus` 统计。

### 3.2 tagmanagement/

| 级别 | 位置 | 问题 |
|------|------|------|
| **P0** | `handler/board_crud_handler.go` (1285 行) / `service/board/semantic_board_upgrade.go` (1196 行) | 上帝文件，混 CRUD + LLM + 事务 + goroutine |
| **P0** | `service/board/semantic_board_backfill.go:112` / `core/tag_queue.go:225,250` / `core/extractor_enhanced.go:46,50` | fire-and-forget `go func(){ job(context.Background()) }` 无 recover，崩了进程退 |
| P1 | `service/merge/tag_merge_suggest.go:32-37,54` | 包级 `scanState`（cancel+chan）多 handler 并发触发时仅靠 `running atomic.Bool` 守门，cancel 替换无锁 |
| P1 | `handler/board_crud_handler.go:218,738` | handler 内联 `db.WithContext().Transaction(...)` + `go func` 分片，分层穿透 |
| P2 | `service/watched/` | 零测试（其它子包 core/embedding/merge/auxlabel 都有） |

### 3.3 reader/

| 级别 | 位置 | 问题 |
|------|------|------|
| **P0** | `handler/feed_handler.go:208,253,365,419` / `handler/preferences_handler.go:41` | handler 直接 `repository.Repo.DB().Create/First/Updates/Delete` 绕过 repo，分层形同虚设（repo 已提供 `GetFeed`/`CreateFeed` 却不用） |
| **P0** | `handler/feed_handler.go:466,546,567` / `handler/opml.go:155` | fire-and-forget goroutine 做 refresh/cleanup，无 ctx 传播无 recover，复用 gin `c` 不安全 |
| P1 | `handler/feed_handler.go` (595) / `article_handler.go` (569) / `service/content_completion_service.go` (464) | 上帝文件 |
| P1 | `handler/feed_handler.go:109` | `if perPage < 10000` 当「不分页」开关，魔数；同处 `if resultPerPage == 0` 除零保护说明已知会炸 |
| P2 | `handler/opml.go` | 无测试 |

### 3.4 topicgraph/ & dataenrichment/

- **P0 上帝文件**：`repository/daily_report_repository.go` (1016) / `handler/daily_report_handler.go` (747) / `service/daily_report_orchestrator.go` (571) / `repository/daily_report_topic_repository.go` (619) / `dataenrichment/service/orchestrator.go` (1243) / `handler/handler.go` (937)
- **P0 panic 在路由注册期**：`dataenrichment/handler/handler.go:101` `panic("InitHandler must be called before RegisterRoutes")` — init 顺序耦合隐蔽
- P1 `dataenrichment/service/orchestrator.go:143` 残留 `// TODO(阶段2b): 视角选择交互`；`debate_service.go:125` `_ = s.repo.CreateStockDebateResult(ctx, dr)` 吞持久化 err
- ✅ **测试覆盖最齐的模块**：daily_report_cluster/dedup/llm/merge/orchestrator/watch/thread_fit 全有 `_test.go`，可作其它模块参考

### 3.5 platform/

- P1 `config/config.go:96` `fmt.Println("Config file not found, using defaults")` 调试残留，应走 logging
- P1 `database/postgres_migrations.go` (1679 行) 单文件迁移上帝
- P2 `ws/hub.go:136` 包级单例 hub，多实例测试受限

### 3.6 跨模块共性问题

- **错误处理**：26 处 `_ =` 裸吞 err；response envelope 不统一（discovery 用 `{success,data,error,message}`，scheduler handler 直接吐 `map[string]interface{}` 含中文 message）；service 层几乎不检 `ctx.Err()`（仅 `catalog_extras.go:50`）
- **测试盲区**：新代码的 `AcceptRecommendation`/`DismissRecommendation` 状态机、`SyncAll` 事务回滚、`Ask` 错误降级路径**无测试**；`watched/`、`opml.go`、`firecrawl_config.go`、`discovery_handler.go`、`preference_profile_handler.go` 全无测试
- **接口设计**：RESTful 不一致（`/feeds/:feed_id` vs `/routes/:capability` vs `/recommendations/:id/accept` 动作型 URL）；分页不统一（reader 用 `page/per_page`，discovery 全量 `Find` 无分页）
- **可观测性**：新代码 `Ask`/`Accept`/`Dismiss` 无 log；全仓无 metric 埋点（推荐命中率/dismiss 率/catalog sync 耗时不可观测）；tracing 仅 `scheduler/base.go:455` 一处 otel

### 上帝文件清单（>500 行非测试，共 20 个）

```
tagmanagement/handler/board_crud_handler.go            1285
dataenrichment/service/orchestrator.go                1243
tagmanagement/service/board/semantic_board_upgrade.go 1196
topicgraph/repository/daily_report_repository.go      1016
dataenrichment/handler/handler.go                      937
tagmanagement/service/auxlabel/auxiliary_label_service 799
topicgraph/handler/daily_report_handler.go             747
admin/service/recommendation_service.go  (新)          612
topicgraph/repository/daily_report_topic_repository.go 619
tagmanagement/service/core/extractor_enhanced.go       617
tagmanagement/service/core/embedding.go                606
admin/handler/scheduler_handler.go                     606
reader/handler/feed_handler.go                         595
admin/service/preference_profile_service.go (新)       558
tagmanagement/service/board/semantic_board_matching.go 573
topicgraph/service/daily_report_orchestrator.go        571
reader/handler/article_handler.go                      569
admin/scheduler/base.go                                525
tagmanagement/handler/board_match_handler.go           516
topicgraph/repository/daily_report_matching.go         503
```

---

## 4. 前端代码质量

### 4.1 上帝组件（>500 行，共 15 个；2 个 P0 >1500 行）

| 级别 | 组件 | 行数 | 问题 |
|------|------|------|------|
| **P0** | `features/tags/components/BoardThreadBrowser.vue` | **2458** | timeline/lanes/focus/compose 4 视图模式 + 编排态 + popup thread + 持久话题管理全塞一文件，超 500 行红线 5 倍 |
| **P0** | `features/tags/components/TopicDetectiveWall.client.vue` | **1536** | Three.js 场景已抽 `detective-wall/` 子模块但 .vue 仍 1500+ 行 |
| P1 | `UpgradeSuggestionPanel.vue` | 1160 | — |
| P1 | `ComposePanel.vue` | 1135 | — |
| P1 | `CausalAnalysisReport.vue` | 922 | — |
| P1 | `daily-report/DailyReportTopicSection.vue` | 874 | — |
| P1 | `SectionLifecyclePanel.vue` | 786 | — |
| P1 | `BoardEnrichmentPanel.vue` | 674 | — |
| P1 | `dialog/TopicManageDialog.vue` | 670 | 聚合分类/合并/候选/归档 4 套 UI，应按 tab 拆 |
| P1 | `daily-report/DailyReportWatchBar.vue` | 654 | — |
| P1 | `BoardDailyReportTimeline.vue` | 641 | — |
| P1 | `MatchDetailPanel.vue` | 619 | — |
| P1 | `AuxiliaryLabelPool.vue` | 610 | — |
| P1 | `shell/FeedLayoutShell.vue` | 569 | 含 4 路编排 + 动态 import，应抽 `useFeedLayout` composable |
| P1 | `QAPanel.vue` | 545 | — |

### 4.2 类型安全（any / 断言热点）

- **AI ctx 全 any**：`features/ai/components/AIRouterBackupProviders.vue:7,9,11,15,18` + `AIRouterCapabilityRoutes.vue:11` — `inject<AIRouterCtx>` 拿到的 ctx 含 6 处 `any[]`/`any`/`(p:any)=>void`，Provider/Route 表单类型全丢。应抽 `interface BackupProvider` 到 `app/types/ai.ts`
- `features/articles/components/ArticleContentView.vue:64` `selectedContentSource.value = source as any` — 该 ref 有显式联合类型，此处绕过检查
- `stores/articles.ts:34` `response.data as unknown as ArticlePayload[] | ArticlesResponse` 双重断言
- `components/dialog/AddFeedDialog.vue:20` `ref<any>(null)` — 预览响应有明确 schema 应改 `FeedPayload`
- 全仓 `as unknown as` 断言 **9 处**（stores/api.ts、articles.ts、useArticlePagination.ts 等）

### 4.3 API / Store 层

- **死代码**：`utils/api-helpers.ts:48,73` 定义 `ApiError` 类 + `unwrapResponse()`，grep 全仓 0 处消费
- **双重错误处理**：`client.ts:69-74` 返 `{success:false,error}`，`api-helpers.ts` 又抛异常 — 两套范式并存
- **store 职责重叠**：`stores/api.ts:132` 持 `feeds` 做 normalize，`stores/feeds.ts:15-17` 又只是 `computed(() => apiStore.feeds)` 透传，三层 store 互相依赖
- **重复 fetch**：`server/api/fetch-feed.post.ts`（rss2json 代理）与 `app/api/feeds.ts:16`（后端 `/feeds/fetch`）两条路径结果形状不一致
- `client.ts:160-172` `buildQueryParams` 两段重复 JSDoc
- `client.ts:53` `await response.json()` 在非 JSON 响应（204/断网）会抛，外层 catch 丢原始 status

### 4.4 配置 / 依赖

- **P1 死依赖**：`package.json` `@nuxt/ui@4.3.0` 在 dependencies，grep 全仓 `<U[A-Z]` **0 处使用** — 应移除
- P1 `nuxt.config.ts:6` `ssr: false` 全 SPA，但依赖 `@nuxt/ui`/`three`/`gsap`/`marked`，放弃首屏 SEO/perf，需确认是否有意
- P2 `nuxt.config.ts:32` Google Fonts 直链阻塞，应改 `@nuxt/fonts` 预连接+preload
- P2 `nuxt.config.ts:36` theme 闪屏脚本用 inline `innerHTML`，CSP strict 下会断

### 4.5 跨目录共性

- **Magic number**：`per_page: 10000` 出现 **17 次**跨 `stores/api.ts` / `useGlobalSettings.ts` / `useAutoRefresh.ts` / `useRefreshPolling.ts` / `SettingsSectionFeeds.vue` / `FeedLayoutShell.vue` — `utils/constants.ts` 已有常量范式但未定义 `PER_PAGE_ALL`
- **可访问性**：`BoardThreadBrowser.vue:1060-1072` SVG `<g role="button">` 缺 keydown（Enter/Space）；全仓 `<label>` 0 命中（表单普遍用 placeholder）
- **测试盲区**：`BoardThreadBrowser.vue` 本体 / `FeedLayoutShell.vue` / `app.vue` / `stores/articles.ts` / `stores/feeds.ts` / `stores/preferences.ts` / `composables/useEventStream.ts`（WS 重连无测）/ `api/client.ts`（核心请求层无测）全无单测；e2e 仅 2 个 spec
- ✅ **卫生良好**：v-for 全有 key、defineProps/Emits 全 TS 泛型、composable 全 `use*` 前缀、无 console.log、无 TODO/FIXME、Pinia 无反应性丢失、fetch 单一入口

---

## 5. 优化执行路线图（最小风险 · 性价比最大化）

> 原则：**先补文档（解锁 change archive）→ 再修数据完整性 P0 → 再修代码 P0 → 上帝文件/组件拆分放最后**（风险高、收益慢）。

### 🥇 第一波：解锁 + 止血（1-2 天，低风险高收益）

| 序 | 动作 | 风险 | 收益 |
|----|------|------|------|
| 1.1 | 补 `DATABASE_FIELDS.md` 4 张新表 + 删 `user_preferences` + 表数对齐 47 | 极低 | 解锁 archive，消除最大漂移源 |
| 1.2 | 改 `api/reading.md` 删 `/api/user-preferences`；新建 `api/discovery.md` + `api/preference-profile.md` | 极低 | API 文档对齐 |
| 1.3 | 改 `flow/scheduler.md`（15 个调度器 + 删 preference_update + 改文件名）+ `architecture/runtime.md` / `backend.md` 同步 | 极低 | 架构文档对齐 |
| 1.4 | 改根 `AGENTS.md:3` 路径 `my-robot` → `Syntopica`；清理失效 Python/tests 引用 | 极低 | AGENTS 一致 |
| 1.5 | **DDL**：给 `preference_vectors.embedding` / `route_embeddings.embedding` 加 HNSW 索引（新表无数据，零风险） | 极低 | 推荐链路避免全表扫 |
| 1.6 | **后端**：`recommendation_service.go` `AcceptRecommendation` 包 `db.Transaction` + `Ask()` 4 处吞 err 改 `log.Errorf` | 低 | 数据完整性 + 可观测性 |
| 1.7 | **后端**：`catalog_sync_service.go` `SyncAll` 包事务 + `GetStatus`/`Pluck` 检 err | 低 | 同步任务幂等 |
| 1.8 | **后端**：删 `config.go:96` `fmt.Println` 调试残留；`preference_profile_service.go:557` 改 `errors.Is` | 极低 | 卫生 |

### 🥈 第二波：补完整性（3-5 天，中风险中收益）

| 序 | 动作 | 风险 | 收益 |
|----|------|------|------|
| 2.1 | 新建 `flow/discovery.md` / `flow/recommendation.md` / `flow/rsshub-catalog.md`（按 APPLY_TODO §D1-D6） | 低 | 业务约束落档 |
| 2.2 | `architecture/map.md` 补 discovery / preference-profile / rsshub-catalog 三域 | 低 | map 同步合规 |
| 2.3 | `configuration.md` 补 4 项新配置；`DefaultRSSHubBaseURL` 提 config + **真正实现 `ai_settings` 覆盖**（消除撒谎注释） | 中 | 配置化 + 消除硬编码 IP |
| 2.4 | **DDL**：`ReadingBehavior` 补 FK `OnDelete:CASCADE` + 迁移建 FK（小表，先 `DELETE` 孤儿再建） | 中 | 消除孤儿数据 |
| 2.5 | **DDL**：`articles`/`feed`/`category`/`narrative_board` 加 `UpdatedAt`（GORM AutoMigrate 自动加列） | 低 | 状态审计 |
| 2.6 | **DDL**：8+ status 列加 CHECK 约束（先 `UPDATE` 清脏值再加） | 中 | 防脏值 |
| 2.7 | **后端**：`feed_handler.go` 4 处 `repository.Repo.DB()` 改走 repo 方法（分层修复） | 中 | 分层一致性 |
| 2.8 | **后端**：fire-and-forget goroutine 加 recover + ctx 传播（reader ×3 + tagmanagement ×5） | 中 | 防进程崩 |
| 2.9 | **前端**：抽 `PER_PAGE_ALL` 常量替换 17 处 magic number；删 `@nuxt/ui` 死依赖；删 `api-helpers.ts` 死代码 | 极低 | 卫生 |
| 2.10 | **前端**：抽 `interface BackupProvider` 到 `app/types/ai.ts`，替换 6 处 any | 低 | 类型安全 |

### 🥉 第三波：技术债重构（按需，高风险高成本，建议拆独立 change）

| 序 | 动作 | 风险 | 收益 |
|----|------|------|------|
| 3.1 | **DDL**：`articles` 25 列宽表拆 `article_completion_jobs` 子表 | 高 | 规范化，但需改读写路径 |
| 3.2 | **DDL**：`NarrativeSummary`/`NarrativeBoard` 5 个 text ID 列改 JSONB / junction | 高 | 可查询，但迁移需回填 |
| 3.3 | **DDL**：`topic_tags.Kind` DROP COLUMN + 删字段 | 中 | 清 dead column |
| 3.4 | **DDL**：`AICallLog` 按月分区 + 保留期 task | 中 | 防无限增长 |
| 3.5 | **后端**：拆 20 个上帝文件（优先 `board_crud_handler.go` 1285 / `orchestrator.go` 1243 / `semantic_board_upgrade.go` 1196） | 高 | 可维护性 |
| 3.6 | **前端**：拆 `BoardThreadBrowser.vue` 2458（按 4 视图模式拆）+ `TopicDetectiveWall.client.vue` 1536 | 高 | 可维护性 |
| 3.7 | **前端**：拆 `stores/api.ts` vs `feeds.ts` 职责重叠；统一 response envelope | 中 | 状态管理清晰 |
| 3.8 | **迁移治理**：实现 Down 迁移执行器 | 中 | 可回滚 |

---

## 6. 决策点（需用户拍板）

以下项**风险/收益比不明确**或**有产品语义影响**，需用户确认后再动：

1. **`articles` 宽表拆分（3.1）**：拆 `article_completion_jobs` 子表会改读写路径，影响 summary/firecrawl 两条 pipeline，工作量大、回归风险高。是否现在做，还是等下次大重构？
2. **`AICallLog` 分区（3.4）**：分区迁移对存量数据需要 reshape，单用户场景可能直接「保留 30 天 + truncate 旧数据」更划算。选分区还是保留期？
3. **`DefaultRSSHubBaseURL` 配置化（2.3）**：注释承诺 `ai_settings` 覆盖但未实现。是补真覆盖（需要 ui + service + 配置项），还是直接删注释改为 config.yaml 静态配置？
4. **前端 `BoardThreadBrowser.vue` 2458 行拆分（3.6）**：这是核心交互组件，拆分回归风险高。是开独立 change 做，还是先只补测试再拆？
5. **`ssr: false`（前端 4.4）**：是否有意全 SPA？若改 SSR 需评估 `three`/`gsap` client-only 兼容。
6. **bool 命名统一（DDL P2.12）**：纯风格统一，工作量和回归面都不小，是否值得做？

---

## 7. 建议立即执行的第一波

若无特殊决策，**建议先落地第一波（1.1-1.8）**：全部低风险、解锁 change archive、止血数据完整性 + 可观测性，预计 1-2 天可完成。第二/三波按独立 change 推进，每波 archive 前跑 `doc-impact.sh verify` + `check-standards.sh`。

**报告生成方式**：4 个 Explore subagent 并行审查（文档/后端/DDL/前端），仅回收结论 + `file:line` 证据，未 dump 文件内容。
