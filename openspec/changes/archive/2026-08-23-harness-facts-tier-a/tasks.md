# Tasks: harness-facts-tier-a

> 实现顺序即依赖顺序：先基础（D1/D2），再采集点（D3/D4/D5），最后测试与文档。
> 参考实现：`docs/research/lib/harness-log.ts`、`docs/research/harness-telemetry.ts`（源项目 fileRefs 无效，只搬逻辑）。

## 1. 基础设施迁移与安全开库（D1/D2/D7）

- [x] 1.1 新建 `.pi/extensions/lib/harness-log.ts`：迁移源实现（node:sqlite warning 外科抑制、dbCache 按路径缓存、WAL/NORMAL/busy_timeout、logEvent/queryBySession/queryByChange、TTL 分级、100MB 保险丝），`HarnessEventKind` 扩为六值（+pin.read/gate.check）
- [x] 1.2 在 openDb 中实现安全开库（D2 流程）：打开后先只读校验 application_id/user_version/sqlite_master，新库初始化魔数 0x53594E54 + user_version=1，他人库/未来版本拒绝（close + console.error + 返回 null）；全部写 PRAGMA 移到检查通过之后
- [x] 1.3 新建 `.pi/extensions/lib/active-change.ts`：从 constraint-injection.ts L120-153 提取 `listChangeDirs` + `detectActiveChange`，constraint-injection 改为 import（行为不变）
- [x] 1.4 `.gitignore` 追加 `.pi/harness/`

## 2. telemetry 迁移 + 失败白名单（A4/D4）

- [x] 2.1 新建 `.pi/extensions/lib/failure-classify.ts`：纯函数 `classifyFailure({errorText, details, started})` → `{stage, category, exitLike, diag}`；有序关键词表（quota-block/timeout/gate-fail/model-error/tool-error），不中落 unknown 不透传；diag 单行 ≤512B 剥控制字符
- [x] 2.2 新建 `.pi/extensions/harness-telemetry.ts`：迁移源实现（session_start → session.start、agentStarts 暂存、tool_result(Agent) → subagent.dispatch，details.tokens/durationMs 兼容 usage 兜底），失败路径接 `classifyFailure` 产出 failure 对象（stage 按 agentStarts 有无判定）

## 3. 采集点插桩（A3/A5/D3/D5）

- [x] 3.1 quality-gate.ts 插桩：三处门禁循环（gates 数组 / domain tests / pnpm lint）每次 pi.exec 返回后自报 `gate.check {cmd, phase:"turn_end", ok, ms, diag}`；diag 复用 A4 截断规范（512B）；change 取 `detectActiveChange`；ok=true 也记
- [x] 3.2 constraint-injection.ts 插桩 constraint.inject：planInjection 返回值增加 `docEntries: {path, mode, reason, bytes}[]`（docList 渲染不变），before_agent_start 送达时逐条 logEvent
- [x] 3.3 constraint-injection.ts 插桩 pin.write：pin_finding 成功落盘后自报（title/topic/change/最终路径），失败不记
- [x] 3.4 constraint-injection.ts 插桩 pin.read：`injectChangeFile` 注入 explore-findings.md 成功后按 `^##\s+(.+)$` 解析标题逐条自报 `{title, change, doc, digested}`；模块级 `Set<sessionId|title>` 会话内去重，session_start 清空
- [x] 3.5 pin_finding research 语境写盘追加锚点：标题行后插 `<!-- pin:<8hex> -->`（change 语境不加，复合键已够）

## 4. 测试（.pi/extensions/tests/）

- [x] 4.1 新建 `harness-log.smoke.cjs` + 挂进 `run-harness-smoke.sh`：临时目录验证——首次开库初始化（app_id/version/建表）、logEvent 写入 queryBySession 查回、他人库拒绝（预建含表 sqlite → logEvent 返回 false）、TTL 清扫（插旧行重开库被删、pin.write 保留）
- [x] 4.2 新建 `failure-classify.smoke.cjs`：六类白名单命中、不中落 unknown 不透传原文、diag 截断 ≤512B、stage 判定（started 有/无）、成功/取消不产出
- [x] 4.3 `constraint-injection.smoke.cjs` 回归：注入主路径（docList 渲染）不受 docEntries 增量影响；pin.read 去重（同会话二次注入不重复记）
- [x] 4.4 跑通 `.pi/extensions/tests/run-smoke.sh` 与新 `run-harness-smoke.sh` 全绿

## 5. 文档

<!-- doc-impact: none（harness 扩展层变更，非产品代码，无文档域命中） -->
<!-- doc-impact-excuse: api=另一 pi 窗口的 backend-go 脏文件（非本 change 改动）; database=同上; architecture=同上; configuration=同上; flow=同上 -->

- [x] 5.1 `docs/reference/constraints-index.md` 常驻索引不变（本 change 无 flow 业务约束）；在 `docs/research/harness-survey/findings.md` 末尾追加落地记录段（A 级四件套已实施 + 指向本 change），保持调研文档溯源完整
- [x] 5.2 更新 `docs/research/harness事实库.md` 头部实现状态注记：标注本仓库（Syntopica）已迁移落地 `harness-facts-tier-a`，源项目实现保持同步语义（六类事件）

## 6. 验证

- [x] 6.1 `bash .pi/extensions/tests/run-harness-smoke.sh` → 全部用例 PASS（安全开库/写入查询/TTL/白名单分类）
- [x] 6.2 `bash .pi/extensions/tests/run-smoke.sh` → constraint-injection 既有烟测不回归
- [x] 6.3 `grep -c "logEvent" .pi/extensions/quality-gate.ts .pi/extensions/constraint-injection.ts .pi/extensions/harness-telemetry.ts` → 三个采集点文件均 ≥1 处调用
- [x] 6.4 手动触发一轮真实回合后 `sqlite3 .pi/harness/events.db "SELECT kind, COUNT(*) FROM events GROUP BY kind"` → 至少出现 session.start，且实现档会话出现 constraint.inject / pin.read
- [x] 6.5 `git status --porcelain | grep -c "^??.*\.pi/harness/"` → 0（gitignore 生效，events.db 不入库）
