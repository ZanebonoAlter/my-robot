## Why

tag-merge 功能已上线基本流程（LLM 批量评估、分组展示、勾选合并），但 UX 有明确缺陷影响可用性：

1. **SSE 评估进度推送失效**：点击"AI 评估"后前端一直卡在"正在启动..."，SSE EventSource 未正常接收后端推送事件。前端 POST 触发后才建 SSE 连接，goroutine 已开始发事件导致竞态丢失。
2. **缺少快速全选 AI 建议的入口**：用户必须逐个勾选，当有几十个建议时操作繁琐。需要一个醒目的"按 AI 建议全选"按钮，选中后用户仍可审核调整，再点"合并选中"。
3. **页面离开后 SSE 连接丢失**：评估或扫描期间用户切换页面再回来，SSE EventSource 已断开，进度信息丢失。

## What Changes

- 修复 SSE 竞态：前端先建 EventSource 再 POST 触发，后端 channel==nil 时阻塞等待而非立即关闭
- 新增"按 AI 建议全选"按钮：醒目位置一键选中所有 `should_merge=true` 的建议，用户审核后点"合并选中"执行
- 合并进度展示：`mergeSelected()` 执行时显示 "N/M 已合并" 进度
- 页面恢复 SSE：后端新增 `GET /merge-preview/status` 端点，返回 scan/eval 运行状态，前端回来后自动重连

## Capabilities

### Modified Capabilities

- `tag-merge-ui`: SSE 竞态修复（前后端），页面恢复，"按 AI 建议全选"按钮，合并进度展示

## Impact

- **前端**: `TagMergePreview.vue` — SSE 连接时序调整 + "按 AI 建议全选"按钮 + 合并进度 + 页面恢复
- **后端**: `tag_merge_suggest.go` — SSE handler 阻塞等待 channel，新增 status 端点
- **后端**: `tag_merge_preview_handler.go` — 新增 status handler + 路由注册
