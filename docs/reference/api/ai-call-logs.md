# AI 调用日志 Call Logs

AI 调用业务日志（`ai_call_logs` 表，`models.AICallLog`）。按 session 回放编排、按 operation 分组查询。与 [`/api/traces`](traces.md)（OTel 链路 span）正交：本端点记业务侧（prompt/token/operation/session），traces 记代码侧（span 树/耗时）。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ai/call-logs` | AI 调用日志查询 |

## GET /api/ai/call-logs

查询参数（全部可选，组合过滤）：

| 参数 | 类型 | 说明 |
|------|------|------|
| `operation` | string | 业务操作名精确匹配（如 `daily_report.cluster_tags`） |
| `session_id` | string | 编排分组键精确匹配；非空时结果按 `created_at` **升序**（回放编排顺序） |
| `capability` | string | airouter capability 精确匹配（如 `embedding`） |
| `from` | ISO8601 | `created_at >= from` |
| `to` | ISO8601 | `created_at <= to` |
| `limit` | int | 默认 `50`，上限 `200`（超过截断为 200，不报错） |
| `offset` | int | 分页偏移，默认 `0` |

**排序**：`session_id` 非空时按 `created_at` 升序（编排回放）；否则降序（最近在前）。

**响应**：`{"success": true, "data": [...]}`，每条含：

| 字段 | 说明 |
|------|------|
| `id` / `operation` / `session_id` / `capability` | 标识与分组 |
| `route_name` / `provider_name` / `model` | 路由与实际调用方 |
| `success` / `latency_ms` / `error_code`*(失败时)* | 结果 |
| `token_usage` | `{prompt, completion, total}` 的 jsonb |
| `prompt` | 完整 messages 截断预览（前 500 runes + `...`） |
| `response_snippet` | 响应摘要 |
| `created_at` | 时间 |

> **按 session 回放编排**：`GET /api/ai/call-logs?session_id=daily_report_<board_id>_<uuid8>` 返回该日报编排期间全部 LLM/embedding 调用，按时间正序，可重建一次编排的全过程。
