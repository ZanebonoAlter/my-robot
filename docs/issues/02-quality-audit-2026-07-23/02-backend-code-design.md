# 后端代码设计规范审计报告

> **审计对象**: `backend-go/internal/` 全域（admin/app/dataenrichment/models/platform/reader/tagmanagement/topicgraph，约 6.2 万行 Go）
> **对照规范**: `docs/reference/standard/backend/{package-layout,code-style,lint,testing}.md` + `docs/reference/architecture/backend.md`
> **评级**: **B−**（技术债治理后升至 **B**：model tag Top3 治理 106 处 + topicgraph/reader 4 处 handler 下沉 + 约束完整性回归测试 + check-standards H 段守门）
> **审计日期**: 2026-07-23

## 技术债治理进度（2026-07-23）

| 治理项 | 状态 | 说明 |
| ------ | ---- | ---- |
| **model tag Top3 治理**（M2，原 154 处） | ✅ 已完成 Top3（106处） | 「先补显式迁移 20260723_0001（含回填）→ 加约束断言测试 constraints_test.go → 删 tag」三步走；3 个 jsonb 字段（metadata/aliases/context_layers）保留 default（serializer:json 零值省略必需）；剩余 48 处长尾留给未来 |
| check-standards H 段守门 | ✅ 已加 | 扫描 Top3 文件禁止 GORM tag `not null`，防回潮 |
| topicgraph handler 越层（2处） | ✅ 已修复 | `CountSectionsByTopic` 下沉 + `LoadPersistentTopicConfig` 改走 `repository.Repo.DB()` |
| reader content_completion handler 越层（2处） | ✅ 已修复 | 新增 `ListArticlesByFeedAndStatuses` + 复用 `GetArticle` |
| handler 越层（系统性，admin/reader 大头） | ⏳ 待排期 | admin「假三层」补建半个 service 层 + reader article/feed_handler 53 处，工作量大，留给专门 change |
| IDOR 漏洞（H1） | ✅ 已修复（前一轮） | 见前一轮汇报 |

**门禁结果**：lint/vet/build 全过（0 issues）；测试仅 develop 预先存在的 2 包失败（tagmanagement/service/auxlabel + board，与本次治理无关，duplicate key seed 问题）。

## 总体评价

分层骨架整体扎实：domain 白名单 8 个目录全部合规、三层结构（handler/service/repository）在多数域落地、wire.go/routes.go 分离规范。并发处理在关键路径上展现了**超出平均水平**的工程意识。但存在两个系统性短板拉低评级：**handler 越层访问 DB 普遍存在**、**models 约 154 处 GORM tag 违反迁移分工红线**。

## 正面发现（保持）

| 亮点 | 位置 | 说明 |
| ---- | ---- | ---- |
| ETF 缓存规避 `sync.Once` 永久缓存失败 | `dataenrichment/service/tool_registry.go:246` | 用 mutex + loaded flag + 失败可重试，注释明确说明设计意图 |
| 合并事务 `FOR UPDATE` 双锁 + 死锁事故记录 | `tagmanagement/handler/tag_merge_preview_handler.go:188,229-234` | 注释记录 FK 导致 application-level deadlock 真实事故及修复 |
| tag 队列限流 + goroutine 防崩溃 | `tagmanagement` tag_queue | `sem` channel 限流 + `defer recover` |
| IDOR 校验正面范例 | `dataenrichment/handler/handler.go:401,482` | `getResult`/`triggerDebate` 正确校验 `result.PersistentTopicID != topicID` |

---

## 按域问题清单

### reader 域（6053 行 / 29 文件）

#### [High] Handler 直接操作 DB，绕过 service/repository

- **位置**: `reader/handler/article_handler.go:18,49,116,203,327,395,455,542`（30+ 处 `repository.Repo.DB()`）；`reader/handler/category_handler.go`
- **证据**:
  ```go
  // handler 内直接做 GORM 查询
  repository.Repo.DB().Model(&models.Article{}).Where(...)
  // handler 内构造队列
  queue := tagging.NewTagJobQueue(repository.Repo.DB())  // article_handler.go:410
  ```
- **规范依据**: package-layout.md §Anti-Patterns「❌ Handler 直接访问 DB」；code-style.md §业务归属「Handler 不直接访问 DB」
- **建议**: 查询下沉到 `reader/service` 或 `reader/repository`，handler 只调 service 方法

#### [Medium] feed_handler.go（595 行）无测试且承载重逻辑

- **位置**: `reader/handler/feed_handler.go`
- **建议**: 拆分 + 补 service 层测试

#### [Low] OPML 导入异步刷新吞错

- **位置**: `reader/handler/opml.go:165` `_ = feedService.RefreshFeed(...)`
- **建议**: best-effort 可接受，但建议至少 warn 日志

---

### tagmanagement 域（17860 行 / 73 文件 — 最大域）

#### [High] Handler 直接操作 DB + handler 内 new service + handler 内开事务

- **位置**: `tagmanagement/handler/tag_management_handler.go:29,68,72`；`board_crud_handler.go:103-105`；`tag_merge_preview_handler.go:55,101,107,129,133,188`
- **证据**:
  ```go
  // handler 内直接查 DB
  repository.Repo.DB().Where(...).First(&source, body.SourceTagID)
  // handler 内构造 service
  service.NewAuxiliaryLabelService(repository.Repo.DB(), nil)
  // handler 内开事务（service 接口设计被迫让 handler 编排）
  repository.Repo.DB().Transaction(...)
  ```
- **规范依据**: package-layout.md §Anti-Patterns「Handler 直接访问 DB」
- **深层原因**: service 接口设计本身有问题——`HardMergeTags uses repository.Repo.DB() which opens a separate transaction`（代码注释自认），导致 handler 被迫编排事务
- **建议**: 合并/标签查询进 service 层；事务封装在 merge service 内

#### [Medium] board_crud_handler.go（1285 行，全项目最大文件）职责过重

- **位置**: `tagmanagement/handler/board_crud_handler.go`
- **证据**: 1 个文件 20+ 方法 + 2 个 package-level mutex + 1 个 package-level 缓存（`auxClusterCache` :582，TTL 10min）
- **问题**: 包级可变状态（`auxClusterCacheMu`/`auxClusterComputeMu`/`auxClusterCache`）混在 handler 包
- **建议**: 聚类计算+缓存抽到 `service/board/`，handler 只做 HTTP 适配

#### [Medium] 两个超大 service 文件

- **位置**: `tagmanagement/service/board/semantic_board_upgrade.go`（1196 行）、`tagmanagement/service/auxlabel/auxiliary_label_service.go`（799 行）
- **建议**: 按阶段拆分（upgrade 的聚类/补 co-tag/LLM 判断可独立）

#### [Medium] 辅助标签向量列 DDL 用 `fmt.Sprintf` 拼 SQL 标识符

- **位置**: `tagmanagement/service/auxlabel/auxiliary_label_service.go:767,769-771,773-775`
- **证据**:
  ```go
  db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS idx_semantic_labels_%s", column))
  db.Exec(fmt.Sprintf("ALTER TABLE semantic_labels ALTER COLUMN %s TYPE %s", column, expected))
  ```
- **风险评估**: `column` 来自内部常量（"embedding"/"merge_embedding"），非用户输入，**实际无注入**，但模式危险
- **规范依据**: code-style.md「raw SQL 是否参数化」
- **建议**: 白名单校验 `if column != "embedding" && column != "merge_embedding" { return err }` 或常量直接写两条函数

#### [Low] 合并后入队吞错

- **位置**: `tag_merge_preview_handler.go:242` `_ = service.EnqueueMergeReembedding(...)`
- **风险**: 合并已提交，入队失败丢弃 → 该 tag 不会触发 re-embedding
- **建议**: 失败时记录告警，或提供补偿接口

---

### topicgraph 域（11835 行 / 45 文件）

#### [High] daily_report_handler.go（760 行）在 handler 内直接 DB 查询 + 调配置

- **位置**: `topicgraph/handler/daily_report_handler.go:294,307`
- **证据**:
  ```go
  database.DB.Table("daily_report_sections")...
  repository.LoadPersistentTopicConfig(database.DB).UpgradeThreshold
  FilterVisibleTopics(...)  // :309 业务过滤在 handler
  ```
- **规范依据**: package-layout.md §Anti-Patterns；code-style.md「业务逻辑在 service/，不在 handler」
- **建议**: 可见性过滤/section 查询下沉到 topicgraph service

#### [Medium] daily_report_repository.go（994 行）单文件过大

- **位置**: `topicgraph/repository/daily_report_repository.go`
- **建议**: 按职责拆分（报告/topic/section/assignment）

#### [Medium] daily_report_merge.go（267 行）等核心合并逻辑无测试

- **位置**: `topicgraph/service/daily_report_merge.go`
- **现状**: service 层测试覆盖尚可（22 源 / 23 测试），但合并逻辑是空白
- **建议**: 补合并逻辑单测

---

### admin 域（4667 行 — 最弱域：21 源 / 仅 4 测试）

#### [High] ai_handler.go、preferences_handler.go 整个 handler 是 DB 操作，无 service 层

- **位置**: `admin/handler/ai_handler.go:42,91,108,139,156,162,171,180,255,281,293`；`admin/handler/preferences_handler.go:43,59,94,113,137,141,146`
- **证据**:
  ```go
  repository.Repo.DB().First(&provider, uint(id))
  repository.Repo.DB().Delete(&provider)
  repository.Repo.DB().Model(&models.ReadingBehavior{}).Update(...)
  ```
- **规范依据**: 三层结构强约束；admin/scheduler 包 `job_*.go` 也普遍直接 `repository.Repo.DB()`
- **建议**: 补 `admin/service` 业务层；admin 是 handler 直连 DB 的最严重区域

#### [High] 调度器 9 个 job 零单元测试

- **位置**: `admin/scheduler/job_*.go`
- **证据**: 9 个调度任务（auto_refresh/aux_label_cleanup/blocked_article_recovery/content_completion/daily_report/firecrawl/log_cleanup/preference_update/tag_quality_score）**全部无 `_test.go`**（仅 base.go、job_board_upgrade_suggest 有测试）
- **规范依据**: 测试覆盖维度；架构 backend.md §当前边界已知问题「scheduler/ 包缺少单元测试」自认
- **建议**: 至少为 firecrawl/content_completion/tag_quality_score 这类写库任务补集成测试

#### [Medium] registry.go:42 用 panic 处理重复注册

- **位置**: `admin/scheduler/registry.go:42`
- **证据**: `panic(fmt.Sprintf("scheduler %q already registered", name))`
- **规范依据**: code-style.md「禁止 panic 处理错误」
- **评估**: init 期注册冲突属可接受的「编程错误 fail-fast」，但严格按规范应返回 error

---

### dataenrichment 域（13534 行 / 47 文件 — 含大量新增未提交文件）

#### [High] 🔴 IDOR 漏洞：review 偏差/应用 handler 未校验归属 — ✅ 已修复（2026-07-23）

> **修复**：TDD 流程——先写 3 个复现测试（`TestUpdateReviewDeviation_IDORProtection`/`TestApplyReview_IDORProtection`/`TestSedimentQA_IDORProtection`）确认漏洞（Red），再修 handler 加 `parseTopicID` + 归属校验（Green）。审计中额外发现 `sedimentQA` 同源 IDOR（qa 经 result 间接校验），一并修复。全包测试通过无回归，lint/vet/build 全绿。

- **位置**: `dataenrichment/handler/handler.go`（`updateReviewDeviation`/`applyReview`/`sedimentQA`）
- **证据**（已主线程验证）:
  ```go
  // 路由带 :topicId
  // PUT /persistent-topics/:topicId/enrichment/reviews/:id        (handler.go:148)
  // POST /persistent-topics/:topicId/enrichment/reviews/:id/apply (handler.go:149)

  func (h *EnrichmentHandler) updateReviewDeviation(c *gin.Context) {
      id, ok := parseIDParam(c, "id")   // ← 只读 :id，从不读 :topicId
      ...
      h.repo.UpdateTopicEnrichmentReviewDeviation(ctx, id, req.DeviationSummary)
  }
  func (h *EnrichmentHandler) applyReview(c *gin.Context) {
      id, ok := parseIDParam(c, "id")   // ← 同样不校验归属
      ...
      h.repo.ApplyTopicEnrichmentReview(ctx, id)
  }
  ```
- **风险**: `TopicEnrichmentReview` 有 `PersistentTopicID` 字段（`repository/models.go:97`），完全有能力校验却没做。攻击者可遍历 `:id` 改任意 topic 的 review 偏差摘要/应用状态
- **对比**: 同文件 `getResult`(:401)、`triggerDebate`(:482) 都正确做了 `result.PersistentTopicID != topicID { 404 }`
- **规范依据**: handler 安全维度（IDOR）
- **建议**: 在 `GetTopicEnrichmentReviewByID` 返回后加归属校验，与 getResult 保持一致。**修复成本约 15 分钟**

#### [High] 根包布局违规：9 个非 routes.go/wire.go 文件

- **位置**: `dataenrichment/` 根包（已验证）
- **证据**: 根包下存在 9 个非测试 `.go` 文件：
  ```
  active_topic_lister.go, board_config_impl.go, board_listers_impl.go,
  capability.go, production_wiring.go, scheduler_jobs.go, scheduler_next_run.go
  ```
- **问题**: `board_config_impl.go`/`board_listers_impl.go` 实际是**生产环境 repository 实现**（`dbBoardConfigReader`/`dbBoardLister` 直接做 GORM 查询），明显应进 `repository/` 子包
- **规范依据**: package-layout.md §Domain 三层结构「根包**只**放 routes.go 和 wire.go」
- **建议**: `*_impl.go` 移进 `dataenrichment/repository/`；`production_wiring.go`/`scheduler_jobs.go` 合并进 `wire.go`

#### [Medium] production_wiring.go 用 raw SQL 直查 topicgraph 的表，绕过其 repository

- **位置**: `dataenrichment/production_wiring.go:28-31,50-53,70`
- **证据**:
  ```go
  r.db.WithContext(ctx).Raw(... FROM daily_report_sections ds JOIN board_daily_reports bdr ...)
  // 注释：Uses raw SQL to avoid circular imports with the topicgraph package
  ```
- **风险评估**: raw SQL 字面量字符串（无注入），但把 topicgraph 的 schema 知识复制到 dataenrichment，schema 变更会静默漏改。经 grep 确认 topicgraph 与 dataenrichment **当前并无实际循环依赖**（互相不 import），注释的「avoid circular」是预防性的
- **建议**: 在 topicgraph/repository 暴露只读查询方法供 dataenrichment 调用，或通过接口注入

#### [Medium] orchestrator.go（1243 行）单文件偏大但内聚

- **位置**: `dataenrichment/service/orchestrator.go`
- **评估**: 职责划分清晰（按 spec 三角色 interpret/agentLoop/toolLoop 分段，每段独立函数），不算上帝对象，但已超 800 行阈值
- **建议**: 可把 prompt 构建（`interpretPrompt`/`agentLoopSystemPrompt`）抽到单独 `prompts.go`

#### [Low] debate_service.go:125 吞错丢弃辩论结果

- **位置**: `dataenrichment/service/debate_service.go:125` `_ = s.repo.CreateStockDebateResult(ctx, dr)`
- **建议**: 失败时至少 warn

---

### models 域（系统性问题）

#### [High → Medium] 约 154 处 GORM tag 违反迁移分工规范

- **位置**: `internal/models/ai_models.go`(30处)、`topic_graph.go`(27处)、`semantic_label.go`(21处)、`feed.go`(13处)、`job_queue.go`(13处) 等，全包约 154 处
- **证据**（已主线程验证，grep 实测 154）:
  ```go
  Name string   `gorm:"size:50;unique;not null;index"`
  Enabled bool  `gorm:"not null;default:true;index"`
  ```
- **规范依据**: code-style.md §GORM model tag 与迁移「model tag 只写字段名/类型/json，**不写 not null/default**——让显式迁移唯一管 DB 约束」，并记录了 `ai-call-logging-schema` 事故教训
- **风险**: AutoMigrate 启动时与显式迁移的「ADD NULL → 回填 → SET NOT NULL」三步竞争，有历史数据的库上 AutoMigrate 先失败污染启动日志
- **建议**: 批量移除 model tag 里的 `not null`/`default:xxx`，DB 约束收敛到 `postgres_migrations.go`。建议分 domain 渐进迁移

---

### platform 域（6854 行 / 46 文件）

#### [Low] database.DB / repository.Repo 全局单例普遍使用

- **位置**: 全后端
- **规范依据**: 架构 backend.md:472 §当前边界已知问题已自认「部分域仍使用全局单例」
- **建议**: 长期向依赖注入收敛（非阻塞）

---

## 后端总评：B−

**升 A- 的路径**（按 ROI 排序）：

1. **修 IDOR（H1）** — 成本最低、安全收益最高（~15 分钟，1 文件）
2. **治 handler 越层（M1）** — 从 admin 域入手补 service 层，是评级主因
3. **治 models tag（154 处）** — 消除迁移竞争风险，恢复 code-style.md 红线
4. **补 admin 调度器单测（M4）** — 写库任务无回归保护是最大测试盲区

前两项修复即可升至 B+；四项全做可冲击 A-。
