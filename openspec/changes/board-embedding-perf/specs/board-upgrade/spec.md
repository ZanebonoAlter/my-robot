## Purpose (Delta from `board-upgrade`)

拆分升级为两个方向（发现新版块 / 扩充已有版块），通过 `mode` 参数切换。详细行为定义见 `board-upgrade-expand` spec。

## Requirements

### Requirement: suggestUpgrades API 支持 mode 参数

`POST /api/semantic-boards/upgrade/suggest` SHALL 接受 `?mode=discover_new|expand_existing` 查询参数（默认 `discover_new`）。`GenerateSuggestions` SHALL 将 mode 传递给 `collectCandidates`、LLM prompt 构造和 `filterSemanticBoardUpgradeSuggestions`。

#### Scenario: API default mode backward compatible

- **WHEN** 前端调用 `POST /api/semantic-boards/upgrade/suggest` 不带 mode 参数
- **THEN** 系统 SHALL 以 `discover_new` 模式运行，响应格式与变更前一致

#### Scenario: API expand mode

- **WHEN** 前端调用 `POST /api/semantic-boards/upgrade/suggest?mode=expand_existing`
- **THEN** 系统 SHALL 返回可能包含 `merge_into_existing` 决策的建议列表

### Requirement: getUpgradeCandidates API 支持 mode 参数

`POST /api/semantic-boards/upgrade/candidates` SHALL 接受 `?mode` 查询参数，影响 `collectCandidates` 的查询范围。

#### Scenario: Candidates in discover mode

- **WHEN** 调用 `candidates?mode=discover_new`
- **THEN** 系统 SHALL 返回不在 board_composition 中的候选辅助标签

#### Scenario: Candidates in expand mode

- **WHEN** 调用 `candidates?mode=expand_existing`
- **THEN** 系统 SHALL 返回已在 board_composition 中的辅助标签，带有所属 board 信息

### Requirement: Upgrade Confirm 触发缓存失效

`ConfirmSuggestion` 在成功执行 DB 事务后 SHALL 触发 board match cache 失效（board auxiliaries + board embeddings），确保后续匹配使用最新数据。

#### Scenario: Confirm create_new invalidates cache

- **WHEN** 用户确认 create_new 建议创建 board #200
- **THEN** 系统 SHALL 清除 board auxiliaries 缓存和 board embeddings 缓存

#### Scenario: Confirm merge_into_existing invalidates cache

- **WHEN** 用户确认 merge_into_existing 将标签合并到 board #42
- **THEN** 系统 SHALL 清除 board auxiliaries 缓存和 board embeddings 缓存
