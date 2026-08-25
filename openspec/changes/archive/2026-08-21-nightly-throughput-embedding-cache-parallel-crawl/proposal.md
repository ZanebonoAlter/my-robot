# Proposal: nightly-throughput-embedding-cache-parallel-crawl

## Why

用户仅在夜间启动程序（本地显卡跑 LLM 服务），400+ 篇/天的文章处理被压缩到几小时的消化窗口。实测近 7 天数据（`ai_call_logs`）：LLM 总调用量约 8.2 万次，其中 embedding 占 89%（73,500 次，因 `GenerateEmbedding`/`router.Embed` 无缓存、同一 label 跨文章重复调 API）；firecrawl 队列串行处理（concurrency=1）导致积压 556 个 pending 任务、平均等待从 157s 恶化到 3.8h。两个瓶颈叠加使夜间窗口处理能力不足。

## What Changes

- **embedding 结果缓存**：`Router.Embed`（`backend-go/internal/platform/airouter/router.go:251`）在 route 确定后、真实 provider 调用前，按 `(model, input_hash)` 查询新增的 `ai_embedding_cache` 表；命中直接返回（记 `cache_hit` 日志），未命中调用成功后写缓存。一处改动覆盖所有 embedding 调用点（tagmatch / auxlabel / section / embedding 队列）。
- **firecrawl 队列并行化**：`job_firecrawl.go` 的串行 for 循环改为 worker pool（并发 3），completed/failed 计数器 atomic 化；每 worker 内保留对目标站点的限速 sleep。失败降级、租约、退避逻辑不变。
- **摘要环节不动**：本地 LLM 服务并行度仅 2，摘要并行化收益有限，本次不改（embedding 调用减少后，LLM 并行槽位会自然释放给摘要/抽取）。

## Capabilities

### New Capabilities
- `embedding-cache`: 在 airouter 层对 embedding API 调用结果做持久化缓存，按 `(model, input_hash)` 命中复用，消除跨文章/跨窗口的重复 embedding 调用。

### Modified Capabilities
- `article-content-crawling`: firecrawl 队列消费从串行（concurrency=1）改为并行 worker pool（concurrency=3），要求抓取结果落库、tag 入队、WS 进度广播在并发下保持正确。

## Impact

- **代码**：
  - `backend-go/internal/platform/airouter/router.go`（Embed 缓存逻辑）
  - 新增 `ai_embedding_cache` 表（GORM 模型 + migration）
  - `backend-go/internal/admin/scheduler/job_firecrawl.go`（并行化）
  - `backend-go/internal/platform/ai_models.go` 或新文件（缓存模型定义）
- **数据库**：新增表 `ai_embedding_cache`，无现有表结构变更，无数据迁移。
- **依赖**：无新增外部依赖（pgvector 不需要，向量以 jsonb/text 存）。
- **风险**：embedding 模型同名升级时缓存向量过期——通过 created_at 时间 + log_cleanup 顺带清理 90 天以上记录缓解；缓存命中不计费、不计 latency，`ai_call_logs` 会新增 `cache_hit` 标记便于观测命中率。
