# tool-output-spill 设计

## Context

见 proposal.md（Why：三层防线全靠自觉+截断即丢；C12 判定：`tool_result` 入口替换零缓存代价）。pi 事实（实测 extensions.md + 本仓先例）：`tool_result` 事件 `{toolName, toolCallId, input, content, details, isError, usage}`，handler 返回 partial patch（`content` 等），middleware 链按扩展加载序；sessionId 用 `ctx.sessionManager?.getSessionId?.()`（harness-telemetry.ts:103 同款先例）。

## Goals / Non-Goals

**Goals**：强制保险丝（超阈值无条件 spill，不依赖自觉）；spill 文件与 events.db 同体系（30 天对齐）；`spill.write` 记账可聚合。

**Non-Goals**：不做存量历史剪枝（C9 已判不抄）；不做语义摘要（只保头尾）；不处理图片块；不引入 Redis/队列。

## Decisions

### D1 钩子与覆盖面：toolName 无关

handler 不按工具名过滤——pi 内置工具（bash/read/write）与 MCP 工具（playwright 等）的结果走同一条 `tool_result` 链，无关注写法天然全覆盖；烟测用一个真实 MCP 工具结果验证此假设（若 MCP 不走此链，记录实际覆盖面并降级为内置工具覆盖）。

### D2 阈值与配置

默认 32768 字节（32KB），读 `.pi/harness.json` 的 `spillThresholdBytes`（可选，文件不存在用默认；非正数禁用 spill 直通）。32KB 依据：pi read 截断上限 50KB、bash 2000 行（~100-200KB）之间的中间档；单次 spill 至少省 ~8K token 才值得付出一次 read 取回的交互成本。

### D3 预览格式：头 2048 + 尾 512 字节

```
[spill] 完整输出 83968 字节已存 .pi/harness/spill/<sid>/<file>，read 可取回
--- 头部 2048 字节 ---
<head>
--- 尾部 512 字节 ---
<tail>
```

头部大于尾部（文件头/命令回显信息密度高）；字节切片遇 UTF-8 多字节边界回退到合法边界（中文内容不产生乱码字节）。

### D4 文件布局与命名

`.pi/harness/spill/<sessionId>/<seq>-<toolName>.txt`（seq 用毫秒时间戳，toolName 过滤非法文件字符）。会话目录隔离，取回路径自含上下文。

### D5 生命周期：挂 session_start 清扫

扩展加载/会话启动时扫 `.pi/harness/spill/` 删 30 天前文件（fs stat mtime 判龄，与 events.db 开库清扫同时机、同周期，但不耦合其实现——spill 清扫失败不影响 events.db）。

### D6 失败安全

写盘 try/catch：失败则原结果原样返回 + `spill.write` 事件记 `{ok:false, error}`（spec 场景要求）；spill 机制自身故障 MUST NOT 阻塞工具链。

### D7 防 read-spill 死循环（探索期新发现）

agent 按 preview 指引 read spill 文件取回完整内容——若该 read 结果仍超阈值会被再 spill，形成「read→spill→read」死循环。解法：`toolName === "read"` 且 `event.input.path` 以 `.pi/harness/spill/` 开头时直通不判阈值（显式取回意图，信任 agent）。

### D8 记账

`spill.write` 事件（30 天 TTL），payload `{tool, bytes, path, ok, error?}`；kind 加进 harness-log 的 union 与 RETENTION_DAYS（TEXT 列零迁移）。

## Risks / Trade-offs

- [MCP 工具结果不走 tool_result 链] → D1 烟测验证；不覆盖则记已知局限（内置工具已是主要漏网源）
- [多 handler 链序：telemetry 在前改了 content?] → telemetry 的 tool_result 只读 input 不改 content（实测），链序无影响；文档注释声明本扩展不依赖链序
- [preview 误伤「头尾都没用、中间才有用」的输出（如长 JSON 数组）] → 接受：语法剪枝本就不懂语义，取回路径永远在
- [spill 目录膨胀（大量 MCP snapshot）] → 30 天清扫 + spill.write 月度聚合可见趋势，必要时调低阈值或加 per-tool 配置（后续 change）

## Migration Plan

无迁移。新扩展文件 + harness-log 两行 + 新目录；不装本扩展的会话行为不变（kind 词汇超前定义不影响旧库读取）。

## Open Questions

（无——proposal 四个开放点均已在探索期实证解决，见 Context/D1-D3。）
