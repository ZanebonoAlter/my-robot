# Proposal: harness-facts-tier-a

## Why

本仓库 pi harness 已有约束注入、质量门禁、额度门禁等扩展，但 harness 层发生的事实（注入了什么、门禁拦了几轮、子线程为何失败、pin 有没有被动用）没有结构化账本——跨会话审计、change 收尾复盘、规则调优只能翻 session JSONL。另一项目已实现 harness 事实库核心（`.pi/harness/events.db` + 唯一写入方 `logEvent`，设计文档见 `docs/research/harness事实库.md`）；本 change 将其迁移为本仓库基础设施，并落地 2026-08-21 dsh/codex 调研（`docs/research/harness-survey/findings.md`）确定的 A 级增强四件套。

## What Changes

- **迁移前提**：`harness-log.ts` 落地 `.pi/extensions/lib/`（node:sqlite 单表 append-only、WAL、TTL 分级清扫、100MB 保险丝、fail-loud）；`harness-telemetry.ts` 落地 `.pi/extensions/`（session.start / subagent.dispatch 采集）
- **A2 安全开库**（迁移时一并做，避免二次改开库逻辑）：`PRAGMA application_id` 魔数 + `user_version` schema 版本；识别到他人库/未知版本一律**拒绝打开**（本库是 append-only 审计账本，不做 dsh 派生库的"重置重建"）
- **A3 门禁记账**：quality-gate.ts 插桩，每命令一条 `gate.check` 事件（cmd/phase/ok/ms/diag 截断摘要），失败喂 steer 属正常流程，账本只记不评判
- **A4 子线程失败白名单**：subagent.dispatch 失败 payload 增加 `failure` 对象（stage: dispatch|run|result；category: quota-block|timeout|gate-fail|model-error|tool-error|unknown；diag ≤512B）。映射不进白名单一律 unknown，不透传原始错误文本
- **A1 pin 使用遥测**：constraint-injection 注入 explore-findings.md 时按 `###` 标题自报 `pin.read`；pin 身份 change 语境用 `(change, title)` 复合键，research 语境（`docs/research/<topic>/`）用 pin_finding 写入时生成的锚点 id。**效果待观察项：A4 的失败分类是否真能提升诊断效率，后续特别关注**
- `.gitignore` 追加 `.pi/harness/`（本机运行数据不入库）
- 扩展 `.pi/extensions/tests/` 烟测覆盖新模块

不改变任何注入/门禁/派发行为本身，只增加记账侧写。**无产品代码影响（front/、backend-go/ 零改动）。**

## Capabilities

### New Capabilities

- `harness-fact-log`: harness 层事实账本——存储约定（单表 append-only、TTL、安全开库）、唯一写入方 `logEvent`、事件类型词汇（session.start / constraint.inject / pin.write / pin.read / gate.check / subagent.dispatch 含失败白名单）、fail-loud 边界

### Modified Capabilities

<!-- constraint-injection / quality-gate 仅增加记账插桩，注入与门禁的 spec 级行为不变，不列入 -->

## Impact

- **新增**：`.pi/extensions/lib/harness-log.ts`、`.pi/extensions/harness-telemetry.ts`
- **修改**：`.pi/extensions/quality-gate.ts`（gate.check 插桩）、`.pi/extensions/constraint-injection.ts`（pin.read 插桩）、`.gitignore`、`.pi/extensions/tests/`
- **运行时产物**：`.pi/harness/events.db`（本机生成，gitignore）
- **参考**：源实现 `docs/research/harness-telemetry.ts`、`docs/research/lib/harness-log.ts`；调研依据 `docs/research/harness-survey/findings.md` §三 A 级
