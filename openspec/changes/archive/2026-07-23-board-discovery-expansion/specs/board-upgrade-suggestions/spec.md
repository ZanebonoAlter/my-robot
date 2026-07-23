## ADDED Requirements

### Requirement: 升级建议持久化存储

系统 SHALL 将每次生成的升级建议持久化到 `board_upgrade_suggestions` 表，字段至少包括：mode、decision（create_new / merge_into_existing / watch）、board_label、description、target_board_id（可空）、auxiliary_label_ids、confidence（high / llm）、evidence（shortlist/margin/co-tag 事件/泳道摘要快照）、status（pending / confirmed / dismissed）、created_at、resolved_at。建议 SHALL NOT 即算即弃。

#### Scenario: 生成建议落库

- **WHEN** 生成一轮建议（手动触发或 scheduler 触发），产出 3 条非 skip 建议
- **THEN** 系统 SHALL 将 3 条建议写入 `board_upgrade_suggestions`，初始 status=pending

#### Scenario: skip 决策不落库

- **WHEN** LLM 对某簇判 skip
- **THEN** 系统 SHALL NOT 为该簇写入建议记录

### Requirement: 建议生成幂等

系统 SHALL 对 pending 状态的建议按 hash(mode, decision, target_board_id, 排序后 auxiliary_label_ids) 做唯一约束；生成时与既有 pending 建议哈希相同的 SHALL 跳过插入。

#### Scenario: 重复生成不重复落库

- **WHEN** 连续两天生成，某簇的 aux 集合与决策未变
- **THEN** 系统 SHALL 保留原 pending 建议，不插入重复记录

### Requirement: dismissed 冷却期

系统 SHALL 记录用户 dismiss 的建议及其哈希；在 `semantic_board_upgrade_suggestion_dismiss_cooldown_days`（默认 14 天，ai_settings 可配）内，相同哈希的建议 SHALL NOT 再次生成；冷却期满后允许重新生成。

#### Scenario: 冷却期内不重复建议

- **WHEN** 用户 3 天前 dismiss 了"DeepSeek → merge 生成式AI版块"建议，今日再次生成
- **THEN** 系统 SHALL NOT 再次产出相同哈希的建议

#### Scenario: 冷却期满允许重生

- **WHEN** 用户 15 天前 dismiss 了某建议，冷却配置为 14 天
- **THEN** 系统 SHALL 允许再次生成该建议

### Requirement: 建议查询 API 读持久化表

系统 SHALL 提供 `GET /api/semantic-boards/upgrade-suggestions`，从建议表读取，支持按 status、decision 过滤；默认返回 pending 且非 watch 的建议，按 confidence（high 优先）与 created_at 排序。

#### Scenario: 默认列表

- **WHEN** 前端请求 `GET upgrade-suggestions` 无参数
- **THEN** 系统 SHALL 返回 status=pending 且 decision≠watch 的建议，high confidence 在前

#### Scenario: 查看观察池

- **WHEN** 前端请求 `GET upgrade-suggestions?decision=watch`
- **THEN** 系统 SHALL 返回观察池中的单标签簇建议

### Requirement: 建议 dismiss 与 confirm 联动

系统 SHALL 提供 `POST /api/semantic-boards/upgrade-suggestions/:id/dismiss` 将建议置为 dismissed。用户确认建议时，`POST /upgrade-execute` 请求体 SHALL 携带 `suggestion_id`，系统 SHALL 在写 board_composition 的同一事务内按 `suggestion_id` 将对应 pending 建议置为 confirmed 并记录 resolved_at；事务失败时建议状态不变。未携带 `suggestion_id` 的请求（兼容旧前端）SHALL 正常执行版块写入但不联动建议状态。

#### Scenario: dismiss 建议

- **WHEN** 用户对 pending 建议调用 dismiss API
- **THEN** 系统 SHALL 将其 status 置为 dismissed，记录 resolved_at，并写入冷却记录

#### Scenario: confirm 建议

- **WHEN** 用户对 pending 的 merge 建议执行 upgrade-execute（请求体携带 `suggestion_id`）成功
- **THEN** 系统 SHALL 在写 board_composition 的同一事务内按 `suggestion_id` 将建议置为 confirmed；事务失败时建议状态不变

### Requirement: scheduler 定期生成建议

系统 SHALL 注册定时任务 `job_board_upgrade_suggest`（默认每天固定时间点 06:30 触发，可配置），自动以 discover_new 模式生成建议并入表；任务失败 SHALL 仅记录日志，不影响其他定时任务。时序为松耦合（固定时间点，不保证紧随日报，见 design D4）。手动 `POST /api/semantic-boards/upgrade-suggestions/generate` SHALL 执行与定时任务相同的生成逻辑，返回本次新增建议数。

#### Scenario: 定时生成

- **WHEN** 到达调度时间
- **THEN** 系统 SHALL 执行一轮建议生成，新增建议入表，日志记录新增/跳过（幂等）/冷却拦截数量

#### Scenario: 手动触发与定时任务等效

- **WHEN** 用户手动 POST /upgrade-suggestions/generate
- **THEN** 系统 SHALL 执行与定时任务相同的生成逻辑，返回本次新增建议数

### Requirement: 观察池建议自动回收

系统 SHALL 对 decision=watch 的 pending 建议，在创建满 `semantic_board_upgrade_watch_gc_days`（默认 30 天，ai_settings 可配）后仍未成簇的，自动置为 dismissed，避免观察池单调膨胀。

#### Scenario: 超期未成簇自动回收

- **WHEN** 某 watch 建议创建已满 `semantic_board_upgrade_watch_gc_days`（默认 30 天）且仍为 pending、未成簇
- **THEN** 系统 SHALL 在建议生成轮次或独立 GC 轮次中将其 status 置为 dismissed，并记录 resolved_at

#### Scenario: 观察池未超期不受影响

- **WHEN** 某 watch 建议创建未满 GC 天数
- **THEN** 系统 SHALL 保留其 pending 状态，不做回收
