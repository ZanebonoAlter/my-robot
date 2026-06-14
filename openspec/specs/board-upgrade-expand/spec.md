## Purpose

扩充已有版块的升级模式，查找可归入现有版块的辅助标签并生成 merge_into_existing 建议。

## Requirements

### Requirement: Upgrade Mode Parameter

`GenerateSuggestions` 和 `suggestUpgrades` API SHALL 接受 `mode` 参数：
- `discover_new`（默认）：当前逻辑，查找未分配的辅助标签聚类
- `expand_existing`：查找已分配到版块的辅助标签，生成 merge_into_existing 建议

#### Scenario: Default mode preserves current behavior

- **WHEN** `suggestUpgrades` 被调用时不带 `mode` 参数
- **THEN** 系统 SHALL 以 `discover_new` 模式运行，行为与变更前完全一致

#### Scenario: Explicit discover_new mode

- **WHEN** `suggestUpgrades` 被调用时 `?mode=discover_new`
- **THEN** 系统 SHALL 查找 NOT EXISTS board_composition 的辅助标签，聚类后让 LLM 判断 create_new 或 skip

#### Scenario: Expand existing mode

- **WHEN** `suggestUpgrades` 被调用时 `?mode=expand_existing`
- **THEN** 系统 SHALL 查找已存在于 board_composition 的辅助标签，聚类后让 LLM 判断 create_new、skip 或 merge_into_existing

### Requirement: Expand Mode Candidate Collection

`collectCandidates` 在 `mode=expand_existing` 时 SHALL 查询已分配到 board 的辅助标签（即 EXISTS board_composition），并为每个候选附加所属 board 信息。

#### Scenario: Collect assigned labels in expand mode

- **WHEN** `collectCandidates` 以 `mode=expand_existing` 运行，有标签 "AI"（board #42）和 "光伏"（board #15）
- **THEN** 系统 SHALL 返回包含 "AI" 和 "光伏" 的候选列表，每个候选带有所属 board_id 信息

#### Scenario: No assigned labels in expand mode

- **WHEN** `collectCandidates` 以 `mode=expand_existing` 运行，但所有辅助标签均未分配到任何 board
- **THEN** 系统 SHALL 返回空候选列表，不调用 LLM

### Requirement: Expand Mode LLM Prompt

`mode=expand_existing` 时，LLM prompt SHALL 包含 `merge_into_existing` 作为有效决策选项，并在簇上下文中包含候选标签当前所属 board 的名称和组成。

#### Scenario: LLM produces merge_into_existing in expand mode

- **WHEN** 簇包含 "AI"（当前属于 board "人工智能"）和 "大语言模型"（当前属于 board "人工智能"），LLM 判断这些标签应合并到 "人工智能"
- **THEN** LLM SHALL 返回 `merge_into_existing` 决策，包含 `target_board_id`

#### Scenario: LLM produces create_new in expand mode

- **WHEN** 簇包含来自不同 board 的标签，LLM 判断它们更应组成新 board
- **THEN** LLM SHALL 返回 `create_new` 决策，与 discover_new 模式结果格式一致

### Requirement: Expand Mode Filter Accepts merge_into_existing

`filterSemanticBoardUpgradeSuggestions` 在 `mode=expand_existing` 时 SHALL 接受 `create_new`、`skip` 和 `merge_into_existing` 三种决策。在 `mode=discover_new` 时 SHALL 继续过滤掉 `merge_into_existing`。

#### Scenario: Filter accepts merge_into_existing in expand mode

- **WHEN** `mode=expand_existing`，LLM 返回一个 `merge_into_existing` 建议
- **THEN** 过滤器 SHALL 保留该建议，传递给前端

#### Scenario: Filter rejects merge_into_existing in discover mode

- **WHEN** `mode=discover_new`，LLM 错误地返回一个 `merge_into_existing` 建议
- **THEN** 过滤器 SHALL 丢弃该建议（防御性过滤）

### Requirement: Frontend Mode Selector

前端 SHALL 在 Upgrade Suggestion Panel 中提供模式选择 UI（如 radio/toggle），在触发 "Generate Suggestions" 前选择模式。选择 SHALL 作为 `?mode=` 参数传递给 API。

#### Scenario: User selects expand mode

- **WHEN** 用户在升级面板选择 "扩充已有版块" 模式并点击生成建议
- **THEN** 前端 SHALL 发送 `POST /api/semantic-boards/upgrade/suggest?mode=expand_existing`

#### Scenario: User selects discover mode

- **WHEN** 用户在升级面板选择 "发现新版块" 模式并点击生成建议
- **THEN** 前端 SHALL 发送 `POST /api/semantic-boards/upgrade/suggest?mode=discover_new`