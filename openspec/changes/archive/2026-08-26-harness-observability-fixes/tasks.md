# harness-observability-fixes Tasks

> 实现顺序按 design Migration Plan：纯函数（1）→ quality-gate（2）→ telemetry（3）→ 文档标签（4）。
> 每步完成即可 `/reload` 生效并用事件库即时验证。`.pi/extensions/` gitignored，4.3 统一快照同步。

## 1. 纯函数层：diag 失败特征提取 + 节最小字节下限

- [x] 1.1 `lib/failure-classify.ts` 新增 `truncateDiagGate(text)` 纯函数：按有序关键词表（`FAIL`、`error`、`# `<pkg> 行、`exit`、`undefined`、`cannot`、`denied`）取首个命中行，无命中回退首个非空行；截断规范复用既有（单行/剥控制字符/≤512B/确定性）。原 `truncateDiag` 不动（classifyFailure 继续用）。扩展 `failure-classify.smoke.cjs` 断言：`"0 issues." + stderr 编译错误` → diag 含编译错误特征行；`"? pkg [no test files]\nFAIL pkg [build failed]"` → diag 含 FAIL；无关键词输出 → 回退首行；同输入两次调用结果逐字节相等
- [x] 1.2 `quality-gate.ts` 的 gateLog diag 换用 `truncateDiagGate`（steer 回喂路径 `tail(30)` 不变）。验证：run-harness-smoke.sh 通过 + 手动制造一次 lint 失败后 `sqlite3` 查最新 gate.check diag 含真实错误行
- [x] 1.3 `constraint-injection.ts` 节提取纯函数返回值扩展为 `{content, fellBack}`：节不存在或字节数 < `minSectionBytes`（config 新增，缺省 512）时 fellBack=true 且 content=全文；记账 bytes 用实际注入字节数。`run-smoke.sh` 增断言：fixture 造 <512B 残缺节 → 注入全文 + constraint.inject bytes=全文字节数

## 2. quality-gate 增量路由（快照差分 + 失败粘性）

- [x] 2.1 新增 `lib/` 纯函数 `computeTriggerSet(prev: Map<path,{mtime,size}>, curr: Map<path,{mtime,size}>): Set<path>`（新增路径或 mtime/size 变化 = 触发），新建 `.pi/extensions/tests/quality-gate.smoke.cjs` 挂进 `run-harness-smoke.sh`。断言：残留基线不触发、编辑后触发、纯删除不触发、空触发集为空集（run-harness-smoke.sh 7 断言全绿）
- [x] 2.2 `quality-gate.ts`：模块级快照 Map（session_start 以当前 tracked-diff+untracked 的 stat 初始化，session_shutdown 清理，reason=startup 跳过防子线程误清）；turn_end 时读当前集合 → computeTriggerSet → 按触发集独立路由两侧，跳过侧零 gate.check；判定后更新快照。touchedCode 保留为纯对话回合早退（bundle 可加载验证 + 行为实测归 2.4）
- [x] 2.3 失败粘性：模块级 `stickyFailures: Set<cmd>`，本回合失败命令入集，下回合即使触发集空也重跑，转绿或 session 边界移除。smoke 或人工验证：制造 go vet 失败 → 下一回合纯对话仍重跑 vet → 修复后转绿并出集
- [x] 2.4 事件库实测：`/reload` 后仅编辑一个后端 `.go` 文件跑一回合，`sqlite3` 验证该回合 gate.check 无 pnpm lint 行；再仅编辑 `front/` 文件，验证只有 pnpm lint

## 3. telemetry 完成回填（subagent.complete）

- [x] 3.1 `harness-telemetry.ts`：Agent tool_result 时（background 派发）以 `Map<agentId, change>` 暂存当时 change 绑定；新增 `tool_result(get_subagent_result)` 监听——解析 agentId/status/toolUses/tokens/durationMs，解析失败（含 `Agent not found`、running 非终态）静默跳过；成功追加 `subagent.complete`（change 复用暂存绑定，reload 丢 map 落 null）；幂等 `Set<agentId>`（session_start 重置）
- [x] 3.2 `failure-classify.smoke.cjs`（或 telemetry 段）增真实样本回放断言：`"Agent: 7d1245e1-35ca-45a\nType: Agent | Status: completed | Tool uses: 79 | 2.0M token | Context: 54% | Duration: 3944.8s\n..."` → 解析出 agentId=7d1245e1-35ca-45a、status=completed、toolUses=79、tokens=2000000、ms=3944800；`Status: cancelled` 样本 → status=cancelled 透传且 isError=false；`"Agent not found: ..."` → 不产生事件；同一 agentId 两次 → 仅一条 complete（7 断言全绿）

## 4. 文档标签与同步

- [x] 4.1 `docs/reference/standard/backend/testing.md` 头部加标签（信号 `_test.go, backend-go/internal/testutil, tests/workflow, tests/firecrawl`——避开裸 `tests/` 子串误命中），节名与 `## ` 标题逐字一致且节 635B≥512 下限；doc-authoring 注册点 checklist 已过（constraints-index 执行规范表已登记本文件）
- [x] 4.2 `docs/reference/standard/frontend/testing.md` 头部加标签（信号 `.test.ts, .spec.ts, front/tests/`），节名一致且节 2811B
- [x] 4.3 快照同步：`lib/failure-classify.ts`、`lib/trigger-set.ts`、`quality-gate.ts`、`harness-telemetry.ts`、`constraint-injection.ts` 与 tests/ 同步到 `docs/research/`；`docs/research/harness事实库.md` 迁注补九类事件 + diag 语义 + 增量路由 + minSectionBytes（md5 diff 校验一致）

## 5. 测试

- 本 change 全部为 harness 扩展层（`.pi/extensions/`），无 backend-go/front 业务代码改动，豁免 go test / pnpm test:unit
- 自动化测试 = extension smoke 套件：`bash .pi/extensions/tests/run-harness-smoke.sh` + `bash .pi/extensions/tests/run-smoke.sh`（1.1/1.3/2.1/3.2 扩展断言后必须仍全绿）
- 行为验证 = 事件库实测（2.4 / 1.2 / 3.x 人工步骤），结果记入本文件勾选备注

## 6. 文档

<!-- doc-impact: standard -->
<!-- doc-impact-excuse: flow=backend-go 脏文件属 candidate-topic-l2-gate 等并行 change 残留，本 change 未改 backend-go（代码全在 gitignored 的 .pi/extensions/）; api=同上（handler 脏文件非本 change）; database=同上（models 脏文件非本 change） -->

> 声明 standard：改了 `docs/reference/standard/backend|frontend/testing.md`（头部标签 + DSN 节补齐）；另 `开发执行规范.md`（根级非 8 域）与 `AGENTS.md`（仓库根）随行为修订同步。

- [x] 6.1 `docs/reference/standard/backend/testing.md` 头部标签（4.1）
- [x] 6.2 `docs/reference/standard/frontend/testing.md` 头部标签（4.2）
- [x] 6.3 `docs/research/harness事实库.md` 事件词汇 + diag 语义同步（4.3）
- [x] 6.4 `docs/research/` 快照同步（extensions/lib/tests，4.3）
- [x] 6.5 `minSectionBytes` 未入 `.pi/constraint-injection.json`（缺省 512 在代码注释 + harness事实库.md 迁注说明，无需额外文档）
- [x] 6.6 `docs/reference/开发执行规范.md` §4.1 门禁分层改增量路由表述；`AGENTS.md`「pi 增量门禁」段同步（行为描述与代码对齐）

## 7. 验证

每条命令实测零失败后方可归档（§11.2）：

```bash
# 1. smoke 全绿（新增断言含在内）
bash .pi/extensions/tests/run-harness-smoke.sh   # 期望 exit 0，无 assertion failed
bash .pi/extensions/tests/run-smoke.sh           # 期望 exit 0

# 2. 标签就位且节名与文档标题逐字一致
grep -n "doc-impact-applies" docs/reference/standard/backend/testing.md   # 期望 1 行且 section 值与该文件某 "## " 标题相等
grep -n "doc-impact-applies" docs/reference/standard/frontend/testing.md  # 期望 1 行且同上

# 3. 快照同步完整（改动文件与 docs/research/extensions/ 一致）
diff <(md5sum .pi/extensions/quality-gate.ts | cut -d' ' -f1) <(md5sum docs/research/extensions/quality-gate.ts | cut -d' ' -f1)  # 期望无输出（其余改动文件同理）

# 4. 事件库行为实测（人工操作 + 机器查询）
#    前置：/reload 后①仅编辑 backend-go 某 .go 跑一回合 ②派发一个后台子线程并 get_subagent_result
sqlite3 .pi/harness/events.db "SELECT DISTINCT json_extract(payload,'$.cmd') FROM events WHERE kind='gate.check' AND ts > datetime('now','-10 minutes')"  # 期望①后无 pnpm lint
sqlite3 .pi/harness/events.db "SELECT kind, json_extract(payload,'$.status'), json_extract(payload,'$.tokens') FROM events WHERE kind='subagent.complete' ORDER BY id DESC LIMIT 1"  # 期望②后一条 status/tokens 非空
```

### Scenario 映射表

| Scenario | 测试文件 |
| --- | --- |
| TTL 分级清扫 | .pi/extensions/tests/harness-log.smoke.cjs |
| 事件追加不可变 | .pi/extensions/tests/harness-log.smoke.cjs |
| spill.write 事件随词汇扩展落库 | .pi/extensions/tests/spill.smoke.cjs |
| subagent.complete 随词汇扩展落库 | .pi/extensions/tests/failure-classify.smoke.cjs |
| 门禁命令失败记账 | .pi/extensions/tests/failure-classify.smoke.cjs |
| 未运行不记账 | 人工：§7 第 4 条①后 sqlite3 查询无 pnpm lint 即未运行侧零记账 |
| diag 失败特征优先（stdout 噪声行不掩盖真实错误） | .pi/extensions/tests/failure-classify.smoke.cjs |
| go test 记账不丢 FAIL 行 | .pi/extensions/tests/failure-classify.smoke.cjs |
| 后台完成回填 | .pi/extensions/tests/failure-classify.smoke.cjs |
| 取消记 cancelled | .pi/extensions/tests/failure-classify.smoke.cjs |
| 断链不伪造 | .pi/extensions/tests/failure-classify.smoke.cjs |
| domain 改动触发自动测试 | 人工：既有行为，§7 第 4 条①事件含 domain go test |
| DB 依赖包跳过 | 人工：既有行为不变（-short 自动 skip，无新增断言面） |
| 前端改动行为不变 | 人工：§7 第 4 条扩展——仅编辑 front 后事件仅 pnpm lint |
| 残留前端脏文件不触发前端门禁 | .pi/extensions/tests/quality-gate.smoke.cjs |
| 会话前残留改动不触发首轮门禁 | .pi/extensions/tests/quality-gate.smoke.cjs |
| 修复循环每轮按新变化触发对应侧 | .pi/extensions/tests/quality-gate.smoke.cjs |
| 实现档注入生效 | .pi/extensions/tests/run-smoke.sh |
| 未激活档仅索引 | .pi/extensions/tests/run-smoke.sh |
| JIT 路径细化 | .pi/extensions/tests/run-smoke.sh |
| flow 文档节级注入 | .pi/extensions/tests/run-smoke.sh |
| 节残缺时回退全文 | .pi/extensions/tests/run-smoke.sh |
| change 文本不触发关键词命中 | .pi/extensions/tests/run-smoke.sh |
| ASCII 关键词词边界整词匹配 | .pi/extensions/tests/run-smoke.sh |
| 命中只增不减（缓存稳定） | .pi/extensions/tests/run-smoke.sh |
| 无声明不注入并提示 | .pi/extensions/tests/run-smoke.sh |
| 未知域名宽容忽略 | .pi/extensions/tests/run-smoke.sh |
| 超预算分层降级 | .pi/extensions/tests/run-smoke.sh |
| 降级永不真丢 | .pi/extensions/tests/run-smoke.sh |
| 模型可见省略通知 | .pi/extensions/tests/run-smoke.sh |
| 降级确定性（缓存友好） | .pi/extensions/tests/run-smoke.sh |
| 降级记账 | .pi/extensions/tests/run-smoke.sh |
| smoke test 直跑 | .pi/extensions/tests/run-smoke.sh |
| 节提取回落 | .pi/extensions/tests/run-smoke.sh |
| 节低于最小字节下限回退全文 | .pi/extensions/tests/run-smoke.sh |

注：标 run-smoke.sh / failure-classify.smoke.cjs / quality-gate.smoke.cjs 的行中，新增/变更断言在任务 1.1/1.3/2.1/3.2 中扩展（映射文件即断言载体）；既有行为的行映射到既有回归套件。
