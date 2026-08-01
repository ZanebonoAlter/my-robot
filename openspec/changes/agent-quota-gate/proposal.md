# Proposal: agent-quota-gate

## Why

主线程派发子线程（Agent 工具）时无法预知目标供应商的剩余额度，经常出现子线程执行到一半额度耗尽、任务烂尾的情况。调研已确认 pi 的 `tool_call` 扩展钩子可以在派发前阻断/改写工具调用，且 zai-coding-cn / kimi-coding / deepseek 三家都有可程序化查询的额度端点，具备落地条件。

## What Changes

- 新增 pi 扩展 `.pi/extensions/quota-gate.ts`：挂 `tool_call` 事件，仅拦截 `Agent` 工具调用。
- 派发前解析 `model` 参数 → provider（省略时取默认 provider），查询该 provider 剩余额度。
- 额度不足时 `block` 派发并返回中文 reason（哪家没额度、何时重置、可换谁），主线程收到后可换模型重试。
- 内置 per-provider 额度适配器：GLM Coding Plan（5h/周窗口百分比）、Kimi Coding（5h/周窗口百分比）、DeepSeek（余额）；opencode-go 无 API，第一版跳过（放行并一次性提示）。
- 额度结果内存缓存（短 TTL），避免每次派发都发 HTTP；查询失败 fail-open（放行 + 警告），不因额度接口故障阻断工作流。
- 阈值可配置（如 5h 窗口剩余 <10% 视为不可派发），默认保守。

## Capabilities

### New Capabilities

- `agent-quota-gate`: 子线程派发前的供应商额度预检——拦截 Agent 工具调用、查询供应商额度端点、阈值判定、阻断/放行策略、缓存与降级行为。

### Modified Capabilities

（无）

## Impact

- **代码**：新增 `.pi/extensions/quota-gate.ts`（唯一新文件）；不改任何产品前后端代码。
- **外部依赖**：调用三个供应商的额度查询 HTTP 端点（GLM `open.bigmodel.cn/api/monitor/usage/quota/limit`、Kimi `api.kimi.com/coding/v1/usages`、DeepSeek `api.deepseek.com/user/balance`），认证复用 `ctx.modelRegistry.getProviderAuth()` 的现有 key，不新增密钥配置。
- **文档**：`docs/reference/standard/` 或 AGENTS.md 补一节「额度门禁」说明；`.pi/extensions/` 已有 quality-gate.ts 同款模式可循。
- **风险**：GLM 端点有反爬，需带 UA + 失败降级；额度查询增加派发延迟（缓存 TTL 控制在 1-5 分钟）。
