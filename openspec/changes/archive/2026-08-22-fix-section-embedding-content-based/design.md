# fix-section-embedding-content-based

## Context

日报持久话题三层分桶（L1 直挂 / L2 LLM 裁决 / L3 新建）的路由锚点是 topic 的 **centroid**（近 30 条 section embedding 均值，退化到首义 embedding）。而 section embedding 目前从 `cluster_label` 文本嵌入（`daily_report_orchestrator.go` Step 6），且 L1/L2 命中 section 的 cluster_label 强制取 topic label——形成回声闭环：

```
tag 被 keep → section 标题 = 话题标题 → section embedding = 标题文本向量
→ centroid 永远 = 标题文本向量 → 次日同域 tag 仍落 L2 带 [0.18,0.30] → LLM keep → …
```

实测话题 976（candidate，hit_count=15）连续多日吸附阿联酋贸易、塞浦路斯、加沙空袭等无关线索，全部 section 的 `topic_match_distance ≈ 0.00002`（无信息量），tag 到质心距离 0.24~0.27 全落 L2 带。

相关既有约束：
- 「日报聚类裁决 prompt 历史隔离」（daily-report-system spec）：L2 prompt SHALL NOT 注入候选话题历史 thread 文案——本设计不触碰任何 prompt，纯几何修复。
- `ComputeTopicCentroid` / vacuum 检测 / `planTopicAssignments` 均只消费 `daily_report_sections.embedding`，不感知文本来源。

## Goals / Non-Goals

**Goals:**
- section embedding 代表 section 实际内容（所聚 tag 的事实信息），打破标题回声闭环
- centroid 随真实内容漂移：挂错内容会把质心拉离标题语义，使后续无关 tag 距离出带（自我纠偏的动力学）
- topic_match_distance / fit_distance 恢复判别力（不再有 ≈0 回声距离）
- 历史数据可回刷（section embedding + 受影响 topic 质心 + 关系重建）

**Non-Goals:**
- 不改 L1/L2/L3 阈值（0.18/0.30）
- 不改 L2 prompt / briefs 注入策略（历史隔离 spec 保持；candidate 不注入 briefs 的现状不变）
- 不改 cluster_label 的展示逻辑（L1/L2 section 仍显示 topic label；话题标题演进是另一个话题）
- 不做 L2 keep 事后校验（方案 C，后续独立 change 评估）
- 不自动清理已被污染的话题（976 由用户手动归档）

## Decisions

### D1. embedding 文本 = tag 事实信息拼接，不含 LLM 产物

文本组装（新增纯函数 `buildSectionEmbedText`，orchestrator 与回刷共用）：

```
per tag（cluster_tag_ids 顺序）: "label" + ("：" + description) + ("；代表文章：" + 截断(ArticleContext, 100 runes))
join("\n")，总长截断 480 runes（按 embedding 网关单条输入 512 token 上限校准）
兜底链：无 tags → thread 标题拼接 → cluster_label
```

- **选 tags（label+description+ArticleContext）而非 threads 标题/摘要**：threads 是 LLM 对 tags 的衍生渲染，有幻觉风险；且历史隔离 spec 确立了"匹配几何锚定事实输入、不受 LLM 输出渗透"的原则，embedding 几何应同源。tag 的 description 与 ArticleContext（≤3 篇文章摘要）是文章事实的确定性压缩，信号密度足够。
- **同模态一致性**：L1/L2 路由计算的是 tag semantic embedding ↔ centroid 距离；centroid 由同族的 tag 事实文本均值而来，比 LLM 叙事文本（threads）更可比。
- **ArticleContext 运行时已就绪**：`collectBoardTags` 在聚类前已填充 `TagInput.ArticleContext`（`buildArticleContextForTag`），无额外查询。

### D2. 生成时机不变，位置替换

保持在 threads 收集之后、同日合并之前（现有 Step 6 位置），只替换 embed 输入文本。同日合并（阈值 0.20/0.25）与跨日关系自动转为内容基准。合并组保留 primary section 的 embedding（tag 并集不重嵌入）——合并组通常 0~2 个/天，重嵌入收益小，记为已知限制。

### D3. 新建 candidate 首义向量零改动内容化

`planTopicAssignments` 已传 `sec.Embedding` 作为新话题 embedding，随 D1 自动变为内容向量。centroid 计算逻辑零改动。vacuum 检测（strong-ratio 基于 topic_match_distance < L1）自动恢复判别力。

### D4. 扩展现有回刷端点，而非新增

现状：`POST /api/daily-reports/backfill-embeddings` → `BackfillSectionEmbeddings` 只补 `embedding IS NULL` 的 section（cluster_label 文本），Phase 2 重建关系，**不刷 centroid**。

扩展该端点，新增 query 参数：`recompute`（bool，默认 false）、`board_id`（可选）、`since_days`（可选，recompute 时生效，默认 30，0 不限）：

1. **补缺模式（recompute=false，默认）**：行为同现状，但 Phase 1 文本改用内容化规则（从 DB 反查 tag label/description/文章上下文），与主流水线一致；
2. **重算模式（recompute=true）**：对范围内**全部** section（含已有 embedding）按内容规则重算并更新；
3. 两种模式完成后均对受影响 topic 重算 centroid（`ComputeTopicCentroid`，现状缺失的步骤），再 `RebuildBoardRelations`。

异步执行（goroutine，与现状同款）；embedding 失败的 section 跳过并计数，日志输出统计。

### D5. fit_distance 语义顺带修正

`computeThreadFitDistances` 比较 thread 标题 ↔ section embedding，随 D1 从「thread↔标题」变为「thread↔内容」。无 spec 约束（observability 字段），前端展示语义注释随代码更新。该指标本会直接暴露本次 bug（线索↔冻结标题的回声距离被压到 ≈0）。

## Risks / Trade-offs

- [距离分布整体上移，旧阈值校准漂移] → 内容 embedding 与 tag embedding 同族，但绝对距离分布会变（不再有 ≈0）；观察一个回刷周期内 L1 命中率/真空检测日志（`persistent-topic: board %d anchors …`），必要时仅调 ai_settings 阈值，不改代码。
- [同域吸附不会立刻消失] → 本设计给的是漂移动力学（错挂内容拉走质心 → 无关 tag 出带 → 真叙事以 L3 重聚），非硬闸门；已污染话题（976）需手动归档。若观察期后 L2 keep 误归率仍高，再立 change 做方案 C（keep 后 fit 校验降级）。
- [回刷消耗 embedding API 配额] → 默认 since_days=30 限量，可分 board 分批跑；失败跳过不阻塞。
- [新旧 embedding 混用窗口] → 不回刷时质心窗口（30 条）自然换血约一个月；建议部署后尽快回刷近期窗口缩短混用期。
- [merge 组 embedding 非 tag 并集] → 已知限制，见 D2；不影响归因（归因在 merge 前完成）。

## Migration Plan

1. 部署后端（无 schema 迁移）。
2. 新生成的日报自动走内容 embedding。
3. 手动执行回刷（建议：先 30 天窗口）。
4. 手动归档污染话题（976 等，UI 操作）。
5. 回滚策略：还原代码即可；已回刷的 embedding 数值仍合法（向量同维度），只是语义基准回退，无数据损坏。

## Open Questions

- 回刷是否需要同时重算 `topic_match_distance`（历史 section 的该列）？倾向：不重算（它是生成时点的观测快照，重写历史观测无意义）；质心已由回刷刷新。
