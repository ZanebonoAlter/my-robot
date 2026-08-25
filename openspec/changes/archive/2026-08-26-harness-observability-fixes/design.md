# harness-observability-fixes Design

## Context

五个缺口的事实证据与根因定位见 proposal.md（Why 节）。涉及的现状代码事实：

- `quality-gate.ts`（214 行）：turn_end 门禁，touchedCode 判定（turn 级工具名集合）→ git 累积 diff（`git diff --name-only HEAD` + untracked）判 isBackend/isFrontend → 全量跑三件套 + change-scope domain 测试 / pnpm lint。每命令经 `gateLog` 记 `gate.check`，diag 用 `truncateDiag`（首行截断）。
- `lib/failure-classify.ts`：`truncateDiag` 只取首个非空行——lint 失败时 stdout 首行 `0 issues.` 掩盖 stderr 真实错误的现场根源。同函数被 subagent 失败白名单（classifyFailure）复用。
- `harness-telemetry.ts`（167 行）：`tool_call(Agent)` 暂存起点 → `tool_result(Agent)` 记 `subagent.dispatch`。后台派发时 tool_result **立即返回**（content 含 `Agent ID: <id>` 与 output file 路径，details.status=background），真实完成不产生第二个 Agent tool_result——13/17 条 dispatch 永远停在 background 态的机制根源。
- **完成信号实测形态**（session JSONL 考古，2026-08-24 会话 01a03179）：父线程取结果走 `get_subagent_result` 工具，其 toolResult content 首两行是结构化摘要：`Agent: 7d1245e1-35ca-45a` / `Type: Agent | Status: completed | Tool uses: 79 | 2.0M token | Context: 54% | Duration: 3944.8s`——agentId/status/toolUses/tokens/duration 全部可解析。另有 `Agent not found: "<id>". It may have been cleaned up.` 形态（isError=false）。
- `constraint-injection.ts`（1445 行）：节级注入由文档头部 `doc-impact-applies` 标签（`| section=节名`）驱动；content-enrichment.md 现场注入 133B 残缺节。smoke test 在 `.pi/extensions/tests/*.smoke.cjs`。
- 标签现状：standard/ 13 个文档仅 3 个有标签；`backend/testing.md`（17.2KB，含 🛑 DSN 红线节）与 `frontend/testing.md`（6.2KB）均无——写 `_test.go` 时仅 test-design.md（测什么）命中，「怎么跑 + DSN 红线」14 天 0 注入。

约束：`.pi/extensions/` gitignored（改动须快照同步 `docs/research/extensions/`）；extension 为运行时 `/reload` 加载，无构建链；事实库新 kind 走 TEXT 列零迁移。

## Goals / Non-Goals

**Goals:**

- gate.check 的 diag 事后可还原真实失败原因（审计可用）。
- 门禁只对本会话、本回合真实发生的改动负责；消除修复循环中的全量重跑与 22s pnpm lint 黑洞。
- 后台子线程完成可审计（agentId 关联、token/耗时回填）。
- 节级注入不进残缺内容；测试规范文档进入 JIT 覆盖。

**Non-Goals:**

- 不改门禁命令集本身（三件套/-short/pnpm lint/cmd.exe 路径约束原样）。
- 不做后台子线程的运行中进度回填（只记完成）；断链不伪造。
- 不动 keywordDocs 关键词表与 budgetBytes 降级算法。
- 不为 8-23 改造日的海量 gate.check 噪音做数据清理（TTL 自清）。

## Decisions

### D1 增量路由：git 状态快照差分（模块级，会话边界重置）

quality-gate 维护会话内快照 `Map<path, {mtimeMs, size}>`（ESM 模块状态，与 constraint-injection 同款模式）：

- **初始化**：`session_start`（new/resume/fork/reload）时读当前 tracked-diff + untracked 的 stat 建基线。会话前残留脏文件进基线 → 不触发（spec「残留不触发」）。
- **触发集**：turn_end 时读当前集合，新增路径或 (mtime,size) 变化者 = 触发集。替换现有 isBackend/isFrontend 全局判定：后端门禁仅当触发集命中后端路径，前端门禁仅当命中前端路径。判定完（无论跑没跑）更新快照。
- **失败粘性**：上回合失败未转绿的命令，本回合即使触发集空也重跑同一命令（防止 agent「口头说修了但没改文件」时门禁沉默）。粘性集合在转绿或 session 边界清空。

选 git 快照差分而非枚举 write/edit 工具事件的理由：bash 间接改动（sed/go fmt/gofmt -w/pnpm fix）不会被工具枚举捕获，git 是唯一全量真相源；且与现有 touchedCode 检测同源（同一组 git 命令），增量小。保留 touchedCode 作为零成本早退（纯对话回合跳过 git 调用）。

mtime+size 双键而非内容 hash：编辑必然更新两者，hash 每回合全量读文件 IO 不可接受（WSL drvfs 慢）。

### D2 diag 失败特征优先：lib 新纯函数，不动 truncateDiag

`lib/failure-classify.ts` 新增 `truncateDiagGate(text)`：按有序关键词表（`FAIL`、`error`、`# `<pkg> 行、`exit`、`undefined`、`cannot`、`denied` 等）取首个命中行，无命中回退首个非空行；截断规范复用现有（单行/剥控制字符/≤512B）。quality-gate 的 gateLog 换用之；`classifyFailure`（子线程白名单）继续用原 `truncateDiag`（错误文本以 error 行开头的场景语义不同）。纯函数、确定性 → 进 smoke test。

steer 回喂路径（`tail(30)`）不变——信息本来就全，只修 DB 记账。

### D3 subagent.complete：挂 tool_result(get_subagent_result) 解析摘要

harness-telemetry 增加监听 `toolName === "get_subagent_result"` 的 tool_result：

- content 首两行正则解析：`^Agent: ([0-9a-f-]+)` 提 agentId；第二行 `Status: (\w+) | Tool uses: (\d+) | ([\d.]+[KM]?) token | ... Duration: ([\d.]+)s` 提 status/toolUses/tokens/ms。
- 解析不出 agentId（含 `Agent not found` 形态、格式漂移）→ 跳过不记账不报错（容错 fail-safe，pi 升级改格式时退化为现状而非崩溃）。
- **change 绑定**：dispatch 时（Agent tool_result）以 in-memory `Map<agentId, change>` 暂存当时绑定，complete 时复用；reload 丢 map 后落 null（change 列可空，审计按 parent_session_id 兜底关联）。
- 幂等：`Set<agentId>` 已完成的跳过（session_start 重置）。
- 取消/失败：status 原样透传（cancelled/error 等枚举由 pi 决定，记账不翻译）；isError = event.isError 或 status 含 error。

不选轮询 output file（/tmp/pi-subagents-0/...）：路径环境耦合、轮询时点不自然；不选挂通知消息：实测会话 JSONL 中无独立 user 角色通知形态，机制不明依赖脆弱。get_subagent_result 是父线程取结果的标准路径，实测 17 条 dispatch 中 7 条有配对取结果。

### D4 minSectionBytes：extractSection 纯函数扩展

配置新增 `minSectionBytes`（缺省 512）。节提取纯函数返回值扩展为 `{content, fellBack}`：节不存在（现状）或节字节数 < 下限时 fellBack=true、content=全文。记账 bytes 用实际注入内容字节数（回退时即全文字节数，spec「不虚标」）。进 smoke test（<512B 节 + 完整全文 → 判定回退）。

### D5 测试规范文档标签分工（避免双文档重复注入）

`test-design.md` 已有标签（signals 含 `_test.go`/`.spec.ts`/`.test.ts`，注「JIT 注入摘要」节=测什么+分层速查）。因此两份 testing.md 补标签时选**不与 test-design 重复**的节：

- `backend/testing.md`：`doc-impact-applies: _test.go, backend-go/internal/testutil, tests/ | section=🛑 DSN 安全红线（事故教训 — 不可违反）`——17KB 全文注入不可行；分层速查 test-design 已覆盖，DSN 红线（清空业务数据级事故教训）是本文档独有且最致命的内容。节名与 `## ` 标题字符串精确匹配（emoji 无碍）。
- `frontend/testing.md`：`doc-impact-applies: .test.ts, .spec.ts, front/tests/ | section=单元测试（Vitest）`——Vitest 文件位置/命名/环境约定是前端测试最常被违反处。

写 `_test.go` 时最终注入：test-design 摘要（2.6K）+ DSN 红线节（~1K），预算内。

### D6 testing.md 标签遵循 doc-authoring 注册点

标签新增按 `standard/shared/doc-authoring.md` 的注册点 checklist 走（该文档自身有标签会 JIT 提示），本 change 的文档任务里显式过一遍 checklist，不豁免。

## Risks / Trade-offs

- [增量路由漏检：文件回写 mtime 未变（理论上）] → mtime+size 双键，编辑场景必然变化；粘性失败重跑兜底「修复未生效」漂移。
- [get_subagent_result 文本格式随 pi 升级漂移] → 正则解析 fail-safe：提不出 agentId 就静默跳过，退化为现状（无 complete 事件），不会误记账；真实样本串进 smoke test 断言。
- [失败粘性在 agent 拒绝修复时每回合全量重跑] → 有意为之（门禁语义就是催修）；上限为既有会话行为，非新增负担。
- [minSectionBytes 误伤真实的短节] → 回退全文只多注字节，预算降级机制兜底，无信息损失。
- [dispatch/complete 跨 reload 丢 change 绑定] → 落 null + parent_session_id 关联兜底；发生频率低（reload 罕见）。
- [后台 agent 完成但父线程从未取结果] → 无 complete 事件（spec「断链不伪造」语义），审计侧「有 dispatch 无 complete」可发现；这是信号源固有局限，接受。

## Migration Plan

1. 实现顺序：D2/D4（纯函数+smoke test，零行为风险）→ D1（quality-gate 增量路由）→ D3（telemetry 回填）→ D5 标签（文档）。
2. 每步实现后 `/reload` 生效，用事件库即时验证（下一回合 turn_end 的 gate.check 触发集、diag 内容）。
3. 快照同步：改动的 extension 文件同步到 `docs/research/extensions/`。
4. 回滚：还原快照文件 + `/reload`；事实库新 kind（subagent.complete）无 DDL，无需回滚。

## Open Questions

（无——D1-D6 均已定案，实现细节不改变 specs 契约。）
