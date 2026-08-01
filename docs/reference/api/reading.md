# 阅读行为（Reading Behavior）

> 旧 `/api/user-preferences/*` 端点已随偏好分数体系废弃删除（preference-vector-feed-discovery）；偏好向量画像见 [preference-profile.md](preference-profile.md)，订阅源发现见 [discovery.md](discovery.md)。

## 阅读行为 Reading Behavior

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/reading-behavior/track` | 记录单条 |
| POST | `/api/reading-behavior/track-batch` | 批量记录 |
| GET | `/api/reading-behavior/stats` | 阅读统计 |

### POST /api/reading-behavior/track

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `article_id` | uint | 是 | 文章 ID |
| `feed_id` | uint | 是 | 订阅源 ID |
| `session_id` | string | 是 | 会话 ID |
| `event_type` | string | 是 | open, close, scroll, favorite 等 |
| `category_id` | uint* | 否 | 留空自动从 feed 填充 |
| `scroll_depth` | int | 否 | 滚动深度 |
| `reading_time` | int | 否 | 秒 |

返回创建的记录。

### POST /api/reading-behavior/track-batch

```json
{ "events": [ { ...同 track 格式... }, ... ] }
```

返回 `{ "success": true, "message": 5 }`（`message` 为写入条数）。

### GET /api/reading-behavior/stats

```json
{
  "success": true,
  "data": {
    "total_articles": 200,
    "total_reading_time": 18000,
    "avg_reading_time": 90.5,
    "avg_scroll_depth": 72.3,
    "most_active_feed_id": 3,
    "most_active_category": 1
  }
}
```

---

> **已移除**：`GET /api/user-preferences` 与 `POST /api/user-preferences/update`（旧偏好分数聚合端点）已删除。`reading_behaviors` 采集链路保留，改为偏好向量画像的权重源，见 [preference-profile.md](preference-profile.md)。
