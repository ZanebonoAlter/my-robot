# harness 可观测性与门禁行为修复

<!-- constraint-domains: 无（纯工具链） -->
<!-- complexity: simple -->

## Why

基于 `.pi/harness/events.db` 近 14 天事实数据（9376 条 gate.check、87 条 inject、17 条 subagent.dispatch）的审计暴露了 harness 改造后的五处缺口：门禁记账失真（lint 报 "0 issues." 却 ok=false，事后无法还原真实失败原因）、测试规范文档 14 天 0 次 JIT 注入（写 `_test.go` 时 DSN 红线等约定裸奔）、门禁按 git 累积 diff 触发导致混合改动回合每轮全量跑 7 条命令（pnpm lint 平均 22.3s 是最大耗时黑洞，8-25 门禁通过率骤降至 84.6%，同一编译错误反复 8 轮每轮都全量等 30s+）、后台子线程派发 17 条里 13 条永远停在 background 态无完成回填（token/耗时审计黑洞）、节级注入撞上文档编辑中间态注入了 133B 残缺节。这些问题共同造成"报错多轮交互"与"测试约束不一定注入"的体感。

## What Changes

- **gate.check diag 记账修复**：`lib/failure-classify.ts` 的 `truncateDiag` 只取首个非空行，失败输出里 stdout 首行（如 "0 issues."）会掩盖 stderr 中的真实错误；改为优先提取含失败关键词（FAIL/error 等）的行、无命中再回退首行，使 DB 记账可还原真实失败原因。喂给 agent 的 steer 消息（`tail` 30 行）不变。
- **测试规范文档 JIT 标签补配**：`standard/backend/testing.md`、`standard/frontend/testing.md` 头部补 `doc-impact-applies` 标签（信号含 `_test.go` / `tests/`），写测试文件时"怎么跑"约定（含 🛑 DSN 安全红线、testcontainer 分层）可被 JIT 注入。纯文档配置修复，不改代码。
- **quality-gate 增量路由与 lint 节流**：门禁触发从「git 累积工作区 diff」改为「本回合新增改动」（对比上回合快照的文件集），纯后端回合不再因工作区残留前端脏文件而跑 pnpm lint；pnpm lint 增加节流（同一时间窗内前端无新改动则跳过并记账）。
- **subagent.dispatch 完成回填**：后台派发的子线程完成时回写事件（status/ms/tokens/toolUses），消除"派发后审计断链"。
- **节级注入最小字节下限**：节级注入内容低于下限时回退注入全文（fail-safe），避免文档编辑中间态注入残缺节。

## Capabilities

### New Capabilities

（无——全部是既有能力的 requirement 修订）

### Modified Capabilities

- `harness-fact-log`：门禁记账 requirement 的 diag 截断规范变更（首行 → 失败关键词优先）；事件词汇表新增 subagent 完成回填字段语义（background 派发事件在完成时补齐 status/ms/tokens）。
- `change-scope-gate`：quality-gate turn_end 触发范围 requirement 变更——从累积工作区 diff 改为本回合增量路由，并新增 pnpm lint 节流要求。
- `constraint-injection`：节级注入 requirement 增补最小字节下限回退（残缺节 → 全文），与既有「配置的节不存在时回落全文」同族 fail-safe。

## Impact

- **代码**（`.pi/extensions/` gitignored，改动后快照同步 `docs/research/extensions/`）：
  - `lib/failure-classify.ts`（truncateDiag 失败关键词优先）
  - `quality-gate.ts`（本回合增量路由 + pnpm lint 节流）
  - `harness-telemetry.ts`（subagent 完成回填）
  - `constraint-injection.ts`（节级注入下限）
- **文档**（仓库内）：`docs/reference/standard/backend/testing.md`、`frontend/testing.md` 补标签；`docs/research/harness事实库.md` 同步事件语义变更（若 diag/事件字段变化）。
- **无业务代码影响**：不触碰 backend-go/ 与 front/；对用户不可见（纯 harness 层）。
- 纯工具链 change，无业务域，不声明 constraint-domains。
