# 日报 API

基础地址：`http://localhost:5000/api`

## POST `/daily-reports/generate`

手动异步触发日报生成。`board_id` 为空时生成所有当日有事件标签的板块。

Request：

```json
{ "date": "2026-05-26", "board_id": 2849 }
```

Response `data`：

```json
{ "job_id": "...", "status": "processing" }
```

WebSocket 进度消息：

```json
{
  "type": "daily_report_progress",
  "job_id": "...",
  "board_id": 2849,
  "board_name": "刚果（金）局势",
  "status": "generating",
  "saved": 0,
  "progress": "0/1",
  "timestamp": "2026-05-26T..."
}
```

## GET `/semantic-boards/:id/section-timeline?days=30`

查询板块日报 section 时间线，供 2D 线索浏览和 3D 侦探墙复用。

Response `data`：

```json
{
  "sections": [
    {
      "id": 101,
      "report_id": 12,
      "period_date": "2026-05-26T12:00:00Z",
      "cluster_label": "...",
      "status": "continuing",
      "article_count": 5,
      "thread_count": 2,
      "image_url": "https://..."
    }
  ],
  "relations": [
    { "from_id": 100, "to_id": 101, "distance": 0.23 }
  ]
}
```

`image_url` 始终回传；无可用图片时为空字符串。后端优先从该 section 的线程关联文章中选择第一张非空图片，找不到时再从该 section 的 cluster tags 当天文章里选择第一张非空图片。

## GET `/daily-reports/sections/:id/lifecycle`

查询一个 section 所在连通分量的完整生命周期。响应结构与 `section-timeline` 相同，同样可能包含可选 `image_url`。

终态总会广播：

```json
{
  "type": "daily_report_done",
  "job_id": "...",
  "total_saved": 1,
  "total_boards": 1,
  "timestamp": "2026-05-26T..."
}
```

`daily_report_progress.status` 使用 `generating`、`completed`、`failed`。

## GET `/semantic-boards/:id/daily-reports?days=7`

查询板块日报列表。

**参数：**
- `days`：查询最近 N 天的日报。默认值为 7（当 `days` 未提供或 `days <= 0` 时）。对于任意正数 `days`，系统将按实际请求天数查询，不再静默截断到 30 天。

Response `data`：

```json
{
  "reports": [
    {
      "id": 1,
      "semantic_board_id": 2849,
      "period_date": "2026-05-26",
      "title": "...",
      "summary": "...",
      "status": "completed",
      "cluster_count": 2,
      "article_count": 3,
      "event_tag_count": 5,
      "created_at": "2026-05-26T..."
    }
  ]
}
```

## GET `/daily-reports/:id`

查询单篇日报详情（含 sections）。

Response `data`：

```json
{
  "report": {
    "id": 1,
    "semantic_board_id": 2849,
    "period_date": "2026-05-26T12:00:00Z",
    "title": "...",
    "summary": "...",
    "status": "completed",
    "highlights": [],
    "dynamics": "...",
    "sections": []
  }
}
```
