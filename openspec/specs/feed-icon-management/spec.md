# Feed Icon Management

## Purpose

定义 Feed 图标的来源状态机与获取策略。通过 `IconSource` 字段显式区分"系统自动抓取的图标（可刷新）"与"用户设定的图标（不可覆盖）"，统一处理 favicon 获取（RSS channel link → /favicon.ico）、默认值收敛、前端降级渲染，消除 icon 字段语义混乱、favicon 取错域名、Google s2 国内被墙等问题。

## Requirements

### Requirement: Feed 模型 icon_source 字段
Feed 模型 SHALL 包含 `icon_source` string 字段，取值范围为 `auto` / `custom` / `fallback`，数据库 default 为 `fallback`。该字段与 `icon` 字段配合，`icon` 承载值（iconify id 或图片 URL），`icon_source` 承载来源语义。

#### Scenario: 新建 feed 默认 fallback
- **WHEN** 创建新 feed（未显式指定 icon）
- **THEN** icon_source 为 `fallback`，icon 为 `mdi:rss`

#### Scenario: OPML 导入默认 fallback
- **WHEN** 通过 OPML 导入 feed
- **THEN** icon_source 为 `fallback`，icon 为 `mdi:rss`

### Requirement: RefreshFeed 按状态机决定是否重算 icon
`RefreshFeed` SHALL 仅当 `icon_source` 为 `auto` 或 `fallback` 时重算 icon。当 `icon_source` 为 `custom` 时，SHALL 跳过 icon 重算，保持用户设定值不变。

#### Scenario: custom 状态不被刷新覆盖
- **WHEN** feed.icon_source = `custom`，执行 RefreshFeed
- **THEN** icon 和 icon_source 均不变

#### Scenario: fallback 状态触发重算
- **WHEN** feed.icon_source = `fallback`，执行 RefreshFeed
- **THEN** 尝试用 RSS image 或站点 favicon 重算 icon

#### Scenario: auto 状态可重复刷新
- **WHEN** feed.icon_source = `auto`，执行 RefreshFeed
- **THEN** 尝试用 RSS image 或站点 favicon 重算 icon（可能换更优图）

### Requirement: icon 选图优先级
当 RefreshFeed 重算 icon 时，SHALL 按以下优先级选取：1) RSS `<image>` 标签 URL（parsed.Image）；2) 站点 favicon（RSS channel link 的 host + `/favicon.ico`）。取到任一非空值则置 icon_source = `auto`，两者均失败则保持 icon_source = `fallback`、icon = `mdi:rss`。

#### Scenario: RSS image 优先
- **WHEN** parsed.Image 非空且站点 favicon 也可取
- **THEN** icon = parsed.Image，icon_source = `auto`

#### Scenario: 仅有站点 favicon
- **WHEN** parsed.Image 为空且 RSS channel link 可解析出 host
- **THEN** icon = `{scheme}://{host}/favicon.ico`，icon_source = `auto`

#### Scenario: 两者均不可取
- **WHEN** parsed.Image 为空且 RSS channel link 为空或无法解析 host
- **THEN** icon = `mdi:rss`，icon_source = `fallback`

### Requirement: 不再使用文章封面图作为 feed icon
RefreshFeed 重算 icon 时 SHALL NOT 读取文章的 `ImageURL`。文章封面图属于文章级别资源，语义上不等于站点 logo。

#### Scenario: 文章图存在但不被用作 feed icon
- **WHEN** feed 下存在带 ImageURL 的文章，但 parsed.Image 和站点 favicon 均失败
- **THEN** feed.icon 保持 `mdi:rss`，不回退到文章图

### Requirement: UpdateFeed 设置 icon 时置 custom
`PATCH /api/feeds/:id` SHALL 在接受非空 `icon` 字段时，将 `icon_source` 置为 `custom`，确保用户设定的图标不被后续 RefreshFeed 覆盖。

#### Scenario: 用户更新 icon
- **WHEN** 发送 PATCH 请求 body 包含 `{"icon": "mdi:github"}`
- **THEN** feed.icon = `mdi:github`，feed.icon_source = `custom`

### Requirement: FetchFaviconURL 使用站点 URL 而非 feed URL
`FetchFaviconURL` SHALL 接收站点首页 URL（RSS channel link）作为参数，返回 `{scheme}://{host}/favicon.ico`。SHALL NOT 使用 RSS feed URL（聚合器域名）或 Google s2 服务。

#### Scenario: 从站点 URL 拼 favicon
- **WHEN** siteURL = `https://example.com/articles`
- **THEN** 返回 `https://example.com/favicon.ico`

#### Scenario: 站点 URL 无法解析
- **WHEN** siteURL 为空或解析失败
- **THEN** 返回空字符串（由调用方保持 fallback 态）

### Requirement: 前端 FeedIcon 图片加载失败降级
前端 `FeedIcon.vue` SHALL 在 `<img>` 加载失败（onerror）时降级渲染 `mdi:rss` Icon 组件，SHALL NOT 留白（display:none）。

#### Scenario: favicon URL 加载失败
- **WHEN** icon 为 `https://example.com/favicon.ico` 但加载触发 onerror
- **THEN** 渲染 `mdi:rss` Icon 组件替代

#### Scenario: 正常 URL 加载成功
- **WHEN** icon 为有效图片 URL 且加载成功
- **THEN** 正常渲染 `<img>`

### Requirement: 存量数据迁移幂等
icon_source 字段的迁移 SHALL 幂等：GORM AutoMigrate 加列带 default `fallback`，再用幂等 `UPDATE ... CASE WHEN` SQL 按 icon 现值推断 icon_source。重复执行不影响已迁移的行。

#### Scenario: URL 值归入 auto
- **WHEN** 存量 feed.icon = `https://example.com/logo.png`
- **THEN** 迁移后 icon_source = `auto`

#### Scenario: 占位符归入 fallback
- **WHEN** 存量 feed.icon IN ('', 'rss', 'mdi:rss')
- **THEN** 迁移后 icon_source = `fallback`，icon 统一为 `mdi:rss`

#### Scenario: 其他 iconify id 归入 custom
- **WHEN** 存量 feed.icon = `mdi:github`
- **THEN** 迁移后 icon_source = `custom`（保守保护可能的用户设定）
