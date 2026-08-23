# Tasks: nightly-throughput-embedding-cache-parallel-crawl

## 1. embedding 缓存（router.Embed 层）

- [x] 1.1 新增 `AIEmbeddingCache` GORM 模型（cache_key PK / model / operation / embedding jsonb / dimensions / input_preview / created_at），注册到 AutoMigrate；编写模型层单测（upsert 幂等 `ON CONFLICT DO NOTHING`）
- [x] 1.2 在 `Router.Embed` 实现 cache_key 计算（SHA-256 of model + "\x00" + join(Input, "\x00")）与命中查询：route 加载后、acquireSem 之前查 `ai_embedding_cache`，命中反序列化返回并写 `ai_call_logs`（Success=true，RequestMeta 含 `cache_hit:true`）
- [x] 1.3 provider 调用成功后 upsert 缓存（含 input_preview 截断 200 字符），写失败仅 warn 不影响返回；并发双写幂等
- [x] 1.4 `log_cleanup` job 增加删除 `ai_embedding_cache` 90 天前记录的逻辑
- [x] 1.5 单测：命中不占信号量、不同 model 不串缓存、命中路径不发起 HTTP、缓存写失败不影响主流程；跑 `go test ./internal/platform/airouter/...`
- [x] 1.6 【2026-08-21 部署观测后调整】缓存写入/查询改为白名单制（仅 `tagmanagement.embedding`）：`section.embedding`、`tagmanagement.auxlabel_embedding`、`discovery.route_embedding` 不查不写（实测命中率 0%/6-10%/一次性，每行 ~30KB 纯浪费）；TTL 90 天 → 14 天（命中集中在写入后 1-2 天，稳态体积 20GB→1-2GB）；补单测：白名单外 operation 不落行不记 cache_hit、TTL 14 天断言

## 2. firecrawl 队列并行化

- [x] 2.1 `job_firecrawl.go` 串行 for 循环改为 3 worker pool（jobs channel 分发），completed/failed 计数改 `atomic.Int32`，每 worker 处理后保留 500ms 限速
- [x] 2.2 核对并发路径：article/feed 查询、失败降级（terminal fallback retag）、`MarkCompleted/MarkFailed`、WS 广播在 3 并发下行为与串行一致；补并发计数正确性单测
- [x] 2.3 跑受影响包测试 `go test ./internal/admin/scheduler/...`（如 job 有专属测试包则只跑该包）

## 3. 验证与收尾

- [x] 3.1 后端门禁：`golangci-lint run ./... && go vet ./... && go build ./...`，影响包测试通过
- [x] 3.2 部署后观测 24h：验证 embedding cache_hit 比例（SQL 统计 ai_call_logs）、firecrawl avg_wait_sec 回落；记录到完工汇报（2026-08-21 核查：tagmanagement.embedding 命中 58→64% 且写入收敛；auxlabel 6-10%、section 0% 属结构性不可命中 → 已触发 1.6 白名单调整；firecrawl avg_wait 3.8h→212s、积压清零、失败率 47%→4%）

## 4. 测试

- `go test ./internal/platform/airouter/` → PASS（含白名单/命中/信号量/幂等/日志标记全部单测）
- `go test ./internal/admin/scheduler/` → PASS（含 firecrawl worker 并发计数 + log_cleanup TTL 14 天断言）

## 5. 文档

<!-- doc-impact: flow database -->
<!-- doc-impact-excuse: api=工作区其他 active change 脏文件命中，非本 change 改动; architecture=同上; configuration=同上 -->

- [x] `docs/reference/flow/ai-summary.md` — 约束 #10：缓存改白名单制（仅 tagmanagement.embedding）+ TTL 90→14 天
- [x] `docs/reference/flow/content-enrichment.md` — 约束 #7 已由 2.1 落地时同步（3 worker 并行 + atomic 计数 + 500ms 限速），本轮复核无需再改
- [x] `docs/reference/database/DATABASE_FIELDS.md` — §3.5 ai_embedding_cache：白名单 + 14 天 TTL
- [x] `docs/reference/database/DATA_LIFECYCLE.md` — log_cleanup 行补 ai_embedding_cache 14 天 + embedding_queues completed 30 天；「队列状态流转/无自动清理」两处同步修正
- 归档后按 §12.2 补 flow 变更溯源链接（ai-summary.md / content-enrichment.md）

## 6. 验证

- `go build ./...` → 零失败
- `go vet ./internal/platform/airouter/ ./internal/admin/scheduler/` → 零失败
- `golangci-lint run ./internal/platform/airouter/... ./internal/admin/scheduler/...` → 0 issues
- `go test ./internal/platform/airouter/` → PASS
- `go test ./internal/admin/scheduler/` → PASS
- 生产库实测（2026-08-21）：`SELECT count(*) FROM ai_embedding_cache` → 仅余 tagmanagement.embedding 19,907 行（白名单外 16,265 行已删 + VACUUM）；ai_call_logs cache_hit 命中 tagmanagement.embedding 58→64%；firecrawl avg_wait 3.8h→212s、pending 积压清零
