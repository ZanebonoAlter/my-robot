## Context

tag-merge 功能已有完整的 LLM 评估 + 分组展示 + 勾选合并流程。当前交互：全量扫描 → AI 评估 → 手动勾选 → 批量合并。

### SSE 竞态时序

```
当前（有 bug）：
  前端 POST /evaluate ──▶ StartEvaluation() go runEvaluation()
  ◀── 202
  前端 new EventSource(/stream) ──▶ handler: channel 可能已有数据 / 已关闭

修复后：
  前端 new EventSource(/stream) ──▶ handler: channel==nil → 阻塞等待
  前端 POST /evaluate ──▶ StartEvaluation() → channel 创建 → goroutine 写数据
  ◀── SSE events 自然到达
```

## Goals / Non-Goals

**Goals:**
- SSE 评估/扫描进度正常推送到前端
- "按 AI 建议全选"一键选中，用户审核后合并
- 合并过程显示进度（N/M 已合并）
- 页面离开后恢复 SSE 连接

**Non-Goals:**
- 不改 LLM 评估核心逻辑
- 不改合并执行逻辑（HardMergeTags）
- 不做撤销/回滚
- 不做合并方向调换（从源头优化方向选择，留到未来）

## Decisions

### D1: "按 AI 建议全选" — 仅选中，不自动合并

按钮调用已有的 `selectAllMergeable()`，选中所有 `should_merge=true` 的建议。用户看到选中结果后仍可取消不想要的，再点"合并选中"。

理由：保留人工审核环节，避免 AI 误判导致错误合并。不需要新函数，`selectAllMergeable()` 已存在。

### D2: SSE 竞态修复 — 前端先 SSE 后 POST + 后端阻塞等待

前端改动：
1. `triggerEvaluate()` / `triggerFullScan()` 先创建 EventSource，再 POST 触发
2. POST 返回 409（已有运行中）时不关 SSE，继续接收旧进度
3. POST 失败（非 409）时关闭已创建的 EventSource

后端改动：
1. `EvaluateStreamHandler` / `ScanStreamHandler`：当 channel==nil 时阻塞等待（轮询间隔 200ms，超时 30s），而非发 idle 立即关闭
2. 这样前端先连 SSE 时，handler 会等待直到 channel 被创建

### D3: 合并进度展示

`mergeSelected()` 改为显示进度：新增 `mergeProgress` ref，每完成一个合并更新 "N/M 已合并"。不需要新后端端点。

### D4: 页面恢复 — 单一 status 端点

后端新增 `GET /api/topic-tags/merge-preview/status`，返回：
```json
{ "scan_running": bool, "eval_running": bool }
```

前端 `onMounted` / `watch(visible)` 时先调 status：
- `eval_running=true` → 显示进度条 + 重连 eval SSE
- `scan_running=true` → 显示进度条 + 重连 scan SSE
- 都 false → 正常加载分组

### D5: 双向重复数据 — 不处理

遗留数据，不影响功能。全量扫描已按 `LEAST/GREATEST` 标准化方向，新数据不会重复。
