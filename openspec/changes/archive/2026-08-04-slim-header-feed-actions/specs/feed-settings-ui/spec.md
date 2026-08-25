# feed-settings-ui delta — slim-header-feed-actions

## ADDED Requirements

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
