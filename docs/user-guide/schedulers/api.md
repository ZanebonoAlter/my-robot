# 定时任务 Schedulers

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/schedulers/status` | 所有调度器状态 |
| GET | `/api/schedulers/:name/status` | 指定调度器状态 |
| POST | `/api/schedulers/:name/trigger` | 手动触发 |
| POST | `/api/schedulers/:name/reset` | 重置统计 |
| PUT | `/api/schedulers/:name/interval` | 更新间隔 |

---

### 支持的调度器

| 名称 | 说明 |
|------|------|
| `auto_refresh` | 自动刷新 RSS |
| `preference_update` | 更新阅读偏好 |
| `content_completion` | 文章内容补全（别名 `ai_summary`） |
| `firecrawl` | Firecrawl 全文抓取 |
| `tag_quality_score` | 重算标签质量分数 |
| `log_cleanup` | 清理过期的 AI 调用日志和追踪数据 |
| `daily_report` | 生成每日报告 |
| `aux_label_cleanup` | 清理无活跃标签引用的辅助标签 |
| `blocked_article_recovery` | 恢复被阻塞的文章 |

### `last_execution_result` 字段说明

各调度器的 `last_execution_result` JSON 包含以下计数字段：

| 调度器 | 字段 | 说明 |
|--------|------|------|
| `log_cleanup` | `last_ai_call_logs_deleted` | 删除的 AI 调用日志条数 |
| `log_cleanup` | `last_otel_spans_deleted` | 删除的 OpenTelemetry 追踪 span 条数 |
| `aux_label_cleanup` | `affected_count` | 清理的辅助标签数 |
| `blocked_article_recovery` | `recovered_count` | 恢复的文章数 |
| `firecrawl` | `completed` | 成功处理的任务数 |
| `firecrawl` | `failed` | 失败的任务数 |
| `daily_report` | `report_count` | 生成的日报数 |
| `preference_update` | `updated_count` | 更新的偏好数 |
| `auto_refresh` | `triggered_feeds` | 刷新的订阅源数 |

### 配置项

| 配置键 | 说明 | 默认值 |
|--------|------|--------|
| `daily_report_time` | 日报生成时刻（HH:MM） | `21:00` |

日报时刻可通过 `PUT /api/settings` 保存，读取时非法值回退默认值。

### GET /api/schedulers/status

返回所有已注册调度器的状态列表。每个调度器包含：

```json
{
  "name": "content_completion",
  "status": "running",
  "check_interval": 300,
  "next_run": 1710000000,
  "is_executing": false,
  "description": "Complete article content and generate article summaries",
  "database_state": { ... },
  "overview": { ... },
  "last_run_summary": { ... }
}
```

### GET /api/schedulers/:name/status

返回单个调度器状态，同上结构。`404` 表示调度器不存在。

### POST /api/schedulers/:name/trigger

手动触发调度器。部分调度器支持 `?date=YYYY-MM-DD` 查询参数。

触发成功时返回执行结果或任务状态；调度器正忙时返回 `409`。

### POST /api/schedulers/:name/reset

重置调度器的统计信息（执行次数、错误计数等）。

### PUT /api/schedulers/:name/interval

```json
{ "interval": 30 }
```

`interval`：正整数，单位取决于调度器（一般为秒）。返回更新后的 `name` 和 `check_interval`。
