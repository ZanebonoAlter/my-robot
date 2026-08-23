
## 日报黑洞根因：内容化 section embedding 引爆 MergeSimilarSections 阈值失配

## 现象
2026-08-22 21:00 生成的日报，全部 11 个 board 的 section 从常态 4~15 个/board 塌缩为 1~2 个/board（全局 51→19），board 1974/1980/2197 各自 13~16 篇线索文章全部挂进单一 topic（如 topic 1151「美伊博弈升级」），乌克兰/以色列等无关线索被标为 l1_direct anchor_hit 直挂美伊话题；当天 0 个新 persistent topic 诞生（常态 9~16 个/天）。

## 根因链（每环有证据）
1. 2026-08-22 00:24 归档 change `fix-section-embedding-content-based` 落地：section embedding 输入从 cluster_label 标题（旧代码 `embedTexts = sec.ClusterLabel`，orchestrator 旧版）改为内容文本 buildSectionEmbedText（tag label+description+代表文章摘录，480 runes 截断，新文件 daily_report_embed_text.go），00:01~00:33 跑了 section.embedding_backfill 回刷历史。
2. 新几何下同 board 无关叙事的 section 间距压缩到 0.11~0.25（实证：board 3030 的 8-21 section 3245「大模型能力边界」vs 3246「AGENT编程工具」= 0.1106；board 1980 灰区对 0.203~0.247）。长同域中文新闻文本在 qwen3-embedding 下趋同，无阈值可分同/异叙事。
3. Step 7 MergeSimilarSections（daily_report_merge.go，阈值 0.20 确定性/0.25 灰区，7-29 未随 embedding 改动重新标定）在 21:00 首次以新几何运行：大量无关对 <0.20 → union-find 传递闭包链式熔断成 mega-section。
4. 合并规则：article_count 最大者为 primary，primary 的 persistent_topic_id/lane_tier/topic_match_distance 全盘保留（merge 不改 embedding、不重算归因）→ 被吸收的 L3 新叙事 section 连同 threads 静默继承 primary 的锚定，且 L3 组本应 auto_new 创建的 candidate topic 在落库前被吞（SaveReport 前合并）→ 当日新话题断粮。
5. LLM 灰区仲裁无法兜底：board 1980 LLM 正确拒绝了全部灰区对（merge_pairs:[]）但仍塌缩——塌缩发生在确定性 <0.20 边上，仲裁只看得到 0.20~0.25 的对，且 prompt 无锚定/lane 上下文。

## 反驳过的假说
- topic 质心吸引（旧黑洞模式）：否——lane 分桶本身干净，L2 裁决正确判乌克兰 new（ai_call_logs 1367799）。
- 单一聚合文章污染 ArticleContext：否——board 1980 的 13 tag 挂 13 篇不同文章。
- embedding 模型/路由变化：否——qwen3-embedding 三天未变。
- 8-21 历史距离对比：无效——8-21 section embedding 已被 00:01 backfill 改写为新内容 embedding。

## 影响面
- mega-section 展示污染（线索混挂）+ 假 l1_direct 归因；topic 1151 consecutive_hits=3 已达激活线（激活前应先修复重跑）。
- 缓释因素：merge 不重算 embedding，primary 的 embedding 仍是纯美伊内容 → topic 质心污染有限；planLifecycle 只 +1 hit。
- 未评估的连带：跨日关系 RebuildBoardRelations（section-relations）同样消费新几何 embedding，阈值是否失准待查。

## 修复方向（供 change proposal 讨论）
A. 合并尊重锚定边界：不同 MatchedTopicID 或 L3 新叙事 section 禁止被吸收（归因为系统记录，merge 仅展示层）。
B. 明天 21:00 前临时 kill switch 关闭 merge（SaveReport 幂等覆盖式重建，修复后重跑 8-22 即可复原，无需数据手术）。
C. 阈值重标定：已被 0.11 数据点证伪为不可行（单独不可行，可作辅助）。
D. 合并后重算 embedding + 降级 confidence（不再伪装 l1_direct）。
E. 确定性合并对也全部记审计日志（当前只有灰区进 LLM，确定性对无痕）。

<!-- pinned 2026-08-22T14:58:10Z -->
