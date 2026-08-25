## MODIFIED Requirements

### Requirement: 最大文章数"无限制"语义正确
最大文章数选项 SHALL 使用 `0` 表示无限制。后端 `CleanupOldArticles` SHALL 将 `maxArticles <= 0` 视为不限制。超出上限的行为语义为归档降级（保留原文、清除衍生数据），而非物理删除，详见 `article-retention` capability。

#### Scenario: 选择无限制
- **WHEN** 用户选择"无限制"选项
- **THEN** 前端发送 `max_articles: 0` 到后端

#### Scenario: 后端不清理 max_articles=0 的 feed
- **WHEN** feed.MaxArticles = 0 且文章数超过任意值
- **THEN** CleanupOldArticles 不归档任何文章

#### Scenario: 兼容旧的 9999 值
- **WHEN** feed.MaxArticles = 9999
- **THEN** 前端显示"无限制"，后端行为与 max_articles=0 一致（不归档）

## ADDED Requirements

### Requirement: 最大文章数控件文案反映归档语义
Feed 设置中最大文章数控件的辅助说明 SHALL 表达"超出后归档保留原文"语义，MUST NOT 出现"删除"措辞。

#### Scenario: 文案展示归档语义
- **WHEN** 用户查看 feed 卡片的最大文章数控件
- **THEN** 辅助说明表述为超出上限的文章将被归档（原文保留、不再出现在列表中）
