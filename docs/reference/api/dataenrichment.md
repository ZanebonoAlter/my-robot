# 话题数据增强 Data Enrichment

> 涵盖话题维度的生命周期上下文（lifeline_context）、增强结果（enrichment_result）、复盘评审（enrichment_review）、个股多空辩论（stock_debate），以及板块维度的数据源绑定（board_data_source）。
>
> 通用约定（响应信封、错误格式）见 [_conventions.md](_conventions.md)。

| 方法 | 路径 | 说明 |
| ------ | ------ | ------ |
| GET | `/api/persistent-topics/:topicId/enrichment/contexts` | 列出话题全部周期归档上下文（可按粒度过滤） |
| GET | `/api/persistent-topics/:topicId/enrichment/contexts/:granularity/:period` | 取指定粒度+周期的单条上下文 |
| PUT | `/api/persistent-topics/:topicId/enrichment/contexts/:granularity/:period` | 手动编辑某周期上下文内容 |
| POST | `/api/persistent-topics/:topicId/enrichment/contexts/:granularity/regenerate` | 重新生成某粒度上下文 |
| GET | `/api/persistent-topics/:topicId/enrichment/results` | 列出话题的增强结果（精简摘要） |
| GET | `/api/persistent-topics/:topicId/enrichment/results/:id` | 取单条增强结果（完整详情） |
| POST | `/api/persistent-topics/:topicId/enrichment/results/trigger` | 手动触发该话题的增强计算 |
| POST | `/api/persistent-topics/:topicId/enrichment/results/:id/debates` | 触发指定结果的多空辩论 |
| GET | `/api/persistent-topics/:topicId/enrichment/results/:id/debates` | 列出指定结果的辩论历史 |
| GET | `/api/persistent-topics/:topicId/enrichment/reviews` | 列出话题的复盘评审 |
| POST | `/api/persistent-topics/:topicId/enrichment/reviews` | 新建复盘评审 |
| PUT | `/api/persistent-topics/:topicId/enrichment/reviews/:id` | 更新评审的偏离摘要 |
| POST | `/api/persistent-topics/:topicId/enrichment/reviews/:id/apply` | 将评审标记为已应用 |
| GET | `/api/semantic-boards/:id/data-sources` | 列出板块绑定的数据源 |
| PUT | `/api/semantic-boards/:id/data-sources` | 新增/更新（upsert）板块数据源绑定 |
| DELETE | `/api/semantic-boards/:id/data-sources/:sourceType` | 删除指定类型的数据源绑定 |

> **路径参数命名**：本模块话题维度统一用 `:topicId`（驼峰，区别于其它模块常用的 `:topic_id` 下划线风格）；板块维度用 `:id`，与 board CRUD 一致。`:granularity` 取值固定为 `week` / `month` / `year` / `all`。

---

## 生命周期上下文 Contexts

### GET /api/persistent-topics/:topicId/enrichment/contexts

列出某话题的全部周期归档上下文。

**查询参数**

| 参数 | 必填 | 说明 |
|------|------|------|
| `granularity` | 否 | 按粒度过滤，取值 `week` / `month` / `year` / `all`；不传则返回全部粒度 |

**响应示例**

```json
{
  "success": true,
  "data": [
    {
      "id": 12,
      "persistent_topic_id": 7,
      "granularity": "week",
      "period": "2026-W27",
      "content": "本周话题演化摘要……",
      "source": "auto",
      "created_at": "2026-07-07T00:00:00Z",
      "updated_at": "2026-07-07T00:00:00Z"
    }
  ]
}
```

非法 `granularity` 返回 `400`。

### GET /api/persistent-topics/:topicId/enrichment/contexts/:granularity/:period

取指定粒度 + 周期的单条上下文。`period` 为必填路径段（如 `2026-W27`、`2026-07`、`2026`、`all`）。不存在时返回 `404`。

### PUT /api/persistent-topics/:topicId/enrichment/contexts/:granularity/:period

手动编辑某周期上下文的正文内容。更新后 `source` 会被置为 `manual`。

**请求体**

```json
{ "content": "手工修订后的上下文正文……" }
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `content` | 是 | 上下文正文，字符串 |

`content` 缺失返回 `400`；目标记录不存在返回 `404`。响应为更新后的上下文对象。

### POST /api/persistent-topics/:topicId/enrichment/contexts/:granularity/regenerate

重新生成某粒度的上下文。

**查询参数**

| 参数 | 必填 | 说明 |
|------|------|------|
| `period` | 否 | 指定周期（如 `2026-W27`）；不传则刷新该粒度当前周期（`RefreshGranularity`） |

响应为重新生成后的上下文对象。生成失败返回 `500`。

---

## 增强结果 Results

### GET /api/persistent-topics/:topicId/enrichment/results

列出该话题的增强结果（最新优先），返回精简摘要。

**响应示例**

```json
{
  "success": true,
  "data": [
    {
      "id": 101,
      "evolution_assessment": "话题持续升温，资金向龙头集中……",
      "sectors": [{ "sector": "原油", "symbols": [...] }],
      "tool_calls_count": 8,
      "session_id": "sess_abc123",
      "created_at": "2026-07-07T10:00:00Z"
    }
  ]
}
```

> `evolution_assessment` 超过 200 字符会被截断并补 `...`；`sectors` 为解析后的 JSON 结构。

### GET /api/persistent-topics/:topicId/enrichment/results/:id

取单条增强结果的完整详情。带 IDOR 校验：若结果不属于该 `topicId` 返回 `404`。

**响应示例**

```json
{
  "success": true,
  "data": {
    "id": 101,
    "persistent_topic_id": 7,
    "evolution_assessment": "完整评估正文……",
    "sectors": [{ "sector": "原油", "symbols": [...] }],
    "causal_chain": "因果链描述……",
    "tool_calls": [...],
    "input_snapshot": {...},
    "session_id": "sess_abc123",
    "created_at": "2026-07-07T10:00:00Z"
  }
}
```

### POST /api/persistent-topics/:topicId/enrichment/results/trigger

手动触发该话题的增强计算（cycle-B）。触发前会校验所属板块是否启用增强（`enrichment_enabled`），未启用返回 `400`。

**响应示例**

```json
{
  "success": true,
  "data": {
    "result": {
      "id": 102,
      "evolution_assessment": "……",
      "sectors": [...],
      "causal_chain": "……",
      "tool_calls_count": 6,
      "session_id": "sess_def456",
      "created_at": "2026-07-07T11:00:00Z"
    },
    "review_generated": true
  }
}
```

增强失败返回 `500`。

### POST /api/persistent-topics/:topicId/enrichment/results/:id/debates

对指定增强结果触发个股多空辩论。带 IDOR 校验。

**请求体**（可选）

```json
{
  "symbols": [
    { "code": "161129", "name": "易方达原油", "sector": "原油" }
  ]
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `symbols` | 否 | 辩论标的列表；缺省或为空时自动从结果 `sectors.symbols` 提取 |

当 `symbols` 为空且结果内无可用标的时返回 `400`。

**响应示例**

```json
{
  "success": true,
  "data": { "debates": [ { /* StockDebateResult */ } ] }
}
```

### GET /api/persistent-topics/:topicId/enrichment/results/:id/debates

列出指定结果的历史辩论记录。

---

## 复盘评审 Reviews

### GET /api/persistent-topics/:topicId/enrichment/reviews

列出该话题的复盘评审（最新优先）。

### POST /api/persistent-topics/:topicId/enrichment/reviews

手动新建一条评审标注。新建的评审 `applied=true`、`source="manual"`。

**请求体**

```json
{
  "curr_result_id": 102,
  "deviation_summary": "本期与上期的偏离说明……",
  "prev_result_id": 101
}
```

| 字段 | 必填 | 说明 |
| ------ | ------ | ------ |
| `curr_result_id` | 是 | 当前结果 ID（uint） |
| `deviation_summary` | 是 | 偏离摘要，字符串 |
| `prev_result_id` | 否 | 上一期结果 ID（uint 指针，可空） |

`curr_result_id` / `deviation_summary` 缺失返回 `400`。响应为新建的评审对象。

### PUT /api/persistent-topics/:topicId/enrichment/reviews/:id

更新某条评审的偏离摘要。

**请求体**

```json
{ "deviation_summary": "修订后的偏离说明……" }
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `deviation_summary` | 是 | 偏离摘要，字符串 |

缺失返回 `400`。响应为更新后的评审对象。

### POST /api/persistent-topics/:topicId/enrichment/reviews/:id/apply

将评审标记为已应用（`applied=true`）。按设计不回写到上下文表（lifeline_context）。响应为更新后的评审对象。

---

## 板块数据源 Data Sources

### GET /api/semantic-boards/:id/data-sources

列出某板块绑定的全部数据源。

### PUT /api/semantic-boards/:id/data-sources

新增或更新（upsert）某板块的一个数据源绑定。`enabled` 缺省为 `true`。

**请求体**

```json
{
  "source_type": "etf_quote",
  "config": { "symbol": "161129", "interval": "1d" },
  "enabled": true
}
```

| 字段 | 必填 | 说明 |
| ------ | ------ | ------ |
| `source_type` | 是 | 数据源类型标识，字符串 |
| `config` | 否 | 数据源配置，任意 JSON 对象 |
| `enabled` | 否 | 是否启用，布尔；不传按 `true` 处理 |

`source_type` 缺失返回 `400`。响应为写入后的数据源对象。

### DELETE /api/semantic-boards/:id/data-sources/:sourceType

删除指定类型的数据源绑定。目标不存在返回 `404`。

**响应示例**

```json
{ "success": true, "data": { "deleted": true } }
```
