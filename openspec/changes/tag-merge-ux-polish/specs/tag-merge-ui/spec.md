## ADDED Requirements

### Requirement: SSE 评估进度推送正常工作

点击"AI 评估"后前端能实时收到后端推送的评估进度事件。

#### Scenario: 前端先建 SSE 连接再触发评估
- **WHEN** 用户点击"AI 评估"
- **THEN** 前端先创建 EventSource 连接 `/evaluate/stream`，再 POST `/evaluate` 触发评估

#### Scenario: 评估已完成时连接 SSE
- **WHEN** 前端连接 SSE 时后端 goroutine 还没启动（channel==nil）
- **THEN** 后端 SSE handler 阻塞等待 channel 创建，不立即关闭连接

#### Scenario: 评估完成后 SSE 关闭
- **WHEN** 后端推送 `status: "done"` 或 `status: "error"`
- **THEN** 前端关闭 EventSource，自动刷新分组列表

#### Scenario: 评估已在运行时点击 AI 评估
- **WHEN** 用户点击"AI 评估"但已有评估在运行
- **THEN** POST 返回 409，前端不关闭已建立的 SSE 连接，继续接收进度

### Requirement: 页面离开后恢复 SSE 连接

评估或扫描期间用户切换页面再回来，自动恢复进度显示。

#### Scenario: 评估进行中离开后返回
- **WHEN** 评估正在运行，用户离开合并预览页面再回来
- **THEN** 前端调用 `GET /merge-preview/status` 查询状态，如果 `eval_running=true`，自动重连 SSE 并显示进度

#### Scenario: 扫描进行中离开后返回
- **WHEN** 全量扫描正在运行，用户离开再回来
- **THEN** 状态查询返回 `scan_running=true`，前端重连 scan SSE 并显示进度

#### Scenario: 已完成后返回
- **WHEN** 评估或扫描已完成，用户回来
- **THEN** 状态查询返回都非 running，前端正常加载分组数据

### Requirement: "按 AI 建议全选"按钮

提供醒目的一键选中所有 AI 建议合并的入口。

#### Scenario: 点击按 AI 建议全选
- **WHEN** 用户点击"按 AI 建议全选"按钮
- **THEN** 系统自动选中所有 `should_merge=true` 的建议（调用 `selectAllMergeable()`），用户可继续取消不想要的

#### Scenario: 无可合并建议
- **WHEN** 没有任何 `should_merge=true` 的建议
- **THEN** 按钮置灰不可点击

### Requirement: 合并进度展示

批量合并时显示实时进度。

#### Scenario: 合并过程中显示进度
- **WHEN** 用户点击"合并选中"执行批量合并
- **THEN** 显示进度 "N/M 已合并"，每完成一个合并更新计数，完成后自动刷新分组列表
