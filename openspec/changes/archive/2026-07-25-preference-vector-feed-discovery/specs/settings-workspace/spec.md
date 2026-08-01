# Spec Delta: settings-workspace

## MODIFIED Requirements

### Requirement: Long Lists

队列和其他长列表 SHALL 使用分页、窗口化或受限默认条数。

#### Scenario: Queue history

- **WHEN** 用户进入队列 section
- **THEN** 首屏显示摘要和有限数量的最近记录
- **AND** 用户可分页或进入完整记录视图

#### Scenario: Bounded initial rendering

- **WHEN** 用户首次进入队列或兴趣画像 section
- **THEN** 页面只挂载当前活动视图和当前页数据
- **AND** 非活动队列列表不得同时保留在 DOM
- **AND** 不能以存在分页控件但仍渲染完整数据集的方式满足本要求

## ADDED Requirements

### Requirement: 兴趣画像 section 内容

`preferences` section SHALL 展示新「兴趣画像」视图：按 SemanticBoard 分组展示偏好 top 标签与权重、偏好来源（行为/问答种子）、最后计算时间，并提供手动重算入口。旧「阅读偏好面板」（阅读分/兴趣分列表与阅读统计）MUST 移除。

#### Scenario: 画像视图按版块展示

- **WHEN** 用户进入 `preferences` section
- **THEN** 页面按版块分组展示 top 标签与权重
- **AND** 展示偏好来源标识与最后计算时间

#### Scenario: 手动重算

- **WHEN** 用户在画像视图点击重新计算
- **THEN** 系统触发偏好重算并刷新视图

#### Scenario: 无数据空态

- **GIVEN** 无任何偏好向量
- **WHEN** 用户进入 `preferences` section
- **THEN** 页面展示空态引导（阅读文章或在发现页通过问答建立兴趣）
- **AND** 不出现恒为 0 的伪分数
