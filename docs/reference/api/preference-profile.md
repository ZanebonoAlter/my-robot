# 偏好向量画像（Preference Profile）

> 偏好画像 = 按 SemanticBoard 聚合的 embedding 向量（替代旧 `user_preferences` 偏好分数）。`reading_behaviors` 为权重源，scheduler 定期重算（零 LLM），问答提问写种子偏好（冷启动）。链路与业务约束见 [../flow/discovery.md](../flow/discovery.md)。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/preference-profile` | 兴趣画像（各版块 top 标签 + 权重 + 来源 + 最后计算时间） |
| POST | `/api/preference-profile/recompute` | 手动触发重算（与 `preference_profile_update` scheduler 同一路径） |

### GET /api/preference-profile

返回各版块（含全局桶 `board_id=null`）的偏好画像，按版块分组。空数据返回空列表。

```json
{
  "success": true,
  "data": [
    {
      "board_id": "12",
      "board_label": "AI 芯片",
      "source": "behavior",
      "tag_weights": { "AI": 0.42, "芯片": 0.31, "制程": 0.18 },
      "dimension": 4096,
      "model": "bge-m3",
      "last_computed_at": "2026-07-25T10:00:00Z"
    },
    {
      "board_id": null,
      "board_label": "全局",
      "source": "seed",
      "tag_weights": { "半导体": 0.5 },
      "dimension": 4096,
      "model": "bge-m3",
      "last_computed_at": "2026-07-25T11:00:00Z"
    }
  ]
}
```

> `source`：`behavior`（scheduler 全量重算产出）/ `seed`（问答兴趣表达加权合并累积）。约束见 [../flow/discovery.md](../flow/discovery.md) §业务约束与不变量。

### POST /api/preference-profile/recompute

手动触发偏好画像重算。返回调度/执行结果摘要。

```json
{ "success": true, "message": "preference profile recomputed" }
```

> 重算幂等、全量重建 `source=behavior` 行，不覆盖 `source=seed` 行。
