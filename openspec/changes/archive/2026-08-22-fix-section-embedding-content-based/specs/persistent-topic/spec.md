## MODIFIED Requirements

### Requirement: PersistentTopic 质心表示

PersistentTopic 的 embedding 匹配锚点 SHALL 为「历史 section embedding 质心」，而非首条 section 标题继承的首义向量。质心 SHALL 按 `centroid_window`（默认最近 30 条 section，ai_settings 可配）加权计算，section 新增或归属变更时增量更新。质心 SHALL 物化存储于 `board_persistent_topics.centroid` 列（与 embedding 同维度），首次部署按历史 section embedding 离线构建一次。

section embedding SHALL 基于 section 实际聚合内容生成（见 section-content-embedding 能力），因此质心 SHALL 随话题下 section 的实际内容漂移：当被归因的 section 内容偏离话题标题语义时，质心 SHALL 被拉向实际内容方向，使后续与该内容无关的 tag 匹配距离增大，脱离 L1/L2 匹配带。

新建 candidate 话题的首义 embedding（`board_persistent_topics.embedding`）SHALL 取该 section 的内容化 embedding（L3 归因时 section embedding 即内容向量），不再继承标题文本向量。

历史 section 数 < 2 的 topic，质心退化为首义向量（等价于现有 embedding 列语义）。

#### Scenario: 质心按近期 section 计算

- **GIVEN** topic「Claude Code 工具链崛起」有历史 section 40 条
- **WHEN** 系统计算其质心
- **THEN** 系统 SHALL 取最近 30 条 section embedding 加权平均作为 centroid，centroid_window 之外的 section 不参与

#### Scenario: 质心随内容漂移

- **GIVEN** topic「美军机从以色列境内基地起飞对伊朗实施打击」质心原锚定标题语义，连续两日被归因了「阿联酋与伊朗贸易」相关 section（内容 embedding 与标题语义距离约 0.24）
- **WHEN** 质心重算
- **THEN** 新质心 SHALL 向阿联酋贸易内容方向移动，后续「美军机打击伊朗」类 tag 到该质心的距离 SHALL 大于原值（标题回声闭环被打破）

#### Scenario: section 不足退化首义向量

- **GIVEN** topic 只有 1 条历史 section
- **WHEN** 系统计算其质心
- **THEN** centroid SHALL 等于该 section 的内容 embedding（首义向量语义）

#### Scenario: 质心增量更新

- **GIVEN** 新 section 归属 topic「X」
- **WHEN** section 持久化
- **THEN** topic X 的 centroid SHALL 增量重算（纳入新 section，窗口溢出的旧 section 淘汰）
