## Context

现有 watch 双轨（label/keyword）是只读命中提示，落 `topic_watch_hits`，仅在日报顶部导航；日报主线（采集→lane 聚类→LLM→组装→SaveReport）完全不感知 watch。本次让 watch 参与管线：keyword_topic / sentence_topic 两轨在日报生成中物化为真实 section。动机见 proposal.md；行为契约见 specs/（watch-materialized-topic 新能力 + topic-watch / daily-report-system delta）。

关键既有机制（物化必须无缝接入的四个点）：

1. `GenerateDailyReport`（orchestrator）产出 `report, sections, threadBatches`，`SaveReport` 事务内 upsert——**重生成会删旧 sections/threads 全量重建**，物化因此天然幂等，无需专门的物化重算逻辑。
2. `SaveReport` 事务内 `assignAndUpdateTopics(tx, boardID, date, sections)` 推进话题生命周期（非致命），`RebuildBoardRelations(tx, boardID)` 重建全 board 关系（非致命）。物化 section 在这两处的进出规则见决策 D5。
3. lane 聚类锚 = `BoardPersistentTopic.Centroid`（section embedding 均值滚动）；manual+active 话题已与 auto 一视同仁参与锚定（persistent-topic 能力既有规则）——sentence 轨话题建好即"正式居民"，无需改聚类代码。
4. 摘要择优惯例：`AIContentSummary > FirecrawlContent > Content > Description`（buildArticleContextForTag 既有顺序），keyword 轨文本层沿用。

## Goals / Non-Goals

**Goals:**

- keyword_topic：当天全量未归档文章的机械关键字聚合（零 AI），tag 体系漏网文章可捞
- sentence_topic：一句话向量检索辅助标签池，物化为挂专属持久话题的 section，享受完整生命周期与聚类锚定
- 两条物化轨与既有提示轨并存、互不干扰；物化失败降级不阻断日报

**Non-Goals:**

- 历史日报回填物化 section（v1 明确不做，创建时提示"下期生效"）
- 物化 section 的 LLM threads 汇总（v1 机械组装；观察后可另开 change）
- 正文全文关键字匹配（严格版文本层；全文开关留待后续）
- 物化轨的实时/即时命中通知（只在日报生成时物化）
- watch 与持久话题之外的新实体表（不建新表，扩展既有表）

## Decisions

### D1 物化 phase 的位置：orchestrator 内、Step 7 merge 之后、return 之前

替代方案：`GenerateAndSaveReport` 中 SaveReport 前后单独跑。否掉——放 orchestrator 内让物化 section 走统一 return 路径，`SaveReport` 一条事务全收，无需旁路写库；且重生成幂等由 SaveReport 既有删建语义免费获得。

物化 phase 输入：board 当天窗口 + active 物化轨 watches；输出：append 到 `sections` / `threadBatches` 尾部（`ClusterIndex` 续排、`ClusterLabel` 取固定名/话题名、`ClusterTagIDs` keyword 轨空数组 sentence 轨命中 tag 集、`BestTier=4`/`AvgScore=0`/`QualityBreakdown` 空——无 match 数据、**Embedding 留空**）。report 级 `article_count/event_tag_count/cluster_count` 在 Step 6 已按聚类口径定稿，物化 phase 不改。

### D2 keyword 轨匹配：SQL 取数 + 内存复用既有 DNF 匹配器

取数：单条 SQL 拉当天 `[start, end)` 未归档文章的 `(id, title, coalesce择优摘要, tag_ids)`（择优顺序沿既有惯例，`NULLIF` 处理空串）。匹配：**内存里跑既有 `parseKeywordExpr` / `matchKeywordGroups`**（从 keyword_match.go 抽出文章文本版入口，title+摘要拼接为待匹配文本）。

替代方案：DNF 直译成 SQL `OR-of-ANDs ILIKE`（语义可对齐，但词项叉乘可能组合爆炸、且与 Go 匹配器双实现漂移风险）。单用户当天文章量级（数百到数千）内存匹配完全可控，复用保证语义零漂移。

thread 组装：每篇命中文章一条 thread——`Title=文章标题`、`Summary=择优摘要截断`（截断长度实现时定，Open Question）、`RelatedArticleIDs=[文章ID]`、`TagIDs=该文章 topic tag ids`、`Confidence=1.0`（机械确定性）。

超量保护：当天文章数超过上限（如 5000）时截断扫描并告警，防极端量级拖慢日报。

### D3 sentence 轨检索：watch 侧缓存向量 + 内存余弦

- **缓存列**：`board_topic_watches.embedding_cache`（vector，维度运行时定，沿 SemanticLabel 的无固定维度惯例）。创建 watch 时 embed 一次写缓存；失败不阻断创建，留空待惰性补算。PATCH label/query 时置空失效。日报生成时发现空缓存 → 现场 embed 并回写（该期多一次调用，失败则该 watch 当期降级跳过）。理由：物化只在日报生成时消费向量，惰性补算的时效与消费点天然对齐，且 PATCH 永不被 AI 调用阻塞。
- **检索**：拉取 board 绑定的辅助标签池（`BoardComposition` join `SemanticLabel`，含 embedding），Go 内存余弦相似，阈值 + top-K 截断。池子量级（几十到几百）内存算最简，不引入 pgvector SQL 操作符（维度运行时定 + 跨部署差异）。替代方案：pgvector `<=>` 查询——被维度声明复杂度否掉。
- **解析**：命中辅助标签 → `TopicTagSemanticLabel` → event tag → 限定当天窗口内有文章 → 文章并集。文章并集去重后每篇一条 thread（组装同 D2），`ClusterTagIDs=命中 tag 集`。

配置：阈值与 top-K 走既有配置读取机制（LoadPersistentTopicConfig 同模式），默认初值 threshold=0.55、top_k=8，命中数进日志便于调参。

### D4 sentence 轨专属话题：watch 侧 FK + 复用 manual topic 机制

- 关联：`board_topic_watches.persistent_topic_id`（可空 FK→board_persistent_topics）。话题侧**不加 watch 感知字段**，`source=manual/status=active` 与既有手动话题无差别——persistent-topic spec 因此零改动，且"手动话题参与自动归属"既有规则直接赋予它聚类锚资格。
- 创建时机：首次物化（watch.persistent_topic_id 为空且当期有命中）时创建：`label=watch.label`、`Embedding=Centroid=检索句向量`（**Centroid 必须同时写**——lane 锚定用的是 Centroid 不是 Embedding），创建后立即回写 watch.persistent_topic_id。后续物化复用该 ID，section 直接带 `PersistentTopicID` 进 SaveReport。
- 归属展示字段：`TopicMatchConfidence=manual`（挂载即用户意图，与手动改归属语义一致）、`TopicMatchDistance=0`（非相似度语义，lane_tier 才是真实来源标记）。
- 无命中日：不产 section，话题自然 miss（consecutive_hits 清零走既有规则），无任何特殊处理。

### D5 SaveReport 内两个钩子的进出规则

`assignAndUpdateTopics`：

- `watch_keyword` section（PersistentTopicID=NULL）**必须排除**——否则会被自动归属/L3 建题逻辑收编，违反"keyword 轨不建持久话题"。排除判据：lane_tier 前缀 `watch_`。
- `watch_sentence` section（PersistentTopicID 已设）**正常放行**——生命周期推进（hit_count/consecutive_hits/last_seen）与普通 section 同机制，这正是"正式居民"待遇；post-commit 的 centroid 滚动同样正常接管（首日 centroid=检索句向量，之后被 section 均值演化）。

`RebuildBoardRelations`：排除全部 `watch_*` section（spec：物化 section 不参与关系计算）。物化 section 的 `Embedding` 本来就留空，多数相似度计算天然跳过，但仍需显式按 lane_tier 过滤防未来逻辑漂移。

### D6 提示轨互斥：分流处跳过 + 输入侧过滤

`evaluateWatchHitsWithChat` 的 type 分流处：`watch_*` 类型不进 label 轨也不进 keyword 轨（不写任何 hits）。输入侧两处过滤：keyword 提示轨的 `ListWatchSectionTexts*` SQL 加 `lane_tier NOT LIKE 'watch_%' OR lane_tier IS NULL`；label 轨 prompt 构建时按同样规则剔除物化 section。双向保证"物化 section 不产提示、提示轨不扫物化"。

### D7 删除联动

- keyword_topic：直接删 watch 行（FK 级联清 hits——本来就没有），历史 section 与普通 section 一样保留（重生成才会消失，与常规语义一致）。
- sentence_topic：前端删除弹确认（"将同时归档话题 X"）；后端 DELETE 收 `confirm_archive_topic` 参数，确认后 soft-archive 话题（status=archived，用户显式操作符合红线）再删 watch 行。历史 section 归属不变。

## Risks / Trade-offs

- [SaveReport 失败时 sentence 话题已建但无归属 section] → 孤儿 active 话题无命中自然衰减，且下次生成复用 watch.persistent_topic_id 不重复建；无功能损坏，仅日志可见。接受。
- [文章双处可见（常规 section + 物化 section）导致读者重复阅读] → 用户已拍板接受共存；report 级计数不重算保证统计不失真。
- [sentence 阈值失当（永不命中/命中过泛）] → 阈值/top-K 可配置 + 每期命中数打日志；初值标注"待调参"。
- [换 embedder 后缓存向量维度漂移] → 与 SemanticLabel 现存风险同源同治：沿用 EnsureVectorDimensionOnce 的运行时维度机制；缓存列不声明固定维度。
- [当天文章量级极端膨胀] → D2 截断保护（上限 + 告警）。
- [物化 section 无 embedding，未来依赖 embedding 的下游（检索/关系）需感知] → lane_tier 是唯一判据且已显式过滤两处钩子；后续新下游必须沿用该过滤惯例。

## Migration Plan

单条 migration（顺序执行，可整体回滚）：

1. `ALTER TABLE board_topic_watches`：`ADD COLUMN query TEXT NULL`、`ADD COLUMN embedding_cache vector NULL`、`ADD COLUMN persistent_topic_id BIGINT NULL REFERENCES board_persistent_topics(id)`（ON DELETE SET NULL）。
2. type 列 CHECK 重建为四值（`label/keyword/keyword_topic/sentence_topic`）——CHECK 归 migration 拥有（沿 20260824_0002 惯例，AutoMigrate 不表达）。
3. 无数据回填：存量行 type 默认 label，新列全空合法。

部署顺序：先 DB migration 后发版（既有惯例）；回滚 = 发旧版 + 新列留存无害（可空列不破坏旧代码路径）。部署后行为变化：已有 label/keyword 提示轨用户零感知；新物化轨需用户主动创建，从下一期日报生效。

## Open Questions

- keyword 物化 thread 摘要截断长度（实现时按前端卡片渲染宽度定，不影响契约）。
- `watch_*` section 的前端视觉形态（角标/边框/分组位置），实现阶段出样式稿。
- 当天文章扫描上限的具体值（初值 5000，压测后可调）。
