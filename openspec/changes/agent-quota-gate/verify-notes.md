# verify-notes: agent-quota-gate

## 4.1 端点实测（2026-08-01，真 key，deepseek-v4-flash 子线程执行）

三端点均 HTTP 200，实际响应结构（与 design 预估的差异已回写适配器注释）：

- **zai-coding-cn**：`data.limits[]` 中仅 `type==="TOKENS_LIMIT"` 是 LLM 额度窗口（5h；带周限制的新套餐多一条周窗口，按 `nextResetTime` 升序区分）；`TIME_LIMIT` 是 MCP 工具用量（`percentage=100` 也与 LLM 派发无关），一律忽略。老套餐（lite）无周限制仅一条 TOKENS_LIMIT。⚠️ 首轮实现曾把 TIME_LIMIT 误判为周配额（用户指正后修正）。`percentage` 数字/字符串均有出现，统一 Number 转换；窗口未激活时无 `nextResetTime`。
- **kimi-coding**：顶层 `usage` = 周额度（`limit/used/remaining/resetTime`，字符串数字，resetTime 为 ISO）；`limits[]` 中 `window.duration === 300` 的 `detail` 为 5h 窗口。
- **deepseek**：`is_available: boolean` + `balance_infos[0].total_balance`（字符串金额，如 "42.63"）。

## 4.2 / 4.3 联调（待 pi 重启后由用户/主线程验证）

- [ ] 4.2 重启 pi → 扩展加载无报错；发起一次 Agent 派发观察放行路径
- [ ] 4.3 `QUOTA_GATE_WINDOW_PCT=100` 重启 → Agent 调用被 block，reason 含 provider/剩余/重置/建议

## 5.1 类型与加载检查（已跑）

- 子线程：strict `tsc --noEmit` 零错误（独立类型环境，GateCtx 结构子集）
- 主线程：`node --experimental-strip-types -e "await import('./.pi/extensions/quota-gate.ts')"` → `load OK`（2026-08-01）
- 主线程真 key 链路冒烟（TIME_LIMIT 修正后）：GLM 派发 → `ok | 5h窗口剩 100%` ALLOWED；缓存命中一致；非 Agent 工具零开销（2026-08-01）
- 子线程冒烟 9 条路径全过：非 Agent 零网络、缓存命中零请求（网络计数验证）、fuzzy 落 opencode-go 告警、未知 provider 无 key fail-open、阈值 100% 时三家全部 block 且 reason 字段齐全

## 7.x 验证记录

- [x] 7.1 strip-types 加载：通过（见 5.1）
- [ ] 7.2 实派发放行路径：待 pi 重启
- [ ] 7.3 阈值 100% 阻断路径：待 pi 重启
- [!] 7.4 `bash scripts/doc-impact.sh verify openspec/changes/agent-quota-gate`：**FAIL 但非本 change 问题**（2026-08-01）——verify 基于工作区 diff，并行会话 localize-icons 的未提交改动（`backend-go/internal/platform/database/*`）被算进来，报「疑似遗漏 database」。本 change 实际只动 `.pi/extensions/` + `AGENTS.md` + `docs/reference/开发执行规范.md`（standard 域），声明无误。待工作区干净（localize-icons 提交/归档）后重跑。
