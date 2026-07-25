# 订阅源发现（Feed Discovery）

> 偏好向量画像 × RSSHub 路由目录 → 向量粗筛 + LLM 精排 → 推荐卡片状态机 + 问答冷启动。链路与业务约束见 [../flow/discovery.md](../flow/discovery.md)。

## RSSHub 路由目录

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/discovery/catalog/sync` | 触发 RSSHub 实例 `/api/namespace` 全量同步（content_hash diff + 参数标记 + gone） |
| GET | `/api/discovery/catalog/status` | 目录状态（路由总数 / 各 status 计数 / 最近同步时间） |

### POST /api/discovery/catalog/sync

后台拉取自建 RSSHub 实例全量路由元数据入库。返回同步摘要（新增/变更/消失计数）。

```json
{ "success": true, "data": { "total": 3245, "added": 18, "updated": 2, "gone": 0 } }
```

> 实例地址读 `ai_settings.rsshub_config.rsshub_base_url`（缺省回落 `http://47.110.71.194:1200`），见 [settings/rsshub](#rsshub-实例配置)。

### GET /api/discovery/catalog/status

```json
{
  "success": true,
  "data": { "total_routes": 3245, "ok": 2100, "broken": 32, "unknown": 1113, "last_synced_at": "2026-07-25T03:00:00Z" }
}
```

## 订阅源推荐

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/discovery/recommendations` | 推荐卡片列表（默认 `status=pending`） |
| POST | `/api/discovery/recommendations/refresh` | 换一批（粗筛 + 精排，幂等落库） |
| POST | `/api/discovery/recommendations/:id/accept` | 接受推荐 → 订阅落地 |
| POST | `/api/discovery/recommendations/:id/dismiss` | 拒绝（冷却 30 天，跨 source） |

### GET /api/discovery/recommendations

返回推荐卡片（含路由元数据 + 相似度 score + 匹配版块 + 参数说明）。

```json
{
  "success": true,
  "data": [
    {
      "id": "57",
      "route_id": "2103",
      "route_namespace": "36kr",
      "route_path": "newsflashes",
      "route_name": "36氪 快讯",
      "route_example": "36kr/newsflashes",
      "usable_directly": true,
      "requires_parameters": false,
      "parameters": "",
      "route_status": "ok",
      "board_id": "12",
      "board_label": "AI 芯片",
      "score": 0.78,
      "llm_reason": "覆盖半导体与 AI 芯片产业快讯，与你「芯片/制程」兴趣高度匹配。",
      "status": "pending"
    }
  ]
}
```

### POST /api/discovery/recommendations/refresh

手动刷新一轮推荐（每版块 top-N 粗筛 → LLM 精排 → 幂等落 pending）。返回本轮摘要。

```json
{ "success": true, "data": { "generated": 12, "skipped": 3 } }
```

### POST /api/discovery/recommendations/:id/accept

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `category_id` | uint | 否 | 订阅到分类 |
| `parameters` | map[string]string | 否 | `requires_parameters` 路由必填（参数 → 值） |

- `usable_directly`：直接 `CreateFeed`（一键订阅）。
- `requires_parameters`：用 `parameters` 走 `POST /feeds/fetch` 验证通过才 `CreateFeed`。
- 成功置 `status=accepted` + 记录 `accepted_feed_id`，返回创建的 feed。

```json
{ "success": true, "data": { /* feed 对象 */ }, "message": "feed created" }
```

### POST /api/discovery/recommendations/:id/dismiss

拒绝推荐，进入冷却期（默认 30 天，跨 source 生效——同 hash 的 qa/手动刷新重出都会被拦截）。

```json
{ "success": true, "message": "dismissed" }
```

## 问答发现

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/discovery/ask` | 自然语言提问 → 即时粗筛 + 精排推荐 + 种子偏好写入 |

### POST /api/discovery/ask

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `question` | string | 是 | 自然语言兴趣表达（如「我想看 AI 芯片相关资讯」） |

返回即时推荐卡片（同 recommendations 形状，同时以 `source=qa` 落推荐表，接受/拒绝走同一状态机）。问题 embedding 同时按阈值匹配版块、以 `source=seed` 加权合并写入偏好画像（冷启动）。

```json
{ "success": true, "data": [ /* RecommendationCard[] */ ] }
```

> 问答推荐恒落全局桶（`board_id=null`）；种子写入按阈值落对应版块。两者独立，见 [../flow/discovery.md](../flow/discovery.md) §业务约束。

## RSSHub 实例配置

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/settings/rsshub` | 读取 RSSHub 实例配置 |
| POST | `/api/settings/rsshub` | 保存 RSSHub 实例配置 |

### GET /api/settings/rsshub

```json
{ "success": true, "data": { "rsshub_base_url": "http://47.110.71.194:1200" } }
```

### POST /api/settings/rsshub

```json
{ "rsshub_base_url": "http://47.110.71.194:1200" }
```

> 实例地址存 `ai_settings.rsshub_config`，缺省回落 `DefaultRSSHubBaseURL=http://47.110.71.194:1200`。改一处全链路（目录同步 + 推荐订阅落地）生效。
