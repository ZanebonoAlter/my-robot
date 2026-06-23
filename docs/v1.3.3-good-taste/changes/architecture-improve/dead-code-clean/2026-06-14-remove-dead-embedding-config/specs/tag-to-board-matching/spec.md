## ADDED Requirements

### Requirement: Embedding 配置入口唯一性
系统 SHALL NOT 提供独立的 embedding 阈值、维度或模型的全局配置入口。Tag 与 SemanticBoard 匹配参数的唯一用户可调入口 SHALL 为 `semantic_board_match_*` 系列配置（通过标签管理-匹配规则对话框维护）。embedding 的 model、provider、dimension SHALL 由 AI 路由（capability-routes）决定，不接受用户在设置页直接覆盖。

#### Scenario: 设置页不含 embedding 配置栏
- **WHEN** 用户打开设置页查看可用配置分区
- **THEN** 系统 SHALL NOT 展示名为 "Embedding" 或包含相似度阈值/维度/模型字段的配置分区

#### Scenario: 旧 embedding 配置 API 不可用
- **WHEN** 客户端请求 `GET /embedding/config` 或 `PUT /embedding/config`
- **THEN** 系统 SHALL 返回 404（路由未注册）

#### Scenario: 板块匹配阈值滑块不存在
- **WHEN** 用户在设置页查找 "板块匹配阈值" 滑块
- **THEN** 系统 SHALL NOT 提供该控件；调整匹配行为 SHALL 通过 `semantic_board_match_*` 参数进行

#### Scenario: 唯一可调入口仍生效
- **WHEN** 用户在标签管理-匹配规则对话框将 `semantic_board_match_sim_threshold` 从 0.6 调整为 0.72
- **THEN** 后续 tag-board 匹配 SHALL 使用新阈值（验证配置入口收敛后唯一入口的功能完整性）
