## 1. SSE 竞态修复

- [ ] 1.1 后端：`EvaluateStreamHandler` / `ScanStreamHandler` channel==nil 时阻塞等待（poll 200ms，超时 30s），不立即关闭连接
- [ ] 1.2 前端 `triggerEvaluate()`：先创建 EventSource 连接，再 POST 触发评估；POST 返回 409 时不关 SSE
- [ ] 1.3 前端 `triggerFullScan()`：同 1.2 模式，先 SSE 后 POST
- [ ] 1.4 验证：点击 AI 评估后前端立即显示进度，不再卡在"正在启动..."

## 2. 页面恢复 SSE

- [ ] 2.1 后端新增 `GET /api/topic-tags/merge-preview/status` 端点，返回 `{ scan_running, eval_running }`
- [ ] 2.2 前端 `loadGroups()` 前先调 status，如果运行中则显示进度条 + 重连对应 SSE
- [ ] 2.3 验证：评估期间切换页面再回来，进度条自动恢复

## 3. "按 AI 建议全选"按钮

- [ ] 3.1 header 区域新增醒目的"按 AI 建议全选"按钮（存在 AI 建议时显示），调用已有 `selectAllMergeable()`
- [ ] 3.2 无 `should_merge=true` 建议时按钮置灰
- [ ] 3.3 验证：点击后所有 AI 建议合并的建议被选中，用户可取消部分后点"合并选中"

## 4. 合并进度展示

- [ ] 4.1 `mergeSelected()` 执行时显示进度 "N/M 已合并"，每完成一个更新计数
- [ ] 4.2 验证：批量合并时能看到实时进度
