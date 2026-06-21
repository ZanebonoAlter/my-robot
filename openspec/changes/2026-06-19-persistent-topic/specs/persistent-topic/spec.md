## Purpose

PersistentTopic（持久叙事话题）：在 SemanticBoard（板块）和 DailyReportSection（当天聚类）之间引入持久叙事话题层。强制 section 1:N 归属、自动升级生命周期、关系叠加身份边，解决板块下话题因命名漂移而散乱、每日聚类随机的根因问题。

## Requirements

### Requirement: PersistentTopic 持久化数据模型

系统 SHALL 维护 `board_persistent_topics` 表，持久化每个 board 内的长期叙事话题。每行 SHALL 包含：`semantic_board_id`（所属板块）、`label`（叙事框架标题）、`description`、`embedding`（与 `semantic_labels.embedding` 同维度）、`status`（candidate/active/archived）、`first_seen_date`、`last_seen_date`、`hit_count`、`consecutive_hits`。

`status` SHALL 受 CHECK 约束为 candidate/active/archived 三态。表 SHALL 在 `(semantic_board_id, status)` 上建 BTree 索引、在 `embedding` 上建 HNSW 索引。

建表 SHALL 通过显式数据库迁移完成（按开发执行规范 §10），不依赖 gorm AutoMigrate。

#### Scenario: 候选话题创建

- **WHEN** 归属算法判定某 section 无法归属到已有 topic，需新建 PersistentTopic
- **THEN** 系统 SHALL 创建一行，status=candidate，first_seen_date=last_seen_date=当天，hit_count=1，consecutive_hits=1

#### Scenario: 话题状态约束

- **WHEN** 尝试写入 status='pending'
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

### Requirement: DailyReportSection 强制归属 PersistentTopic

`daily_report_sections` SHALL 新增 `persistent_topic_id`（外键到 board_persistent_topics）、`topic_match_distance`、`topic_match_confidence` 三列。`topic_match_confidence` SHALL 取值 anchor_hit / auto_new / unmatched。

归属算法 SHALL 保证每个新生成的 section 归属到恰好 1 个 PersistentTopic：embedding 为空时 confidence=unmatched 且 persistent_topic_id 允许为 NULL（仅此例外）。

`persistent_topic_id` 列 SHALL NOT 加 NOT NULL 约束，以兼容回刷过渡期和历史数据。

#### Scenario: 归属命中已有话题

- **WHEN** section embedding 到某 active topic 的距离 ≤ match_threshold（默认 0.30）且 ClusterTags 当轮 LLM 输出的 matched_topic_id 等于该 topic
- **THEN** 系统 SHALL 设置 persistent_topic_id 指向该 topic，topic_match_confidence=anchor_hit

#### Scenario: 双重确认不一致开新候选

- **WHEN** section embedding 最近邻距离 ≤ match_threshold，但 LLM matched_topic_id 指向另一 topic 或为空
- **THEN** 系统 SHALL 新建 candidate topic 归属，topic_match_confidence=auto_new

#### Scenario: embedding 为空标记未匹配

- **WHEN** section embedding 为空
- **THEN** 系统 SHALL 设置 topic_match_confidence=unmatched，persistent_topic_id 保持 NULL

#### Scenario: LLM matched_topic_id 幻觉降级

- **WHEN** LLM 输出的 matched_topic_id 不在传入的 existingTopics 集合中
- **THEN** 系统 SHALL 将其视为 null，按双重确认规则重新判定

### Requirement: ClusterTags 注入历史叙事框架

`ClusterTags` SHALL 在调用 LLM 前，查询该 board 下所有 active 与 candidate 状态的 PersistentTopic，将 label 列表注入 system prompt，指示 LLM 优先将标签归入已有框架、仅当属于新叙事时开新组。

LLM 输出 schema SHALL 为每个 group 增加 `matched_topic_id` 字段（可为 null）。

`ClusterTags` SHALL 校验返回的 matched_topic_id 合法性（必须存在于传入集合），非法值降级为 null。

#### Scenario: 聚类优先复用已有框架

- **GIVEN** board 有 active topic "AI 编程工具平台化竞争"，当天标签含 "开发者 Agent 平台化"
- **WHEN** 执行 ClusterTags
- **THEN** LLM prompt SHALL 包含该 topic，输出 group 的 matched_topic_id 指向它

#### Scenario: 注入候选与正式两类框架

- **WHEN** 注入 existingTopics
- **THEN** 列表 SHALL 同时包含 active 和 candidate 状态的话题，各自标注状态

### Requirement: PersistentTopic 自动升级生命周期

系统 SHALL 在每日报生成并归属后，执行 topic 状态机更新：

- 当天有 section 归属到某 topic：consecutive_hits += 1，hit_count += 1，last_seen_date = 当天；若 status=candidate 且 consecutive_hits ≥ upgrade_threshold（默认 3），SHALL 自动转为 active。
- 当天无 section 归属：consecutive_hits 归 0；若 status=active 且 today - last_seen_date > decay_window（默认 30 天），SHALL 自动转为 archived。

upgrade_threshold、decay_window SHALL 可通过 ai_settings 配置。

#### Scenario: 候选连续命中升级

- **GIVEN** topic #12 status=candidate，consecutive_hits=2
- **WHEN** 第 3 天有 section 归属到 #12
- **THEN** #12 SHALL 转为 active，consecutive_hits=3，hit_count+1

#### Scenario: 连续命中中断重置

- **GIVEN** topic #12 consecutive_hits=2
- **WHEN** 第 3 天无 section 归属到 #12
- **THEN** #12 consecutive_hits SHALL 归 0，status 保持 candidate

#### Scenario: 正式话题超期归档

- **GIVEN** topic #18 status=active，last_seen_date=2026-05-15，当前日期 2026-06-19（间隔 35 天 > decay_window 30）
- **WHEN** 执行状态机更新
- **THEN** #18 SHALL 转为 archived

#### Scenario: 正式话题窗口内保留

- **GIVEN** topic #18 status=active，last_seen_date=2026-06-05，当前日期 2026-06-19（间隔 14 天 < decay_window 30）
- **WHEN** 执行状态机更新
- **THEN** #18 status SHALL 保持 active

### Requirement: Section 关系叠加身份边

`daily_report_section_relations` SHALL 新增 `relation_type` 列（identity/similarity，默认 similarity）。匈牙利二分匹配产出的关系 SHALL 标记为 similarity（算法实现不动）。

关系重建 SHALL 额外写入身份边：对每个 PersistentTopic 下相邻天（按 period_date）的两个 section，SHALL 写入或覆盖一条 relation_type=identity 的关系。身份边 SHALL NOT 受匈牙利 penalty（0.28）限制——即使两 section embedding distance > penalty，只要归属同一 topic 即写边。

当同一 (from_id, to_id) 同时存在 similarity 与 identity 关系时，SHALL 以 identity 为准（覆盖）。

#### Scenario: 命名漂移不断链

- **GIVEN** Day1 section 归属 topic #12，Day2 section 归属 topic #12，但两者 embedding distance=0.32 > penalty 0.28
- **WHEN** 关系重建
- **THEN** 系统 SHALL 写入 (Day1, Day2, distance=0.32, relation_type=identity)，DAG 上两节点连通

#### Scenario: 不同话题不写身份边

- **GIVEN** Day1 section 归属 topic #12，Day2 section 归属 topic #18
- **WHEN** 关系重建
- **THEN** 系统 SHALL NOT 写入 identity 关系（仅由匈牙利 similarity 边决定是否连通）

### Requirement: 话题生命线 API

`GET /api/daily-reports/topics/:id/lifeline` SHALL 返回该 PersistentTopic 下的全部 section（不限天数，按 `persistent_topic_id` 聚合）及内部关系。

响应 SHALL 包含 topic 元信息（label/status/first_seen_date/last_seen_date/hit_count）和 sections 列表（含 period_date/cluster_label/status/article_count/thread_count）。relations SHALL 只包含两端 section 都属于该 topic 的关系。

相比 `getSectionLifecycle`，本 API SHALL NOT 依赖 embedding 连通性，直接按 persistent_topic_id 聚合。

#### Scenario: 查询话题完整生命线

- **WHEN** topic #12 下有 4 天的 section（#40/#50/#60/#70）
- **THEN** 响应 SHALL 返回这 4 个 section 及其内部 identity/similarity 关系

#### Scenario: 话题跨命名漂移聚合

- **GIVEN** topic #12 下 section 标签依次为 "AI 编程竞争"/"开发者生态重构"/"AI 工具内卷"
- **WHEN** 查询 lifeline
- **THEN** 响应 SHALL 返回全部 3 个 section（即使 embedding 两两 distance 部分超过 penalty）

### Requirement: 现有 API 向后兼容扩展

`getBoardSectionTimeline` 和 `getSectionLifecycle` 响应 SHALL 增加可选字段：每个 section 嵌套 `persistent_topic`（id/label/status/color）、每条 relation 增加 `relation_type`。新字段 SHALL 全部 optional，老前端忽略不报错。

`persistent_topic.color` SHALL 由后端按 topic_id 哈希分配稳定色，避免前端刷新跳色。

#### Scenario: 老客户端兼容

- **WHEN** 老前端调用 getBoardSectionTimeline，不读取新字段
- **THEN** 系统 SHALL 正常返回响应，前端 SHALL 正常渲染（忽略新字段）

#### Scenario: 主题色稳定

- **WHEN** 同一 topic #12 连续两次查询
- **THEN** 两次返回的 persistent_topic.color SHALL 一致

### Requirement: 历史数据回刷

系统 SHALL 提供 `BackfillPersistentTopics(boardID)`，从该 board 历史 section 用 average_link 聚类反推 PersistentTopic（默认 cluster_threshold=0.30）。回刷产出的 topic SHALL 直接给 active 状态（因已有历史命中支撑）。

回刷 SHALL 回填所有历史 section 的 persistent_topic_id。

回刷 SHALL 在 SaveReport 之后、RebuildBoardRelations 之前执行（关系重建需要 topic_id 写身份边）。

#### Scenario: 回刷历史 section 归属

- **GIVEN** board 有 30 天历史 section，无任何 PersistentTopic
- **WHEN** 执行 BackfillPersistentTopics
- **THEN** 系统 SHALL 产出 active topic 集合（目标单 board 5-15 个），全部历史 section 的 persistent_topic_id 被回填

#### Scenario: 回刷后关系含身份边

- **WHEN** 回刷后执行 RebuildBoardRelations
- **THEN** 同 topic 相邻天 section SHALL 写入 identity 关系

### Requirement: 配置项

系统 SHALL 在 ai_settings 新增以下配置项（含默认值）：
- `persistent_topic_match_threshold`：0.30（section 命中 topic 的 embedding distance 上限）
- `persistent_topic_upgrade_threshold`：3（candidate 连续命中天数转 active）
- `persistent_topic_decay_window`：30（active 无命中转 archived 的天数）
- `persistent_topic_cluster_threshold`：0.30（回刷时历史 section 聚类成 topic 的阈值）

#### Scenario: 读取默认配置

- **WHEN** ai_settings 未设置上述 key
- **THEN** 系统 SHALL 使用默认值

#### Scenario: 自定义升级阈值

- **WHEN** ai_settings 设置 persistent_topic_upgrade_threshold=5
- **THEN** candidate 连续命中 5 天才转 active
