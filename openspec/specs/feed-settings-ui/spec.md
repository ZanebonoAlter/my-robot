# Feed Settings UI

## Purpose

TBD
## Requirements
### Requirement: Feed 卡片展示完整处理管线 toggle
每个 feed 卡片 SHALL 展示 4 个 toggle 开关，按管线执行顺序排列：Firecrawl 全文抓取、打标签、AI 摘要、内容补全。

#### Scenario: 展示所有 toggle
- **WHEN** 用户打开全局设置的订阅源配置 tab
- **THEN** 每个 feed 卡片底部显示 4 个 toggle：Firecrawl、打标签、AI 摘要、内容补全

#### Scenario: Toggle 状态与后端数据一致
- **WHEN** feed 的 firecrawl_enabled = true, tagging_enabled = false, article_summary_enabled = true, completion_on_refresh = true
- **THEN** 对应 toggle 分别为 开/关/开/开

### Requirement: Toggle 变更即时保存
用户切换 toggle SHALL 立即调用 feed update API 保存到后端，无需额外提交按钮。

#### Scenario: 切换 toggle
- **WHEN** 用户点击某 feed 的"打标签" toggle
- **THEN** 前端调用 PATCH /api/feeds/:id 更新 tagging_enabled，并刷新 feed 列表

### Requirement: 分类标题可折叠
订阅源配置 tab 中的分类标题 SHALL 支持点击折叠/展开。默认展开。折叠状态在当前页面生命周期内保持。

#### Scenario: 折叠分类
- **WHEN** 用户点击某分类标题
- **THEN** 该分类下的 feed 列表收起，标题显示折叠图标（▶）

#### Scenario: 展开分类
- **WHEN** 用户点击已折叠的分类标题
- **THEN** 该分类下的 feed 列表展开，标题显示展开图标（▼）

#### Scenario: 刷新页面后恢复默认
- **WHEN** 用户关闭并重新打开全局设置对话框
- **THEN** 所有分类恢复为展开状态

### Requirement: 最大文章数"无限制"语义正确
最大文章数选项 SHALL 使用 `0` 表示无限制。后端 `CleanupOldArticles` SHALL 将 `maxArticles <= 0` 视为不限制。

#### Scenario: 选择无限制
- **WHEN** 用户选择"无限制"选项
- **THEN** 前端发送 `max_articles: 0` 到后端

#### Scenario: 后端不清理 max_articles=0 的 feed
- **WHEN** feed.MaxArticles = 0 且文章数超过任意值
- **THEN** CleanupOldArticles 不删除任何文章

#### Scenario: 兼容旧的 9999 值
- **WHEN** feed.MaxArticles = 9999
- **THEN** 前端显示"无限制"，后端行为与 max_articles=0 一致（不删除）

### Requirement: Feed 卡片展示 firecrawl 和补全 toggle
Firecrawl toggle SHALL 控制 `firecrawl_enabled` 字段，内容补全 toggle SHALL 控制 `completion_on_refresh` 字段。

#### Scenario: 切换 Firecrawl toggle
- **WHEN** 用户点击 Firecrawl toggle
- **THEN** 前端调用 PATCH /api/feeds/:id 更新 firecrawl_enabled

#### Scenario: 切换内容补全 toggle
- **WHEN** 用户点击内容补全 toggle
- **THEN** 前端调用 PATCH /api/feeds/:id 更新 completion_on_refresh

### Requirement: 订阅源管理区聚合全部 feed 管理入口
设置工作区「订阅源」section SHALL 提供 4 个管理入口：添加订阅源、添加分类、导入 OPML、导出 OPML，入口与订阅源主列表同处一屏可见位置。首页顶栏 MUST NOT 再出现这 4 个按钮。

#### Scenario: 设置页可见全部管理入口
- **WHEN** 用户打开 `/settings?section=feeds`
- **THEN** 订阅源管理区显示「添加订阅源」「添加分类」「导入」「导出」4 个入口

#### Scenario: 添加订阅源
- **WHEN** 用户点击「添加订阅源」入口
- **THEN** 打开 `AddFeedDialog` 对话框，行为与迁移前首页顶栏一致

#### Scenario: 添加分类
- **WHEN** 用户点击「添加分类」入口
- **THEN** 打开 `AddCategoryDialog` 对话框，行为与迁移前首页顶栏一致

#### Scenario: 导入 OPML
- **WHEN** 用户点击「导入」入口
- **THEN** 打开 `ImportOpmlDialog` 对话框，行为与迁移前首页顶栏一致

#### Scenario: 导出 OPML
- **WHEN** 用户点击「导出」入口
- **THEN** 浏览器下载 `feeds-export-<date>.opml` 文件，行为与迁移前首页顶栏一致

#### Scenario: 首页顶栏不再出现管理按钮
- **WHEN** 用户打开首页 `/`
- **THEN** 顶栏仅显示：暂停分析、AI 健康、刷新、全部已读、设置、新手引导、主题切换共 7 个按钮
- **AND** 不存在添加订阅、添加分类、导入、导出按钮

#### Scenario: 空状态引导入口保留
- **GIVEN** 用户没有任何订阅源和分类
- **WHEN** 用户打开首页 `/`
- **THEN** `FeedEmptyGuide` 空状态引导仍提供「添加订阅」按钮并可正常打开 `AddFeedDialog`

