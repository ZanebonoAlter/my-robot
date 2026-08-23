# Design: nightly-throughput-embedding-cache-parallel-crawl

## Context

- 文章处理管线：RSS 刷新（并行 8）→ firecrawl 队列（**串行**，300s tick 领 50 个）→ 摘要（串行，不动）→ 打标队列（并发 3）。
- 用户仅夜间启动程序（本地显卡 LLM，并行 2），400+ 篇/天挤在几小时窗口内消化。
- 实测 7 天数据：embedding 调用 73,500 次（占 LLM 总调用 89%），其中 `tagmanagement.embedding` 54,043 次 + `tagmanagement.auxlabel_embedding` 19,459 次；firecrawl 任务平均等待恶化到 3.8h（pending 积压 556）。
- embedding 调用链：`TagMatch`（slug/alias 精确匹配不花钱）→ 未命中 → `FindSimilarTags` → `GenerateEmbedding` → `router.Embed` → provider HTTP。embedding 文本只由 tag 自身属性构成（label+description+aliases+category），同一 label（如 "OpenAI"）跨文章重复出现，每次都实时调 API。

## Goals / Non-Goals

**Goals:**
- embedding 缓存命中率 ≥90%，消除跨文章/跨窗口（程序重启后仍有效）的重复调用
- firecrawl 队列吞吐 ×3（串行 → 并发 3），夜间窗口内消化完当日积压
- 缓存命中可观测（ai_call_logs 可区分 cache_hit）

**Non-Goals:**
- 摘要并行化（本地 LLM 并行 2，无意义；embedding 瘦身后槽位自然释放）
- 常驻部署方案
- 打标队列/抽取逻辑改动
- 现有 `topic_tag_embeddings` 表结构变更（该表是 per-tag 落库语义，与 API 级缓存分离）

## Decisions

### D1: 缓存放在 `Router.Embed` 层（而非 EmbeddingService 层）

**选择**：route 确定后、provider 循环前，按 `(model, input_hash)` 查 `ai_embedding_cache` 表。

**理由**：一处改动覆盖全部 4 个调用点（tagmatch 的 `GenerateEmbedding`、auxlabel 的 `auxiliary_label_service.go:688`、`section.embedding`、embedding 队列）。放 service 层则要改多处且未来新调用点容易漏。

**备选**：
- EmbeddingService 层加内存 LRU——重启丢失，用户隔天启动的窗口场景下命中率低；且 auxlabel 不走该 service。
- 不缓存、改批量 API——调用点分散，聚合改造复杂，收益不如缓存确定。

**key 设计**：`hash(model + "\x00" + strings.Join(req.Input, "\x00"))`，text hash 用 SHA-256 截断 hex。model 参与键保证换模型/跨 provider fallback 时向量空间不会串。fallback provider 成功的结果同样按其 model 落缓存，键不冲突。

### D2: 缓存表 `ai_embedding_cache` 用 jsonb 存向量（不用 pgvector）

**选择**：`ai_embedding_cache(cache_key text PK, model text, operation text, embedding jsonb, dimensions int, input_preview text, created_at)`。命中后 json.Unmarshal 还原 `[][]float64`。

**理由**：缓存命中路径只是"取回字节 → 反序列化 → 返回"，不需要相似度计算（那是 `topic_tag_embeddings` + pgvector 的职责）。jsonb 避免引入向量维度约束，任意 embedding 模型（1536/1024/768…）都能存。

**写入策略**：仅白名单 operation（`tagmanagement.embedding`，tag 固定属性输入、跨文章重复）写缓存；provider 调用成功后同步 upsert（单行 upsert，失败只记 warn 不阻塞返回）。并发下同 key 双写用 `ON CONFLICT DO NOTHING` 幂等处理。白名单外 operation（`section.embedding` 文章正文、`tagmanagement.auxlabel_embedding` 标签+文章上下文一次性组合、`discovery.route_embedding` 路由回填）不查不写：部署后实测（2026-08-20/21）这三类命中率分别为 0%/6-10%，每行 ~30KB（2560 维 jsonb），属纯存储浪费，命中也几乎全部发生在写入后 1-2 天内。

**清理**：`log_cleanup` job 顺带删除 `created_at < now() - 14d` 的记录（原定 90 天：夜间窗口场景下命中集中在写入后 1-2 天，90 天稳态会把表拄到几十 GB；14 天稳态约 ~1-2 GB）。

### D3: 命中时仍写 `ai_call_logs`（标记 cache_hit）

**选择**：命中后记一条 `Success=true, LatencyMs≈0`，`RequestMeta` 附 `{"cache_hit":true}`。

**理由**：现网 `ai_call_logs` 是本次性能调查的核心数据源，命中率必须可量化验证（改动后 `SELECT count(*) FILTER (WHERE (request_meta::jsonb->>'cache_hit') = 'true')::float / count(*) FROM ai_call_logs WHERE capability = 'embedding'`；request_meta 列为 TEXT，需先转 jsonb）。不额外加表。

### D4: firecrawl 并行用固定 worker pool（并发 3），不改队列 claim 语义

**选择**：`job_firecrawl.go` 中 claim 50 个后，起 3 个 goroutine 消费同一 jobs channel；completed/failed 计数器改 `atomic.Int32`；每 worker 处理完一个 job 后 `time.Sleep(500ms)` 保留（对目标站点的礼貌限速）。

**理由**：
- `ReadabilityCrawler` 仅持有 `*http.Client`（并发安全），无共享可变状态；`ws.Hub.BroadcastRaw` 有 RWMutex 保护；GORM 并发安全——并行化无前置障碍。
- 并发 3 而非更高：readability 打的是目标网站，过高并发易触发反爬；且抓取通常 1-2s/篇，并发 3 吞吐 ≈ 6-9 篇/分钟，一晚消化 400+ 篇绰绰有余。
- 不改 claim 批大小（50）与 300s tick：lease 机制已保证 worker 崩溃后任务可回收。

**备选**：改 firecrawl 官方 API 并发——用户主要走降级 readability（本地抓取），firecrawl API 是兜底，优化它收益小。

### D5: 缓存读写不占用 embedding 信号量

**选择**：缓存查询在 `acquireSem` **之前**执行（route 加载后即可查），命中直接返回，不进信号量。

**理由**：缓存命中是本地 DB 读（<5ms），让它排队等信号量毫无意义；且腾出的槽位让真实调用更快拿到。信号量内只包 provider 调用 + 缓存写入。

## Risks / Trade-offs

- [模型同名升级导致缓存向量与新生成向量不一致] → 90 天 TTL 清理；极端情况可手动 `TRUNCATE ai_embedding_cache`，无数据正确性风险（缓存只影响"匹配相似度"，不影响落库数据）。
- [input_preview 截断不当导致排查困难] → 存前 200 字符，足够人工核对。
- [并行抓取触发目标站反爬/限流] → 并发仅 3 + 每 worker 500ms 间隔；失败路径已有 5 次退避重试（1min→30min），最坏降级 RSS description，不会卡死管线。
- [WS 进度广播乱序] → 广播的是计数快照（atomic 读），乱序只影响 UI 瞬时显示，最终一致。
- [缓存表写入与读放大] → 每次未命中多一次 upsert（写）+ 每次调用多一次按 PK 查询，均为主键操作，量级 7 万/周，可忽略。

## Migration Plan

1. 部署：新表由 GORM AutoMigrate 创建，重启后端即生效。无存量数据迁移。
2. 验证：观察 24h 内 `ai_call_logs` 中 embedding 类 operation 的 cache_hit 比例（预期 >90%）；观察 firecrawl 任务 avg_wait_sec 回落到分钟级。
3. 回滚：还原代码部署即可；`ai_embedding_cache` 表留存无害，或手动 DROP。

## Open Questions

- 无（并发数 3、TTL 90 天、批大小 50 均为可调经验值，先落地观测再调）。
