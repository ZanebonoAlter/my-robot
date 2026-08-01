# 订阅 Feeds

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/feeds` | 获取订阅列表 |
| GET | `/api/feeds/:feed_id` | 获取单个订阅 |
| POST | `/api/feeds` | 创建订阅 |
| PUT | `/api/feeds/:feed_id` | 更新订阅 |
| DELETE | `/api/feeds/:feed_id` | 删除订阅 |
| POST | `/api/feeds/:feed_id/refresh` | 刷新单个订阅 |
| POST | `/api/feeds/fetch` | 预览 Feed URL |
| POST | `/api/feeds/refresh-all` | 刷新所有订阅 |

---

### GET /api/feeds

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `page` | int | 1 | 页码 |
| `per_page` | int | 20 | 每页条数，≥10000 返回全部 |
| `category_id` | int | - | 按分类过滤 |
| `uncategorized` | string | - | `true` 查未分类 |

返回带分页的订阅列表，含 `article_count` 和 `unread_count`。

### GET /api/feeds/:feed_id

单个订阅详情，含文章统计。

### POST /api/feeds

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | RSS feed URL |
| `title` | string | 否 | 默认 `Untitled Feed` |
| `description` | string | 否 | 描述 |
| `category_id` | uint* | 否 | 分类 ID |
| `icon` | string | 否 | 图标值（iconify id、图片 URL，或系统本地化后的 `/icons/feeds/<id>.<ext>` 同源路径——由 RefreshFeed 自动管理，不建议手填）。传入非空值时 `icon_source` 置为 `custom`；不传则默认 `mdi:rss` + `icon_source=fallback` |
| `color` | string | 否 | 默认 `#8b5cf6` |
| `max_articles` | int | 否 | 默认 `100` |
| `refresh_interval` | int | 否 | 刷新间隔（分钟），默认 `60` |
| `ai_summary_enabled` | bool | 否 | 启用 AI 总结 |
| `article_summary_enabled` | bool | 否 | 启用文章级总结 |
| `completion_on_refresh` | bool | 否 | 刷新时自动补全 |
| `max_completion_retries` | int | 否 | 补全最大重试次数 |
| `firecrawl_enabled` | bool | 否 | 启用 Firecrawl |

`201`：返回创建的订阅。`409`：URL 已存在。

### PUT /api/feeds/:feed_id

只更新请求体中明确提供的字段。布尔字段需显式包含才生效。

传入非空 `icon` 时，`icon_source` 自动置为 `custom`（用户主权图标，后续 RefreshFeed 不会覆盖）。

### DELETE /api/feeds/:feed_id

删除订阅及其关联文章；本地化图标文件（`data/icons/feeds/<feed_id>.*`，若存在）一并清理，清理失败不阻断删除。

### GET /icons/*

静态路由，直接服务 `storage.icon_dir` 目录（默认 `data/icons`），提供本地化后的 feed 图标（如 `/icons/feeds/42.png`）。独立于前端产物托管，dev 与生产模式均可用；文件不存在返回 `404`。

### POST /api/feeds/:feed_id/refresh

后台刷新，`202 Accepted`：

```json
{ "success": true, "message": "Started refreshing feed in background" }
```

### POST /api/feeds/fetch

预览 RSS URL：

```json
{ "url": "https://example.com/feed.xml" }
```

返回 `{ "title": "...", "description": "..." }`。

### POST /api/feeds/refresh-all

`202`：

```json
{
  "success": true,
  "message": "Started refreshing all feeds in background",
  "data": { "total_feeds": 15 }
}
```
