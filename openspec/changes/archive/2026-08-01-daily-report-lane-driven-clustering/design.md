# Design — daily-report-lane-driven-clustering

## 1. 背景与根因

当前日报聚类两段式（LLM 自由聚类成 section → section 标题 embedding 匹配 topic），偏差双层叠加。三批调研（`docs/experience/cluster-bias-investigation.md`）实证 5 形态偏差：跑题 thread 搭便车（`fit>0.28` 占 14%）、万能包装标题强行打包、脑补标题、section↔topic 错锚（事后匹配放大）、跨 board 重复发散。

根因：LLM 同时承担「发现新叙事 + 归并旧框架 + 起标题」三职，事后再用两个宽泛标题做向量匹配。

## 2. 数据支撑（决定方案形态）

| 指标（最近7天 718 event tag） | 首义向量 | 历史质心 |
|---|---|---|
| 强挂 <0.20 | 14.3% | **62.4%** |
| 弱区 0.20–0.30 | 66.7% | 36.4% |
| 挂不上 >0.30 | 18.9% | **1.3%** |

- 质心强挂抽样 ~85% 语义真匹配，~15% 沾边误判（中东系 / 教程类）。
- **双向最近邻过滤被推翻**：单向<0.18 有 340 tag，双向（互为最近）仅 84（误杀 Kimi K3↔智谱GLM 等真强匹配）→ **L1 用单向质心 <0.18**。
- **吸尘器 topic**：万能标题 topic 质心过宽，`strong/(strong+mid)<20%`（中国央行新闻 strong=0、开发者工具链 strong3/mid31），需检测降级。
- 碎片化：33% tag 有 <0.10 近重复邻居（**本 change 不治**）。

## 3. 方案设计

### 3.1 三层分桶（详见 spec delta）
L1（<0.18）直挂 / L2（[0.18,0.30]）LLM「留/换/新」三选一 / L3（>0.30）新建 candidate。section 天生挂 topic，消除事后 section 标题↔topic 匹配（形态4根源）。

### 3.2 质心表示
`board_persistent_topics.centroid`（vector 2560），`centroid_window`=最近 30 条 section 加权平均；section<2 退化首义；section 新增/归属变更时增量重算。

### 3.3 吸尘器检测
`vacuum_window`=7 天吸引统计，`strong/(strong+mid)<vacuum_ratio(0.20)` → `is_vacuum=true`；挂到 vacuum topic 的 tag 降级 L2。

### 3.4 LLM 退化
`ClusterTags` 只处理 L2（候选 top-K 上留/换/新）+ L3（起新叙事），从全量自由聚类收窄到弱区+兜底子集。

## 4. 数据模型变更
- `board_persistent_topics`：+ `centroid` vector(2560) NULL、+ `is_vacuum` bool default false、+ `vacuum_strong` int、+ `vacuum_mid` int（吸引统计快照）。
- `daily_report_sections`：+ `lane_tier` varchar(16) NULL。
- 迁移：①加列 ②离线构建 centroid（按历史 section embedding，centroid_window=30）③按近 7 天吸引统计初始化 is_vacuum。

## 5. 算法细节
- **分桶**：每个当天 tag 算到该 board 所有 active+candidate topic centroid 的余弦距离（pgvector `<=>`），argmin 决定归属桶。
- **L2 三选一 prompt**：注入 top-K 候选 topic（按距离）+ 各候选近期 section 摘要，要求 LLM 输出 keep/switch/new + target_topic_id。
- **吸尘器统计**：日报生成后增量更新 attracted/strong/mid（当日 tag 对各 topic 的最近邻归属计数）。

## 6. 边界与降级
- topic 无 centroid（新 board / 无 section）：tag 全走 L3。
- embedding 服务失败：tag 走 L3（不阻断，记录告警）。
- L2 候选集为空（board 无 active topic）：所有 L2/L3 tag 走 L3 新建。
- 离线 centroid 构建失败：退化用现有 embedding（首义）。

## 7. 验证计划
- **SQL 复算**：对最近 N 天日报，用新分桶逻辑重算 lane 分布，对比现有 `topic_match_confidence`，量化错锚减少。
- **功能验收**：生成一次日报，确认 ① L1 section 不调 LLM ② L2 section 有 LLM decision ③ 无万能包装标题 ④ 吸尘器 topic 的 tag 进 L2。
- **回归**：历史 section 归属不回刷（lane_tier 为 NULL 视为历史），仅新日报走新流程。

## 8. 本 change 不做
- tag 碎片化去重（33% 近重复，留后续 change）。
- 跨 board 重复聚类（同 tag 多 board 各聚，留后续）。
- 旧 section 回刷（历史归属保持）。

## 9. 风险
- 质心继承历史 section 偏差（万能标题固化）→ 吸尘器检测 + 后续 section-lifecycle 治理兜底。
- L2 占比 ~51%，LLM 调用量仍可观（但输入聚焦，质量提升）。
- centroid 增量更新一致性（并发日报生成）→ 生成后异步重算，读旧值可接受（日报日级，非实时）。
