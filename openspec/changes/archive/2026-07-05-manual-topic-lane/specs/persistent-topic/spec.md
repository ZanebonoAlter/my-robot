## MODIFIED Requirements

### Requirement: PersistentTopic 持久化数据模型

系统 SHALL 维护 `board_persistent_topics` 表，持久化每个 board 内的长期叙事话题。每行 SHALL 包含：`semantic_board_id`（所属板块）、`label`（叙事框架标题）、`description`、`embedding`（与 `semantic_labels.embedding` 同维度）、`status`（candidate/active/archived）、`source`（auto/manual）、`first_seen_date`、`last_seen_date`、`hit_count`、`consecutive_hits`。

`status` SHALL 受 CHECK 约束为 candidate/active/archived 三态。`source` SHALL 受 CHECK 约束为 auto/manual 二态，默认 auto（历史数据与自动涌现的 topic 均为 auto）。表 SHALL 在 `(semantic_board_id, status)` 上建 BTree 索引、在 `embedding` 上建 HNSW 索引。

建表 SHALL 通过显式数据库迁移完成（按开发执行规范 §10），不依赖 gorm AutoMigrate。

#### Scenario: 候选话题创建

- **WHEN** 归属算法判定某 section 无法归属到已有 topic，需新建 PersistentTopic
- **THEN** 系统 SHALL 创建一行，status=candidate，source=auto，first_seen_date=last_seen_date=当天，hit_count=1，consecutive_hits=1

#### Scenario: 话题状态约束

- **WHEN** 尝试写入 status='pending'
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

#### Scenario: 来源约束

- **WHEN** 尝试写入 source='imported'
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

#### Scenario: 历史 topic 默认来源

- **WHEN** 迁移执行后，历史已存在的 topic 行
- **THEN** 其 source SHALL 为 auto（迁移默认值），不报错

### Requirement: DailyReportSection 强制归属 PersistentTopic

`daily_report_sections` SHALL 新增 `persistent_topic_id`（外键到 board_persistent_topics）、`topic_match_distance`、`topic_match_confidence` 三列。`topic_match_confidence` SHALL 取值 anchor_hit / auto_new / unmatched / manual。

归属算法 SHALL 保证每个新生成的 section 归属到恰好 1 个 PersistentTopic：embedding 为空时 confidence=unmatched 且 persistent_topic_id 允许为 NULL（仅此例外）。

`persistent_topic_id` 列 SHALL NOT 加 NOT NULL 约束，以兼容回刷过渡期和历史数据。`topic_match_confidence=manual` 表示该归属由用户手动改写（非算法判定），其 `topic_match_distance` 为该 section embedding 到所属话题聚合锚点的距离。

#### Scenario: 归属命中已有话题

- **WHEN** section embedding 到某 active topic 的距离 ≤ match_threshold（默认 0.30）且 ClusterTags 当轮 LLM 输出的 matched_topic_id 等于该 topic
- **THEN** 系统 SHALL 设置 persistent_topic_id 指向该 topic，topic_match_confidence=anchor_hit

#### Scenario: 双重确认不一致开新候选

- **WHEN** section embedding 最近邻距离 ≤ match_threshold，但 LLM matched_topic_id 指向另一 topic 或为空
- **THEN** 系统 SHALL 新建 candidate topic 归属，topic_match_confidence=auto_new

#### Scenario: embedding 为空标记未匹配

- **WHEN** section embedding 为空
- **THEN** 系统 SHALL 设置 topic_match_confidence=unmatched，persistent_topic_id 保持 NULL

#### Scenario: 手动改写归属

- **GIVEN** section #123 的 persistent_topic_id=8（confidence=anchor_hit）
- **WHEN** 用户在编排态将 section #123 串联进手动新建的 topic #20
- **THEN** 系统 SHALL 将 section #123 的 persistent_topic_id 改写为 20，topic_match_confidence=manual，topic_match_distance=该 section 到 #20 聚合锚点的距离

#### Scenario: LLM matched_topic_id 幻觉降级

- **WHEN** LLM 输出的 matched_topic_id 不在传入的 existingTopics 集合中
- **THEN** 系统 SHALL 将其视为 null，按双重确认规则重新判定

## ADDED Requirements

### Requirement: 手动建持久化话题

系统 SHALL 提供手动创建持久化话题的能力：用户提交 `label` 与一组 `section_ids`，系统在单个事务内创建新 PersistentTopic 并将选中 section 归属改写到该话题。

创建流程 SHALL：
1. 读取选中 section 的 embedding（维度缺失的 section 跳过并返回提示）。
2. 聚合为 mean pooling 平均向量作为新话题 embedding。
3. `CreateTopic`：status=active、source=manual、embedding=聚合向量、first_seen_date=min(选中 section period_date)、last_seen_date=max(...)、hit_count=选中数、consecutive_hits=选中数。
4. 对每个选中 section 调用归属改写（persistent_topic_id=新话题、confidence=manual、distance=该 section 到聚合锚点距离）。
5. 事务末尾触发 `RebuildBoardRelations`（幂等重建该 board 关系，重算 identity 边，保证血统一致）。

手动话题 SHALL 跳过 `upgrade_threshold` 连续命中门禁（自动路径的 candidate→active 门禁对本路径不生效），因用户主权声明且已有选中内容支撑。

手动话题 SHALL NOT 调用任何 LLM（embedding 聚合为纯向量运算）。

#### Scenario: 手动串联建泳道

- **WHEN** 用户提交 label="美伊博弈"、section_ids=[#101,#105,#110]（均属其他话题），三者 embedding 聚合得向量 V
- **THEN** 系统 SHALL 创建 topic（label=美伊博弈, status=active, source=manual, embedding=V），并将 #101/#105/#110 的 persistent_topic_id 改写为该 topic、confidence=manual

#### Scenario: 绕过升级门禁

- **GIVEN** upgrade_threshold=3
- **WHEN** 手动创建话题（选中 5 条 section）
- **THEN** 新话题 SHALL 直接 active，SHALL NOT 先进 candidate 状态，SHALL NOT 等待连续命中天数

#### Scenario: 选中 section 含无 embedding

- **GIVEN** 选中 section #200 的 embedding 为空
- **WHEN** 执行手动建泳道
- **THEN** 系统 SHALL 跳过 #200（不计入聚合、不改写其归属），SHALL 在响应中提示"1 条 section 因无向量被跳过"，其余正常建泳道

#### Scenario: 关系重建保证血统一致

- **WHEN** 手动建泳道事务提交
- **THEN** 系统 SHALL 执行 RebuildBoardRelations，新话题下相邻天的 section SHALL 写入 identity 关系边，被移出原话题的 section 与原话题的 identity 边 SHALL 不再保留

#### Scenario: 手动建泳道非致命于日报生成

- **WHEN** 手动建泳道事务中 RebuildBoardRelations 失败
- **THEN** 系统 SHALL 回滚整个事务（含话题创建与归属改写），SHALL NOT 留下半成品状态

### Requirement: 手动话题参与自动归属

`source=manual` 且 `status=active` 的话题 SHALL 与 `source=auto` 的 active 话题在后续日报生成中一视同仁地参与 embedding AND-gate 自动归属：`ListAnchorableTopicsByBoard` SHALL 包含手动 active 话题，其聚合 embedding 作为最近邻锚点。

手动话题 SHALL NOT 因 source=manual 而被排除出归属候选集，也 SHALL NOT 获得任何特殊阈值放宽/收紧（与自动 active 话题共用同一 match_threshold）。

#### Scenario: 手动话题次期被自动命中

- **GIVEN** 手动建 topic #20（source=manual, active, embedding=V），次日新 section #300 embedding 到 V 距离=0.22 ≤ match_threshold
- **WHEN** 日报生成执行 planTopicAssignments
- **THEN** #300 SHALL 被 AND-gate 判定归属到 #20（若 LLM 也认同），confidence=anchor_hit

#### Scenario: 手动话题不享特殊阈值

- **GIVEN** 手动 topic #20，match_threshold=0.30
- **WHEN** 某 section 到 #20 距离=0.35
- **THEN** 系统 SHALL 按统一阈值判定不归属（SHALL NOT 因 #20 是手动而放宽）

### Requirement: 回刷重建持久化话题（单成员不建）

`BackfillPersistentTopics` 为历史未归属 section 重建持久化话题时，SHALL 采用 complete-link 聚类（一个 section 仅当与某 cluster 的**每个**成员距离 ≤ ClusterThreshold 时才加入，否则 seed 新 cluster），并对每个达到规模门槛的 cluster 直接创建 ACTIVE 话题。

**规模门槛**：只有成员数 ≥ 2 的 cluster SHALL 建话题。成员数为 1 的孤立 cluster（一条 section 跟谁都不挨着）SHALL NOT 建话题，其 section SHALL 保持 `persistent_topic_id IS NULL`（unassigned），留给日报生成路径按连续命中观察期（candidate→active）自然涌现。理由：单次出现的孤立 section 不构成「持久叙事」证据，直接建 ACTIVE 话题会绕过连续多天观察期、产生噪音泳道。

达到门槛的 cluster 创建话题时，`status=active`、`source=auto`、`hit_count=成员数`、`consecutive_hits=成员数`（历史成员即历史证据）。

回刷 SHALL NOT 调用任何 LLM（纯聚类 + 向量运算）。回刷事务末尾 SHALL 触发 `RebuildBoardRelations`（失败仅告警、不回滚，与既有行为一致）。

#### Scenario: 单成员 cluster 不建话题

- **GIVEN** board 有两条正交 section（彼此距离 > ClusterThreshold），各自无法与对方聚成 cluster
- **WHEN** 执行 BackfillPersistentTopics
- **THEN** 系统 SHALL 创建 0 个话题，两条 section SHALL 保持 persistent_topic_id IS NULL

#### Scenario: 多成员 cluster 直接建 active

- **GIVEN** board 有 3 条相互距离 ≤ ClusterThreshold 的 section（同一叙事）
- **WHEN** 执行 BackfillPersistentTopics
- **THEN** 系统 SHALL 创建 1 个 status=active、source=auto、hit_count=3、consecutive_hits=3 的话题，3 条 section 归属到该话题

#### Scenario: 孤立 section 留给日报路径涌现

- **GIVEN** 回刷后 section #9 保持 unassigned（孤立单成员）
- **WHEN** 后续日报生成执行 planTopicAssignments 且 #9 仍无法归属已有话题
- **THEN** 系统 SHALL 按日报路径为 #9 开 candidate（hit_count=1），需累计命中 upgrade_threshold 次才能转 active

### Requirement: 候选激活门禁（累计命中口径）

candidate→active 的激活门禁 SHALL 基于**累计命中次数**（`hit_count`），而非连续命中天数（`consecutive_hits`）。`hit_count` 每次归属命中 +1、只增不减；`consecutive_hits` 仍记录"最近连续命中期数"（miss 时 reset 0）但不再作为门禁。

凡"候选是否达标"的判定 SHALL 统一用 `hit_count >= upgrade_threshold`：
- `can_activate`（话题列表响应）= `status=candidate && hit_count >= upgrade_threshold`。
- `FilterVisibleTopics`（管理 UI 可见性）：候选仅当 `hit_count >= upgrade_threshold` 时可见，其余 observing 候选隐藏。
- `UpdateTopic` candidate→active 校验：`hit_count < upgrade_threshold` 拒绝并提示"需累计命中至少 N 次"。
- `PruneUnderqualifiedCandidates`（一次性清理迁移）：删除 `hit_count < upgrade_threshold` 的候选。

理由：话题天然有间隔（今日涌现、隔日中断、后日重现），严格"连续每期命中"在真实数据中极难达到（实测最多连续 1 期），导致候选永远无法达标转正。改用累计命中后，断断续续出现的话题也能在累计够次数后转正。

`PersistentTopicBrief`（timeline 节点的话题摘要）SHALL 含 `hit_count` 字段，供前端展示"累计命中 N 次"。

#### Scenario: 累计命中达标可激活

- **GIVEN** upgrade_threshold=3，候选 #5 历史命中过 3 期（hit_count=3）但最近一期 miss（consecutive_hits=0）
- **THEN** #5 SHALL can_activate=true（累计达标），SHALL 可被人工确认转 active

#### Scenario: 连续命中高但累计不足不可激活

- **GIVEN** 候选 #6 consecutive_hits=2（最近连续 2 期）但 hit_count=2（< 3）
- **THEN** #6 SHALL can_activate=false（累计不足），SHALL NOT 可激活

#### Scenario: 清理迁移按累计口径

- **GIVEN** 一次性清理迁移执行，候选 #7 hit_count=1（< 3）但 consecutive_hits=1
- **THEN** #7 SHALL 被 PruneUnderqualifiedCandidates 删除（累计不足）
