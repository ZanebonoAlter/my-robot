# feed-settings-ui Delta — fix-quality-audit-p0

## ADDED Requirements

### Requirement: AI 摘要开关状态字段名契约
前端读取 feed AI 摘要开关时 SHALL 使用后端 `ToDict` 返回的 `article_summary_enabled` 字段（snake_case），不得使用 `ai_summary_enabled`（后端从不返回该键）。字段缺省（undefined/null）时前端 SHALL 按后端 gorm 默认值 `false` 处理，不得回退为 `true`。`aiSummaryEnabled`/`ai_summary_enabled` 相关死字段与死 key SHALL 从前端类型定义中移除。

#### Scenario: feed 开关状态真实反映后端值
- **WHEN** 后端返回 feed 的 `article_summary_enabled: false`
- **THEN** 前端该 feed 的 AI 摘要开关状态为关闭（旧行为因读错键恒为 true）

#### Scenario: 字段缺省时按后端默认处理
- **WHEN** 后端返回的 feed 数据中 `article_summary_enabled` 缺失或为 null
- **THEN** 前端该 feed 的 AI 摘要开关状态为关闭（false），与 gorm default:false 一致

#### Scenario: 死字段不再存在于类型定义
- **WHEN** 检查 `front/app/types/feed.ts` 与 `front/app/stores/api.ts` 的响应/更新接口
- **THEN** 不存在 `aiSummaryEnabled` / `ai_summary_enabled` 字段定义
