# Tasks: agent-quota-gate

## 1. 扩展骨架与 provider 解析

- [x] 1.1 新建 `.pi/extensions/quota-gate.ts`：注册 `pi.on("tool_call")`，仅 `event.toolName === "Agent"` 时介入，其余工具零开销返回
- [x] 1.2 实现 provider 解析：`provider/modelId` 全称直取；省略 `model` 取 `ctx.model` 的 provider；裸 modelId 用 `ctx.modelRegistry` 按字母序解析并标注 fuzzy 风险
- [x] 1.3 认证获取走 `ctx.modelRegistry.getProviderAuth(providerId)`，不读 `auth.json`

## 2. 额度适配器

- [x] 2.1 定义统一返回类型 `QuotaStatus = { kind: "ok"|"low"|"exhausted"|"unknown"; summary: string; resetHint?: string }`
- [x] 2.2 zai-coding-cn 适配器：`GET https://open.bigmodel.cn/api/monitor/usage/quota/limit`（`Authorization: <key>`，带浏览器 UA），解析 `data.limits[]` 的 5h/周 `TOKENS_LIMIT` 百分比与 `nextResetTime`
- [x] 2.3 kimi-coding 适配器：`GET https://api.kimi.com/coding/v1/usages`（`Authorization: Bearer <key>`），解析周窗口与 300 分钟窗口百分比及重置时间
- [x] 2.4 deepseek 适配器：`GET https://api.deepseek.com/user/balance`（Bearer），`is_available=false` 或余额 < 阈值 → `exhausted`
- [x] 2.5 无适配器 provider（opencode-go 等）：放行 + 每会话一次提示（`ctx.ui` 或日志）

## 3. 判定、缓存与阻断

- [x] 3.1 阈值常量集中扩展顶部：窗口百分比阈值默认 10%、DeepSeek 余额阈值默认 1 元，支持环境变量覆盖（如 `QUOTA_GATE_WINDOW_PCT`、`QUOTA_GATE_MIN_BALANCE`）
- [x] 3.2 内存缓存：per-provider `{ status, fetchedAt }`，TTL 默认 3 分钟（`QUOTA_GATE_TTL_MS` 可调）；同 provider 并发查询共享 in-flight Promise
- [x] 3.3 请求守卫：5s 超时、浏览器 UA、全链路 try/catch → 异常一律 `unknown`
- [x] 3.4 阻断逻辑：`low`/`exhausted` 返回 `{ block: true, reason }`；reason 中文，含 provider、剩余情况、重置时间、建议动作；`ok`/`unknown` 放行

## 4. 联调验证

- [x] 4.1 手工验证三个端点真实返回（用本地真 key 各调一次，确认字段解析正确）
- [ ] 4.2 重启 pi 会话，确认扩展加载无报错；发起一次 Agent 派发观察放行路径
- [ ] 4.3 模拟低额度（临时把阈值调到 100%）验证 block reason 被主线程正确接收并能换模型重试

## 5. 测试

本 change 为 pi 扩展（TypeScript，非产品代码），无单元测试基建；验证以手工联调 + lint 为准。

- [x] 5.1 `npx tsc --noEmit`（或 pi 加载时类型检查）通过，扩展无类型错误
- [x] 5.2 任务 4.1-4.3 的联调结果记录在 change 目录 `verify-notes.md`

## 6. 文档

<!-- doc-impact: standard -->

- [x] 6.1 AGENTS.md「子线程派发参考」节补一段：额度门禁存在、阻断时如何换模型重试
- [x] 6.2 `docs/reference/开发执行规范.md` §4.1 门禁分层处补一行 quota-gate 说明（与 quality-gate 并列）

## 7. 验证

- [x] 7.1 `node --experimental-strip-types -e "await import('./.pi/extensions/quota-gate.ts')"` 或重启 pi → 期望：扩展加载无异常
- [ ] 7.2 派发一次 `Agent`（简单任务，model 用 `deepseek/deepseek-v4-flash`）→ 期望：放行，日志可见额度摘要
- [ ] 7.3 `QUOTA_GATE_WINDOW_PCT=100` 重启后再派发 → 期望：Agent 调用被 block，reason 含 provider/剩余/重置时间/建议
- [ ] 7.4 `bash scripts/doc-impact.sh verify` → 期望：文档节声明与实际改动一致（当前受 localize-icons 脏工作区干扰 FAIL，待其提交后重跑，见 verify-notes.md）
