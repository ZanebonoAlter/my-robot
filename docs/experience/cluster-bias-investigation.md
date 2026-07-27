# 日报聚类偏差调研记录（2026-07-26）

> 来源：探索「日报生成改为基于已有 topic 泳道先 embedding 聚类」方向前的现状调研。
> compact 后恢复上下文：直接 read 本文件 + `ctx_search(queries, source:"cluster-bias-investigation")`（ctx 恢复后）。

## 用户真实意图（当前探索方向）

日报生成改为：**先以已有持久话题（PersistentTopic）泳道为框架，用 embedding 把当天 tag 聚类/分桶到这些泳道**，而非当前的「LLM 自由聚类成 section → 再用 section 标题 embedding 匹配 topic」。

核心诉求：让日报内容组织挂在持续话题骨架上，避免 LLM 自聚类的主观偏差（万能标题 / 强行打包 / 脑补 / 错锚）。

用户原话：「自聚类出来的话题本身就是有点偏，然后再和持久化泳道匹配就看起来更偏了」。

## 已否决方向（勿再走，避免第三次跑偏）

1. **candidate 治理（auto_new 门槛 + candidate GC）**：candidate 不是「泛滥」——`FilterVisibleTopics` 按 `hit_count < upgrade_threshold` 隐藏，用户看不到，不影响体验。GC 清 `hit_count<3` 的 candidate 会**破坏累计命中转正语义**（candidate→active 靠累计 `hit_count ≥ upgrade_threshold=3`，不要求连续；清了就永远攒不够）。相关 change `candidate-lifecycle-governance` 已删除。
2. **tag↔topic 首义向量直接匹配替代聚类**：用 topic「首义向量」单一锚点做硬分类，<0.18 仅抓 10% tag（粒度错配），28.7% section 是 embedding<0.30 却被 LLM 门拦下。但这是用了**最弱的单一锚点**（首义向量=第一条 section 标题，又旧又偏）做的片面证伪，不代表整个 embedding 方向不可行——用户坚持要用更丰富的 topic 泳道表示。

## 痛点实证：LLM 自聚类偏的 5 种形态（2026-07-26 数据）

1. **跑题 thread 搭便车**：fit_distance>0.28 的 thread 占 14%（338/2395）。top 例子：thread「墨西哥总统赴美观赛」(0.440) 挂在「中美稀土贸易摩擦」section；thread「蚂蚁投资薄荷健康」(0.413) 挂在「SpaceX 上市波动」section。根因：ClusterTags 把不相关 tag 误归同簇，thread 在簇内生成随之跑题。
2. **万能包装标题 + 强行打包**：section「开发者工具链细分场景：视频弹幕与 BFF 架构」（弹幕工具 + BFF 不相关）；「Agent 架构重构与标准化协作生态」（5 个不相关 tag 万能包装）。prompt 明令禁止但 LLM 频犯。
3. **脑补标题**：section「存储芯片市场爆发：DRAM与NAND价格暴涨」tag 只有「CXL 技术」（标题脑补「价格暴涨」）；「全球AI算力成本引发计费管控」tags 是 MiniMax 发布 + TRACE 训练框架。
4. **section↔topic 错锚（偏差放大，最关键）**：最松 anchor_hit（dist 0.296-0.300）多为「两个偏标题恰好向量近」。「巴林国家银行合并」↔ topic「卡塔尔赠送星链空军一号」(0.300，完全不相关)；「中年群体三无生活」↔ topic「年轻人一人公司创业」(0.299)。section 标题是万能包装，topic 也是宽泛标题，两个宽泛标题向量天然近 → 错锚。
5. **跨 board 重复发散**：同 tag（如「特朗普指示军方不要袭击伊朗」）挂多 board（`topic_tag_board_labels` 多对多），各 board 独立 ClusterTags → 重复 section + 标题各异。最近 1 天 15 个 tag 出现在 ≥2 board。

## 聚类规模两极分化（结构性问题）

- 单 tag section **923 个（占 53%）**——过半 section 聚类没起聚合作用（`len(tags)<=2` 跳过 LLM 各自成一簇，或 LLM 单着）。
- 大簇 16-27 tag 是同事件 tag 变体扎堆（上游 event tag 去重不足，如「特朗普暂停空袭伊朗」16 个变体当独立 tag）。
- 真正「多事件合理聚合」的中间段少。

## 关键代码位置

- **聚类**：`backend-go/internal/topicgraph/service/daily_report_cluster.go` → `ClusterTags`(L114) + `buildClusterSystemPrompt`(L16，注入 active+candidate topic 框架 + active 近期 section/thread 内容) + `buildClusterPrompt`(L187，列 tag 的 ID/Label/ArticleCount/Description/ArticleContext)。`len(tags)<=2` 跳过 LLM 各自成一簇。聚类粒度 6-15 组。
- **采集**：`daily_report_orchestrator.go` → `collectBoardTags`(L367) 当天 event tag(category=event,status=active)+有 board_label+当天有文章，排除 direction_mismatch，按 article_count DESC。每 tag 带 ArticleContext(代表文章)。
- **锚定**：`daily_report_assignment.go` → `planTopicAssignments`(L155) 双重确认 AND-gate：embedding 门(section 标题向量↔topic 首义向量 ≤0.30) + LLM 门(matched_topic_id)。auto_new 开 candidate(首义向量=section 标题向量)。
- **topic 选择器**：`daily_report_topic_repository.go` → `ListAnchorableTopicsByBoard`(L185) active 全部 + 窗口内(7天) candidate 最多 20 条。
- **section embedding**：orchestrator L250-279，从 cluster_label 文本算（airouter Embed）。
- **管线编排**：`GenerateDailyReport`(orchestrator) Step1采集→Step2去重→Step2.5质量过滤→Step3 ClusterTags→Step5 highlights+threads→Step6 组装section+算embedding→Step7 合并→SaveReport→assignAndUpdateTopics 锚定。

## embedding 基础设施

- **tag 向量**：`topic_tag_embeddings` 表，三路：identity(83244 全覆盖)/semantic(83244 全覆盖)/event_keyword(33262 仅 event)。列：topic_tag_id, embedding_type, embedding(vector), dimension, model, text_hash。
- **topic/section 向量**：board_persistent_topics.embedding / daily_report_sections.embedding（pgvector；topic 首义向量=首条 section 标题向量继承；section 向量=cluster_label 文本向量）。
- 双重确认 GATE1 已用 embedding，但只在 section 生成后做归属，**不参与聚类**（聚类纯靠 LLM）。

## 用户方向能/不能解的（继续探索的输入）

- **能解**：section↔topic 错锚（形态4）——直接挂 topic 不用事后匹配，避免「偏标题碰偏标题」。
- **不解**：跨 board 重复（形态5）、tag 碎片化（16 变体当独立 tag）、全新 tag 归不进任何 topic（需 LLM 或新桶兜底）。
- **待定（继续探索的关键）**：topic 泳道用什么向量表示？首义向量太弱/太旧（10% 抓取率）；候选——历史 section embedding 质心 / 加权代表 / 近期 section+thread 内容聚合向量。这是方向可行性的核心未决问题。

## 覆盖率与表示强度调研（2026-07-26 第二批，方向可行性决定性证据）

### 向量基础设施
- 维度 **2560**（tag semantic / topic / section 三者一致，大模型 embedding）。
- active topic 67 个（全部有 embedding），candidate 582 个。

### 覆盖率：当天 event tag → 该 board active topic 最近邻（最近7天，718 tag）
首义向量 vs 历史质心对比（dist 直方图）：

| 区间 | 首义向量 | 历史质心 |
|---|---|---|
| <0.20（强挂）| 103 (14.3%) | **448 (62.4%)** |
| 0.20–0.30（弱区/错挂温床）| 479 (66.7%) | **261 (36.4%)** |
| ≥0.30（挂不上）| 136 (18.9%) | **9 (1.3%)** |

### 结论（推翻上一轮片面证伪）
- **用 topic 历史所有 section 的质心做锚点，embedding 先聚类完全可行**：<0.20 强挂 62%，<0.25 覆盖 88%（634/718），挂不上仅 1.3%（9 个 tag）。
- 上一轮「首义向量抓 10%」的证伪被推翻——锚点从单条 section 标题（首义）换成历史 section 质心后，强挂 14%→62%。
- 质心样本量：67 active topic 中 27 个 ≥10 条 section（质心最稳），19 个 2-4 条，14 个仅 1 条（退化成首义，仍弱）。avg 9.1 条。

### 潜在偏差（诚实保留）
- 质心 = 历史 section 标题向量的平均，而历史 section 标题是 LLM 聚类产生的（带万能包装/强行打包偏差），质心继承了这些偏差。但比首义（单条）平均掉了随机偏差，方向更稳。
- tag（具体事件）↔ 质心（叙事级框架）距离 0.15-0.20 的语义是「事件落在某叙事框架下」，本就合理，可作为「事件→框架」的锚点。

### 抽样验证结果（质心三区质量）
- **强挂 <0.15（22样本）**：~85% 语义真匹配（Karpathy Vibe Coding↔Vibe Coding 框架 0.096；Kimi K3↔智谱 GLM 开源竞争 0.092；Claude Code 工具↔Claude Code 工具链 0.088）。仍有 ~15% 沾边误判：「美伊冲突升级」↔「黎巴嫩停火成为美伊谈判前置」0.079（沾中东但不同叙事）；「Agent 技能实战第三课」↔「Agent 架构重构」0.111（教程挂架构框架）。质心大幅改善但仍非纯净。
- **弱区 0.25-0.30（18样本）**：错挂温床依旧。「卢拉警告干预巴西大选」↔「以色列安全焦虑」0.253（完全不相关，纯沾地缘）；「特斯拉工业扩张」↔「SpaceX 上市」0.252（沾马斯克）；「小鹤音形词典」↔「开发者工具链」0.256（牽强）。质心没消除弱区错挂，只缩小比例。
- **挂不上 >0.30（8样本）**：真新事件 + 噪声 tag。该开新 topic 的（WAIC 主旨讲话 0.318、商务部出口管制 0.369、欧盟反垄断 0.342）+ 该过滤的噪声（名人婚礼 0.348）。且集中挂到「中国中央银行相关新闻」这类过宽 topic 当最近邻——过宽 topic 质心需审视。

### 方案雏形（数据支撑的分层混合）
- **L1 强挂（质心 dist<0.18/0.20，~62%）**：直接挂 topic，高置信。抽检 ~15% 沾边误判→加 LLM 轻校验或双向最近邻过滤。
- **L2 弱区（0.20-0.30，~36%）**：不硬挂，交 LLM 在「embedding 预筛的候选 topic + tag」上判断（LLM 退化从「自由聚类全量」→「只判弱区子集」，输入聚焦、偏差可控）。
- **L3 挂不上（>0.30，~1.3%）**：开新 cluster（LLM 起新叙事）或单 tag / 过滤噪声。
- section 天生挂 topic（L1直挂/L2 LLM挂/L3新topic），消除事后 section↔topic 匹配（形态4）。

### 待验证（下一步）
- tag 碎片化规模（同事件变体两两距离）→ 决定 L1 前是否 tag 去重。
- 双向最近邻（tag↔topic 互为最近）能否把 L1 沾边误判降到 <5%。
- 过宽 topic（如「中国中央银行相关新闻」）的质心如何修正（多向量/代表点）。

## 方案验证结果（2026-07-26 第三批，三个数据缺口）

### tag 碎片化规模（这轮不治，仅记录）
同 board 当天 tag 最近邻 tag 距离：<0.10 占 33%（232/713），<0.15 占 54%。证实「特朗普暂停空袭伊朗 16 变体」是系统性现象，非个例。影响：L1 强挂时近重复 tag 都挂同一 topic，section 冗余（一堆相似 tag）但归属正确，不致命。留作后续 change。

### 双向最近邻过滤——推翻，不适用
- 单向质心 <0.18：340 tag；双向（互为最近）<0.18：84 tag——**砍掉 75%**。
- 抽样：被砍的多是**真强匹配**（「Kimi K3」↔「智谱GLM开源竞争」0.092、「Vibe Coding 工作流」↔「Vibe Coding 框架」0.110 被误杀），只有少数是真沾边（「美伊油价波动」↔「黎巴嫩停火」0.115）。
- 原因：topic 数 << tag 数（67 vs 718），每 topic 只有一个「最近 tag」，多对一的正确关系被误杀。
- 结论：L1 用**单向质心 <0.18**（340 tag，47%），不用双向。之前「双向把误判压<5%」的设想作废。

### 吸尘器 topic——质心表示的真实风险（比沾边误判更要紧）
按 attract 数 + weak 比例排：
- 「智谱 GLM-5.2...」吸 86 tag（strong56/mid30）——核心叙事大吸尘器，强挂多但 section 会臃肿。
- 「中国中央银行相关新闻」吸 17（**strong0/mid11/weak6**）——质心坏，把出口管制/WAIC/反垄断全吸成最近邻（与抽样挂不上区一致）。
- 「XR 硬件生态爆发」吸 23（strong2/mid21）、「开发者工具链从本地调试走向平台化」吸 34（strong3/mid31）、「Meta 加速布局AI算力与金融」吸 21（strong1/mid19）——万能标题吸尘器，strong 极少 mid 一堆。
- 本质：这些是 LLM 聚类产出的万能包装标题（形态2），质心继承其宽泛偏差，把沾边 tag 都吸过来。
- 方案修正：L1 要加**吸尘器检测**——strong/(strong+mid) <20% 的 topic 判为过宽，挂到它的 tag 降级到 L2 让 LLM 判，或用 strong tag 重算子质心。

### L1 最终形态（修正后）
- 主规则：单向质心 <0.18 直接挂 topic（340 tag，47%）。
- 吸尘器检测：strong/(strong+mid) <20% 的 topic 标记过宽，挂它的 tag 降级 L2。
- 接受 ~15% 沾边误判（单向无法根除），靠 L2 兜底或后续治理。

## 决策与状态

- 方向已定 + 数据已齐：**embedding 先聚类（质心）+ LLM 兑底/校验**，三层 L1/L2/L3。
- 用户已拍板：① 切点 0.18；② L2 用「预分配候选 topic，LLM 三选一（留/换/新）」；③ 碎片化 + 跨 board 重复 这轮不碰（另开 change）。
- 方案修正：双向 NN 作废→L1 用单向；新增「吸尘器 topic 检测」。
- 下一步：可以写 openspec change 了（遵循 `docs/reference/开发执行规范.md`）。
- 记录索引：`docs/experience/cluster-bias-investigation.md`（三批调研全貌）。
