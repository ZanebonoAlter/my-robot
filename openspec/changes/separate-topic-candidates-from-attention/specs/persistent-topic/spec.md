## MODIFIED Requirements

### Requirement: ClusterTags 注入历史叙事框架

`ClusterTags` SHALL 在调用 LLM 前，查询该 board 的可锚定 PersistentTopic 集合。该集合 SHALL 包含所有 active，以及 `last_seen_date` 位于 `persistent_topic_candidate_decay_window` 内的 candidate；candidate SHALL 按 `last_seen_date DESC, hit_count DESC, id ASC` 排序并最多保留 `persistent_topic_candidate_prompt_limit` 条。

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

#### Scenario: 陈旧候选不再注入

- **GIVEN** candidate.last_seen_date 距报告日期超过 candidate_decay_window
- **WHEN** 构建可锚定话题集合
- **THEN** 该 candidate SHALL NOT 出现在 ClusterTags prompt 或双重确认归属集合中

#### Scenario: 聚类与归属集合一致

- **WHEN** 某 candidate 因时间窗口或数量上限未进入 ClusterTags prompt
- **THEN** 双重确认归属 SHALL NOT 接受该 candidate 的 id

### Requirement: PersistentTopic 自动升级生命周期

系统 SHALL 在每日报生成并归属后，执行 topic 状态机更新：

- 当天有 section 归属到某 topic：consecutive_hits += 1，hit_count += 1，last_seen_date = 当天；若 status=candidate 且 consecutive_hits ≥ upgrade_threshold（默认 3），SHALL 获得人工确认资格，但 SHALL NOT 自动转为 active。
- 当天无 section 归属：consecutive_hits 归 0；若 status=candidate 且 today - last_seen_date > candidate_decay_window（默认 7 天），SHALL 自动转为 archived；若 status=active 且 today - last_seen_date > decay_window（默认 30 天），SHALL 自动转为 archived。

upgrade_threshold、candidate_decay_window、decay_window SHALL 可通过 ai_settings 配置。

#### Scenario: 候选连续命中后等待人工确认

- **GIVEN** topic #12 status=candidate，consecutive_hits=2
- **WHEN** 第 3 天有 section 归属到 #12
- **THEN** #12 SHALL 保持 candidate，consecutive_hits=3，hit_count+1，并允许用户确认启用

#### Scenario: 未达多天门禁时拒绝确认

- **GIVEN** topic #12 status=candidate，consecutive_hits=2，upgrade_threshold=3
- **WHEN** 用户尝试将 #12 更新为 active
- **THEN** 系统 SHALL 拒绝，并返回连续出现天数不足的错误

#### Scenario: 人工确认后进入持久泳道

- **GIVEN** topic #12 status=candidate，consecutive_hits=3
- **WHEN** 用户确认启用 #12
- **THEN** #12 SHALL 转为 active；只有 active topic SHALL 形成持久话题泳道

#### Scenario: 连续命中中断但仍在观察窗口

- **GIVEN** topic #12 status=candidate、consecutive_hits=2，距 last_seen_date 未超过 candidate_decay_window
- **WHEN** 当天无 section 归属到 #12
- **THEN** #12 consecutive_hits SHALL 归 0，status SHALL 保持 candidate

#### Scenario: 候选超出观察窗口自动归档

- **GIVEN** topic #12 status=candidate，last_seen_date=2026-06-10，当前日期=2026-06-18，candidate_decay_window=7
- **WHEN** 执行状态机更新
- **THEN** #12 SHALL 转为 archived

#### Scenario: 正式话题超期归档

- **GIVEN** topic #18 status=active，last_seen_date=2026-05-15，当前日期 2026-06-19（间隔 35 天 > decay_window 30）
- **WHEN** 执行状态机更新
- **THEN** #18 SHALL 转为 archived

#### Scenario: 正式话题窗口内保留

- **GIVEN** topic #18 status=active，last_seen_date=2026-06-05，当前日期 2026-06-19（间隔 14 天 < decay_window 30）
- **WHEN** 执行状态机更新
- **THEN** #18 status SHALL 保持 active

### Requirement: 配置项

系统 SHALL 在 ai_settings 提供以下配置项（含默认值）：

- `persistent_topic_match_threshold`：0.30（section 命中 topic 的 embedding distance 上限）
- `persistent_topic_upgrade_threshold`：3（candidate 获得人工确认资格的连续命中天数）
- `persistent_topic_decay_window`：30（active 无命中转 archived 的天数）
- `persistent_topic_candidate_decay_window`：7（candidate 无命中转 archived 的天数）
- `persistent_topic_candidate_prompt_limit`：20（单个 board 每次进入聚类与归属锚点集合的 candidate 上限）
- `persistent_topic_cluster_threshold`：0.30（回刷时历史 section 聚类成 topic 的阈值）

#### Scenario: 读取默认配置

- **WHEN** ai_settings 未设置上述 key
- **THEN** 系统 SHALL 使用默认值

#### Scenario: 自定义升级阈值

- **WHEN** ai_settings 设置 persistent_topic_upgrade_threshold=5
- **THEN** candidate 连续命中 5 天才获得人工确认资格

#### Scenario: 自定义候选观察窗口和上限

- **WHEN** ai_settings 设置 candidate_decay_window=14、candidate_prompt_limit=20
- **THEN** 系统 SHALL 保留 14 天窗口内的 candidate，并最多选择 20 条进入聚类与归属锚点集合

## ADDED Requirements

### Requirement: PersistentTopic 候选态不得直接驱动用户注意力

PersistentTopic 的 candidate 状态 SHALL 仅表示系统正在观察一个可能复现的叙事身份。candidate SHALL NOT 被解释为突发、重要、已关注或值得提醒；任何用户注意力入口必须由 active 人工确认、topic watch 命中或独立的显著性规则驱动。

#### Scenario: 首次创建候选不产生突发入口

- **WHEN** 归属算法为当天 section 自动创建 candidate PersistentTopic
- **THEN** 系统 SHALL 保留该 section 的后台话题归属
- **AND** 日报 SHALL NOT 因 candidate 状态创建“突发的新话题”分区

#### Scenario: 候选管理能力保持可用

- **WHEN** 用户进入 PersistentTopic 管理界面
- **THEN** 系统 MAY 展示 candidate 状态、命中次数和归档操作
- **AND** 该管理状态 SHALL NOT 改变日报阅读层的注意力排序
