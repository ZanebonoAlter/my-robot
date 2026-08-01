# Design: agent-quota-gate

## Context

pi 的子线程派发由 pi-subagents 包注册的 `Agent` 工具完成。pi 扩展系统提供 `tool_call` 事件钩子：在工具执行前触发，返回 `{ block: true, reason }` 可阻断执行，且 `event.input` 可变（可改写工具参数）。项目已有 `.pi/extensions/quality-gate.ts`（挂 `turn_end`）作为项目级扩展先例。

调研确认三家主力供应商的额度端点（认证均复用现有 API key）：

| Provider | 端点 | 额度模型 |
|---|---|---|
| zai-coding-cn | `GET https://open.bigmodel.cn/api/monitor/usage/quota/limit`，`Authorization: <key>` | `data.limits[]`：仅 `type==="TOKENS_LIMIT"` 是 LLM 额度窗口（5h；带周限制的新套餐多一条周窗口）；`TIME_LIMIT` 是 MCP 用量，不参与判定。老套餐（lite）无周限制仅一条 |

> 实测修正（2026-08-01）：子线程首轮把 `TIME_LIMIT percentage=100` 误判为「周配额用尽」，实为 MCP 用量条；老套餐用户无周限制。适配器只认 `TOKENS_LIMIT`。
| kimi-coding | `GET https://api.kimi.com/coding/v1/usages`，`Authorization: Bearer <key>` | 周额度 % + 300 分钟窗口 % + 重置时间 |
| deepseek | `GET https://api.deepseek.com/user/balance`，`Authorization: Bearer <key>`（官方文档） | 余额金额（`is_available` + `balance_infos[]`） |

opencode-go 无官方 API（只能 cookie 刮页），本版不做。

## Goals / Non-Goals

**Goals:**

- 派发子线程前检测目标 provider 额度，不足则阻断并给出可操作的 reason（哪家、剩多少、何时重置）。
- 查询开销可控：内存缓存 + 短 TTL，连续派发不重复打端点。
- 额度接口故障时工作流不受损（fail-open）。
- 阈值可配置，默认保守。

**Non-Goals:**

- 不做自动降级（改写 `event.input.model` 换 provider）——二期再说，一期只 block。
- 不做 opencode-go 及其他无 API 供应商的额度查询（放行 + 一次性提示）。
- 不防"子线程运行中途烧光额度"——那是 provider 侧行为，hook 只能管派发这一刻。
- 不改动产品前后端代码，不新增密钥/配置存储。

## Decisions

### D1：挂 `tool_call` 事件，只处理 `Agent` 工具

`event.toolName === "Agent"` 时介入，其余工具调用零开销直接放行。阻断用 `{ block: true, reason }`，reason 喂回主线程 LLM，它可据此换模型重试——与 AGENTS.md「任务→模型对照表」自然衔接。

备选：挂 `turn_start` 预检所有 provider——误伤（不派发也查）且无法对应具体派发的 provider，弃。

### D2：provider 解析规则与 AGENTS.md 硬规则对齐

- `event.input.model` 为 `provider/modelId` 全称 → 直接取 provider。
- 省略 `model` → 用会话默认 provider（`ctx.model` 的 provider）。
- fuzzy 名（裸 modelId）→ 用 `ctx.modelRegistry` 按 pi 同款字母序规则解析，并在 reason/日志里提示这是危险写法（呼应 AGENTS.md 的 fuzzy 名坑）。

key/baseUrl 一律走 `ctx.modelRegistry.getProviderAuth(providerId)`，不自己读 `auth.json`。

### D3：per-provider 适配器 + 统一判定模型

每个适配器输出统一结构：

```ts
type QuotaStatus = {
  kind: "ok" | "low" | "exhausted" | "unknown";
  summary: string;        // 中文人话，如 "5h窗口剩 8%，每周剩 42%，5h窗口 17:00 重置"
  resetHint?: string;     // 最近的重置时间描述
};
```

- GLM/Kimi：取 5h 窗口与周窗口两个百分比，任一 < 阈值 → `low`/`exhausted`（5h 窗口见底但周窗口有余 → `low`，reason 提示等重置；双见底 → `exhausted`）。
- DeepSeek：`is_available=false` 或余额 < 阈值（金额，默认 1 CNY）→ `exhausted`。
- 查询失败/超时/被反爬 → `unknown`，fail-open 放行并在 reason 级别记 warning（不阻断）。

阈值：5h/周窗口百分比默认 10%，DeepSeek 余额默认 1 元；通过扩展模块顶部常量配置（项目自用，不做 UI 设置）。

### D4：内存缓存 + 请求守卫

- 每 provider 一条缓存：`{ status, fetchedAt }`，TTL 默认 3 分钟（环境变量 `QUOTA_GATE_TTL_MS` 可调）。
- 单 provider 并发去重（in-flight Promise 复用），避免一条消息里多个 Agent 调用并发打同一端点。
- 请求带浏览器 UA（GLM 反爬）、5s 超时。

### D5：opencode-go 等无适配器 provider 的处理

放行，且每个会话只提示一次（避免刷屏）。后续若社区方案成熟再补适配器。

## Risks / Trade-offs

- [GLM 端点反爬拦截，返回非 JSON/403] → UA + 失败即 `unknown` fail-open；不因此阻断派发。
- [tool_call 里发 HTTP 增加派发延迟] → 缓存 TTL 3 分钟 + 5s 超时上限；命中缓存时零网络开销。
- [额度数据是窗口百分比，无法精确预测一次派发的消耗] → 阈值保守（10%），本质是"快没了别派"而非精确预算。
- [扩展报错导致 Agent 工具被误阻断] → 适配器全部 try/catch 兜底为 `unknown`；block 只发生在明确 `low/exhausted` 时。
- [子线程嵌套派发（子 Agent 再派 Agent）也被拦] → 预期行为，项目扩展默认加载到子会话，嵌套派发同样需要额度。

## Migration Plan

纯新增文件，无迁移。回滚 = 删除 `.pi/extensions/quota-gate.ts`。

## Open Questions

- 二期自动降级的目标顺序是否就按 AGENTS.md「任务→模型对照表」+ 额度排序？（一期不实现）
- `xiaomi-token-plan-cn`、`zai`（国际站）等长尾 provider 是否需要适配器？（默认走 D5 放行提示）
