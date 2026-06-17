## Why

Feed 的 icon 字段当前是一个"什么都往里塞"的黑洞：同一个 `feeds.icon` 列里同时存在 5 种语义不同的值——OPML 默认值 `"rss"`、handler 兜底 `"mdi:rss"`、RSS `<image>` URL、第一篇文章封面图、以及 Google s2 favicon URL。值类型不区分（iconify id / 图片 URL / 占位标记混在一起），导致：

1. **状态机隐式且腐化**：`feed_service.go:67` 用 `if Icon == "" || Icon == "rss" || Icon == "mdi:rss"` 连判三种占位值来决定是否重算，每新增一种默认值就要改这行；而且一旦 icon 变成任何 URL（哪怕是失效的 Google s2），就**永远冻结不再刷新**。
2. **favicon 取错域名**：`rss_parser.go:216` 用 `feedURL.Host` 拼 Google s2，而 `feedURL` 是 RSS feed 地址，host 经常是 `feedburner.com` / `rsshub.app` 等聚合器，拿到的 favicon 是聚合器的而非内容站的。
3. **Google s2 国内被墙**：拼出的 `https://www.google.com/s2/favicons?...` 在国内 100% 加载失败，而前端 `<img @error>` 只是 `display:none`，**连 `mdi:rss` 占位都不留，直接留白**。
4. **文章图语义错位**：`feed_service.go:71-72` 把第一篇文章的 `ImageURL` 当 feed icon，文章封面 ≠ 站点 logo，显示成随机首图。
5. **用户自定义 icon 无主权**：后端 `UpdateFeed` 支持改 icon，但下次 `RefreshFeed` 只要值不等于那三个占位符就不会覆盖——看似安全，实则"碰巧不覆盖"，不是显式契约；且前端没有编辑入口，功能等于不存在。
6. **侦探墙数据白拉**：后端 `repository.go:299,369` 已为侦探墙返回 `feed_icon`，前端 composable 也读进了 `feedIcon`，但 `TimelineItem.vue` / `TopicGraphArticleCard.vue` 完全不渲染它，只显示 feedName 文本。

根因是**缺少一个显式的 icon 来源状态字段**，让系统无法区分"系统自动抓的图（可刷新）"与"用户设定的图（不可覆盖）"。

## What Changes

引入 `IconSource` 字段，把 icon 的来源显式化，统一为一个清晰的三态状态机：

```
IconSource 状态流转：
  fallback  ──refresh──▶  auto      (系统抓到 RSS image / 站点 favicon)
     │                       │
     │                    refresh 可重复刷新（换更好的图）
     │                       │
     └─── 用户编辑 ──▶  custom    (用户主权，系统永不覆盖)
```

- **状态机字段**：Feed model 新增 `IconSource string`（值：`auto` / `custom` / `fallback`，default `fallback`），与现有 `Icon` 字段配合。`Icon` 只存值（iconify id 或图片 URL），来源语义由 `IconSource` 承载。
- **favicon 获取重写**：`FetchFaviconURL` 改用 RSS channel link（`parsed.Link`，站点首页 URL）的 host 拼 `/favicon.ico`，**弃用 Google s2**。RSS channel link 已在 `ParsedFeed.Link` 中解析好，零额外请求。
- **RefreshFeed 选图逻辑收窄**：移除"第一篇文章图当 feed icon"的分支；选图优先级变为 `RSS <image>` → 站点 `/favicon.ico`。仅当 `IconSource in (auto, fallback)` 时重算；`custom` 永不覆盖。
- **默认值统一**：三种历史占位值（`""` / `"rss"` / `"mdi:rss"`）统一收敛为 `IconSource=fallback, Icon="mdi:rss"`。
- **前端 FeedIcon 降级修复**：`<img @error>` 失败时降级渲染 `mdi:rss` 而非留白；移除前端自行拼 `/favicon.ico` 的重复逻辑（favicon 获取责任归到后端状态机）。
- **侦探墙补渲染**：`TimelineItem.vue` / `TopicGraphArticleCard.vue` 渲染已拉取的 `feedIcon` 字段，消除"数据白拉"。
- **存量数据迁移**：一条幂等 `UPDATE ... CASE WHEN` SQL，按现有 `Icon` 值推断 `IconSource`（URL → auto，占位符 → fallback，其他 iconify id → custom）。

## Capabilities

### New Capabilities
- `feed-icon-management`: Feed 图标的来源状态机与获取策略。定义 `IconSource` 三态（auto/custom/fallback）、刷新覆盖规则、favicon 获取方式（RSS channel link → `/favicon.ico`）、前端降级渲染契约。

### Modified Capabilities
- `feed-settings-ui`: Feed 详情编辑新增 icon 编辑入口（iconify id 或 URL），写入时置 `IconSource=custom`；FeedIcon 组件改造降级逻辑。
- `narrative-board-generation` / `section-lifecycle`（侦探墙）：`TimelineItem` / `TopicGraphArticleCard` 补渲染 `feed_icon`，修复数据白拉。

## Impact

- **后端**：
  - `models/feed.go`：新增 `IconSource` 字段（GORM AutoMigrate，default `fallback`）
  - `reader/service/rss_parser.go`：`FetchFaviconURL` 重写（参数从 feedURL 改为 siteURL，拼 `/favicon.ico`）
  - `reader/service/feed_service.go`：`RefreshFeed` 选图逻辑改为按 `IconSource` 状态机判断，移除文章图分支
  - `reader/handler/feed_handler.go`：CreateFeed/UpdateFeed 写入 `IconSource`（创建默认 fallback，用户传 icon 则 custom）
  - `reader/handler/opml.go`：OPML 导入置 `IconSource=fallback`
  - `topicgraph/repository/repository.go`：feed_icon 输出不变（值已是 URL/iconify id）
  - 存量迁移 SQL（幂等，可重复执行）
- **前端**：
  - `components/feed/FeedIcon.vue`：`@error` 降级到 `mdi:rss`，移除 `getFaviconFromUrl` 重复逻辑
  - `features/topic-graph/components/TimelineItem.vue` / `TopicGraphArticleCard.vue`：补渲染 feedIcon
  - `features/settings/components/FeedDetailEditor.vue`（可选）：新增 icon 编辑输入
  - `types/feed.ts`：新增 `icon_source?: 'auto' | 'custom' | 'fallback'`
- **文档**：`docs/reference/architecture/` feed icon 状态机说明；`docs/reference/database/` feeds 表 `icon_source` 列
- **迁移风险**：GORM AutoMigrate 加带 default 的列，存量行自动填 `fallback`，再用 CASE SQL 精修。回滚路径：`ALTER TABLE feeds DROP COLUMN icon_source`。
