## MODIFIED Requirements

### Requirement: 状态机字段的 GORM default tag 保留

TopicTag / TagMergeSuggestion 的 `Status` 字段 SHALL 保留 GORM `default:` tag（`default:active` / `default:pending`）。该 tag 为功能必需而非风格选择：无 `default:` tag 时 GORM 对零值字段显式 INSERT `""`，DB 层 DEFAULT 无机会生效，产生非法状态行破坏 `status='active'` 类严格过滤（getBoardArticles 等）。其余非状态机字段的 tag 剥除治理（a0b03bdc）维持不变。

#### Scenario: 新建 TopicTag 未显式设 status 默认 active

- **WHEN** 以 GORM Create 插入 TopicTag 且不设置 Status 字段
- **THEN** 落库行 status='active'（GORM default tag 生效，DB DEFAULT 兜底），而非空串
- **AND** `getBoardArticles` 的 `status='active'` 过滤可命中该 tag

#### Scenario: 新建 SemanticLabel 未显式设 context_layers 由 DB DEFAULT 填充

- **WHEN** 以 GORM Create 插入 SemanticLabel 且 ContextLayers 为 nil
- **THEN** INSERT 语句不显式写该列，落库值为 DB DEFAULT `["week","month","year","all"]`，不触发 not-null 约束错误

#### Scenario: jsonb default tag 转义正确性

- **WHEN** 检查 `SemanticLabel.ContextLayers` 的 GORM tag 字面量
- **THEN** default 表达式内为原生双引号（`default:'["week","month","year","all"]'`），不含反斜杠转义（Go 反引号字符串中 `\"` 是字面反斜杠，会损坏 GORM 解析导致显式写 NULL）
