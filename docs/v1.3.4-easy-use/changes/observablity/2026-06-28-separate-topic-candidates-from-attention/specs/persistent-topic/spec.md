## MODIFIED Requirements

### Requirement: ClusterTags 注入历史叙事框架

`ClusterTags` SHALL 在调用 LLM 前，查询该 board 的可锚定 PersistentTopic 集合。该集合 SHALL 包含所有 active，以及 `last_seen_date` 位于 `persistent_topic_candidate_decay_window` 内的 candidate；candidate SHALL 按 `last_seen_date DESC, hit_count DESC, id ASC` 排序并最多保留 `persistent_topic_candidate_prompt_limit` 条。

`candidate_decay_window` 在此处仅用于聚类 prompt 注入的卫生过滤（避免向 LLM 注入长期未复现的陈旧候选），**不**触发任何状态变更或归档。

系统 SHALL 将筛选后的 label 列表注入 system prompt，指示 LLM 优先将标签归入已有框架、仅当属于新叙事时开新组。ClusterTags 注入与 section 双重确认归属 SHALL 使用同一个可锚定集合。

LLM 输出 schema SHALL 为每个 group 增加 `matched_topic_id` 字段（可为 null）。

`ClusterTags` SHALL 校验返回的 matched_topic_id 合法性（必须存在于传入集合），非法值降级为 null。

#### Scenario: 聚类优先复用已有框架

- **GIVEN** board 有 active topic “AI 编程工具平台化竞争”，当天标签含“开发者 Agent 平台化”
- **WHEN** 执行 ClusterTags
- **THEN** LLM prompt SHALL 包含该 topic，输出 group 的 matched_topic_id 指向它

#### Scenario: 注入有效候选与全部正式话题

- **GIVEN** board 有 3 个 active、20 个仍在观察窗口内的 candidate，candidate_prompt_limit=12
- **WHEN** 构建可锚定话题集合
- **THEN** 集合 SHALL 包含全部 3 个 active 和排序后的前 12 个 candidate

#### Scenario: 陈旧候选不再注入 prompt

- **GIVEN** candidate.last_seen_date 距报告日期超过 candidate_decay_window
- **WHEN** 构建可锚定话题集合
- **THEN** 该 candidate SHALL NOT 出现在 ClusterTags prompt 或双重确认归属集合中
- **AND** 该 candidate 的 status SHALL NOT 因此改变（窗口过滤仅影响注入，不归档）

#### Scenario: 聚类与归属集合一致

- **WHEN** 某 candidate 因时间窗口或数量上限未进入 ClusterTags prompt
- **THEN** 双重确认归属 SHALL NOT 接受该 candidate 的 id

### Requirement: PersistentTopic 生命周期（全人工归档）

系统 SHALL 在每日报生成并归属后，更新 topic 的命中计数，但 SHALL NOT 自动变更任何 topic 的 status。candidate 与 active 转为 archived SHALL 完全由用户在话题管理界面手动操作。

状态机更新规则：

- 当天有 section 归属到某 topic：`consecutive_hits += 1`，`hit_count += 1`，`last_seen_date = 当天`；status 不变。
- 当天无 section 归属到某 topic：`consecutive_hits` 归 0；status 不变，`last_seen_date` 不变。
- 任何 status→archived 的转换 SHALL 仅由显式的用户操作触发；系统 SHALL NOT 因时间窗口、未命中或任何后台规则自动归档 candidate 或 active。

candidate 是否"获得人工确认资格"由 `consecutive_hits ≥ upgrade_threshold` 判定（见"候选展示门槛"Requirement）。获得资格仅意味着可被用户确认，SHALL NOT 自动转为 active；只有用户的显式确认操作 SHALL 将 candidate 转为 active。

`upgrade_threshold` SHALL 可通过 ai_settings 配置。

#### Scenario: 命中累计但状态不变

- **GIVEN** topic #12 status=candidate，consecutive_hits=2
- **WHEN** 第 3 天有 section 归属到 #12
- **THEN** #12 SHALL 保持 candidate，consecutive_hits=3，hit_count+1，last_seen_date=当天

#### Scenario: 未命中仅清零不归档

- **GIVEN** topic #12 status=candidate、consecutive_hits=2
- **WHEN** 当天无 section 归属到 #12
- **THEN** #12 consecutive_hits SHALL 归 0，status SHALL 保持 candidate，last_seen_date SHALL 不变

#### Scenario: 长期未命中也不自动归档

- **GIVEN** topic #12 status=candidate，last_seen_date 距今已超过任意天数
- **WHEN** 执行状态机更新
- **THEN** #12 status SHALL 保持 candidate（SHALL NOT 自动转 archived）

#### Scenario: 未达多天门禁时拒绝确认

- **GIVEN** topic #12 status=candidate，consecutive_hits=2，upgrade_threshold=3
- **WHEN** 用户尝试将 #12 更新为 active
- **THEN** 系统 SHALL 拒绝，并返回连续出现天数不足的错误

#### Scenario: 人工确认后进入持久泳道

- **GIVEN** topic #12 status=candidate，consecutive_hits≥upgrade_threshold
- **WHEN** 用户确认启用 #12
- **THEN** #12 SHALL 转为 active；只有 active topic SHALL 形成持久话题泳道

#### Scenario: 归档只能人工触发

- **GIVEN** topic #18 status=active，last_seen_date 距今已超过任意天数
- **WHEN** 执行状态机更新（无用户操作）
- **THEN** #18 status SHALL 保持 active
- **AND** 只有用户在话题管理执行归档操作 SHALL 使其转为 archived

### Requirement: 候选展示门槛（observing → 可见 candidate）

系统 SHALL 对话题管理 UI 隐藏尚未达到确认资格的 candidate。只有 `status=candidate AND consecutive_hits ≥ upgrade_threshold` 的 candidate SHALL 出现在话题管理端点（`GET /api/semantic-boards/:id/topics`）的响应中；`consecutive_hits < upgrade_threshold` 的 candidate（"observing"）SHALL NOT 出现。

observing candidate SHALL 仍持久化于数据库并参与可锚定话题集合（保证命中能跨天累积，最终达到 upgrade_threshold）；它们只是对用户不可见。当 consecutive_hits 达到 upgrade_threshold 时，candidate SHALL 自动在话题管理 UI 中可见（无需任何额外操作）。

active 与 archived topic SHALL 始终在话题管理 UI 可见，不受此门槛约束。

#### Scenario: 未达门槛的候选对用户不可见

- **GIVEN** board 有 1 个 active、1 个 candidate(consecutive_hits=1)、1 个 candidate(consecutive_hits=3)，upgrade_threshold=3
- **WHEN** 请求话题管理列表
- **THEN** 响应 SHALL 包含 active 和 consecutive_hits=3 的 candidate
- **AND** SHALL NOT 包含 consecutive_hits=1 的 candidate

#### Scenario: 累计达门槛后自动可见

- **GIVEN** observing candidate #20 consecutive_hits=2
- **WHEN** 次日有 section 归属到 #20，consecutive_hits 变为 3
- **THEN** 下次请求话题管理列表 SHALL 包含 #20（无需用户操作）

#### Scenario: observing 仍参与锚点归属

- **GIVEN** observing candidate #20 consecutive_hits=1，仍在 candidate_decay_window 内
- **WHEN** 构建 ClusterTags 可锚定集合
- **THEN** #20 SHALL 出现在可锚定集合中（用于跨天命中累积），但对用户管理 UI 不可见

### Requirement: 配置项

系统 SHALL 在 ai_settings 提供以下配置项（含默认值）：

- `persistent_topic_match_threshold`：0.30（section 命中 topic 的 embedding distance 上限）
- `persistent_topic_upgrade_threshold`：3（candidate 获得人工确认资格、并在话题管理 UI 可见的连续命中天数门槛）
- `persistent_topic_candidate_decay_window`：7（candidate 仍被注入聚类 prompt 的观察窗口；仅用于 prompt 卫生过滤，不触发归档）
- `persistent_topic_candidate_prompt_limit`：20（单个 board 每次进入聚类与归属锚点集合的 candidate 上限）
- `persistent_topic_cluster_threshold`：0.30（回刷时历史 section 聚类成 topic 的阈值）

> 说明：本 change 取消了 candidate/active 的自动归档，因此不再使用独立的 active `decay_window`；归档完全由人工触发。

#### Scenario: 读取默认配置

- **WHEN** ai_settings 未设置上述 key
- **THEN** 系统 SHALL 使用默认值

#### Scenario: 自定义升级与展示门槛

- **WHEN** ai_settings 设置 persistent_topic_upgrade_threshold=5
- **THEN** candidate 连续命中 5 天才获得人工确认资格并在话题管理 UI 可见

#### Scenario: 自定义候选观察窗口和上限

- **WHEN** ai_settings 设置 candidate_decay_window=14、candidate_prompt_limit=20
- **THEN** 系统 SHALL 仅向聚类 prompt 注入 14 天窗口内的 candidate，并最多选择 20 条进入聚类与归属锚点集合

## ADDED Requirements

### Requirement: 一次性清理未达门槛的历史 candidate

系统 SHALL 提供幂等的一次性迁移，删除所有 `status=candidate AND consecutive_hits < upgrade_threshold` 的历史 candidate。删除 SHALL 采用与 `DeleteTopic` 一致的语义：先将被引用 section 的 `persistent_topic_id`、`topic_match_distance`、`topic_match_confidence`、`topic_status_at_report` 置 NULL（section 内容保留、仍可正常渲染为"其他动态"中的独立节点），再硬删 candidate 行，最后重建该 board 的 relations 以清除指向已删 topic 的 identity/similarity 边。

迁移 SHALL 可重复执行（幂等）：第二次执行时已无满足条件的 candidate，SHALL 为 no-op。迁移 SHALL NOT 删除 active、archived 或 `consecutive_hits ≥ upgrade_threshold` 的 candidate。

#### Scenario: 清理未达门槛候选并保留 section

- **GIVEN** board 有 218 条 consecutive_hits<3 的 candidate，其中 109 条被 section 引用
- **WHEN** 执行一次性清理迁移
- **THEN** 这 218 条 candidate SHALL 被硬删
- **AND** 被引用的 109 条 section 的 persistent_topic_id 与 topic_status_at_report SHALL 置 NULL
- **AND** 这些 section SHALL 仍在日报与时间线正常渲染

#### Scenario: 达门槛候选与 active/archived 不受影响

- **GIVEN** board 另有 4 条 consecutive_hits≥3 的 candidate、若干 active 与 archived
- **WHEN** 执行一次性清理迁移
- **THEN** 这些 topic SHALL 全部保留

#### Scenario: 迁移幂等

- **GIVEN** 清理迁移已执行过一次
- **WHEN** 再次执行
- **THEN** SHALL 不报错、不改动任何数据

### Requirement: PersistentTopic 候选态不得直接驱动用户注意力

PersistentTopic 的 candidate 状态 SHALL 仅表示系统正在观察一个可能复现的叙事身份。candidate SHALL NOT 被解释为突发、重要、已关注或值得提醒；任何用户注意力入口必须由 active 人工确认、topic watch 命中或独立的显著性规则驱动。

#### Scenario: 首次创建候选不产生突发入口

- **WHEN** 归属算法为当天 section 自动创建 candidate PersistentTopic
- **THEN** 系统 SHALL 保留该 section 的后台话题归属
- **AND** 日报 SHALL NOT 因 candidate 状态创建"突发的新话题"分区

#### Scenario: 未达门槛候选不进入话题管理

- **WHEN** 用户进入 PersistentTopic 管理界面，且存在 consecutive_hits < upgrade_threshold 的 candidate
- **THEN** 系统 SHALL NOT 展示这些 observing candidate
- **AND** 仅展示 active、archived 与 consecutive_hits ≥ upgrade_threshold 的 candidate
- **AND** 该管理状态 SHALL NOT 改变日报阅读层的注意力排序
