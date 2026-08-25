# Design: test-case-entry-gate

## Context

spec-gate 检查⑤a（`scanAcceptanceWording`，spec-gate.ts 导出纯函数）目前只挂 `tool_call` 的 `openspec archive` 命令，warn 级留痕不进 agent 上下文。本 change 把同一义务的反馈提前到 implementation 档动工瞬间，并引入复杂度声明制替代词法猜测作为主判定。事件库考古与三条路线全量校准（4 词 fire 33% / 19 词 fire 69% / 结构化阈值 83%）见 `docs/research/spec-gate-test-cases-entry/explore-findings.md`。

现状事实（实现时直接依赖）：

- `mode.set` 事件由 constraint-injection.ts 在档位激活/绑定修正时写入 events.db（payload `{mode, boundChange}`）；恢复路径只按本会话 session_id 查询（多窗口并行是常态，全局兜底会串档，constraint-injection.ts:1051 实测教训）。
- `lib/harness-log.ts` 提供 `queryBySession(cwd, sessionId, kinds)`；`quality-gate.ts` 演示了 turn_end + `pi.sendMessage({deliverAs:"steer", triggerTurn:true})` 软提示模式与 session_start{reason:startup} 跳过防御（pi-subagents 派发子线程时向同一共享模块实例发 startup 事件，清状态会误伤主会话）。
- 冒烟链：`run-harness-smoke.sh` 用 esbuild 把 .ts bundle 成 .cjs 再 node 跑断言；spec-gate.smoke.cjs 已 import `scanAcceptanceWording` / `COMPLEXITY_KEYWORDS`。

## Goals / Non-Goals

**Goals**

- implementation 档 + 缺白盒用例文档的复杂档，在动工首回合内收到一条进上下文的 steer 提醒。
- 复杂度判定主信号 = proposal 头自声明；4 词表降级为未声明/声明 simple 时的兜底质询。
- entry-gate 与 spec-gate ⑤a 同源调用共享纯函数，不出现两份判定逻辑。

**Non-Goals**

- 不做 block 级阻断（长任务卡死风险，quality-gate 决策 #4 同款理由）。
- 不改 constraint-injection.ts（1451 行零接触，mode 状态经 events.db 读取而非内存共享）。
- 不扩容关键词表、不做结构化阈值（校准否决）。
- 不追改存量 change 的声明（声明制只对新 change 生效；存量由兜底词表覆盖，行为与今天一致）。

## Decisions

**D1 入口挂 turn_end 而非 before_agent_start，mode 状态从 events.db 本会话查询。**
before_agent_start 的 sendMessage 语义未经验证（steer + triggerTurn 在该时点可能递归或排队）；turn_end + steer 是 quality-gate 验证过的模式。代价：提醒落在动工首回合末而非回合前——差半回合，可接受（quality-gate 同款时延）。mode.set 由 constraint-injection 写入，本会话 `queryBySession(cwd, sessionId, ["mode.set"])` 取最新一条即当前档位；无记录（requirements 档/未激活）零成本跳过。备选「扩展间直接 import constraint-injection 内部状态」被否：跨扩展内存共享脆、且违反两扩展职责隔离。

**D2 共享纯函数落 `lib/test-case-gate.ts`，spec-gate.ts 反向引用。**
新增：`parseComplexityDeclaration(proposalText) → "complex" | "simple" | null`（认 `<!-- complexity: complex|simple -->`，大小写敏感、容忍前后空白、多次声明取首个、其余值视为未声明）；`scanComplexityKeywords(tasksMd) → string[]`（从 `scanAcceptanceWording` 抽出任务行收集命中词的核，⑤b 留在原函数）。`COMPLEXITY_KEYWORDS` 常量移入 lib，spec-gate.ts re-export 保持 smoke 现有 import 路径不破。entry-gate.ts 与 spec-gate.ts 都只调 lib。

**D3 entry-gate 提醒文案与去重。**
文案三段：现状（声明/兜底命中词）、义务（case-first-testing 复杂档白盒用例文档）、修复路径（补 test-cases.md 或改 proposal 声明）。去重：模块级 `Map<changeName, true>`，session_start（非 startup）清空；文件补齐后检查自然通过（不依赖去重记忆）。触发即记 `gate.check`（cmd=entry-gate，ok=命中与否，payload 含 declaration/kwHits），事件库可考古。

**D4 spec-gate ⑤a 判定改声明优先，级别维持 warn。**
归档时序：complex 声明 + 缺文档 → 强违例文案；simple/未声明 + 词表命中 → 反向质询文案（「声明 simple 但任务行命中『X』：改声明或补文档」）。不升 block——归档四项 block 检查语义不动（scenario-trace 已是 block 兜底），⑤a 仍是措辞级提醒。

**D5 声明义务的落点：开发执行规范 §2 + test-design.md 双同步（skill 不动）。**
openspec 官方 skill（new-change/propose/continue）不植入仓库定制项（openspec 是外部工具，升级会冲掉定制）；声明义务的家 = `docs/reference/开发执行规范.md` §2 新增「复杂度声明制」段（判定标准 + 机器执行 + 红旗项）。test-design.md「验收措辞规范」节禁用词表 ⑤a 行改写为声明制语义，保持「改表必同步」双向义务（同步对象多一个 lib/test-case-gate.ts）。

## Risks / Trade-offs

- [events.db 读失败/无 sessionId] → fail-open 静默跳过（门禁自身故障不阻断干活），console.warn 留痕；与全扩展一致。
- [mode.set 记录晚于首回合末] → 档位激活发生在 before_agent_start，同回合 turn_end 必然晚于写入，无竞态；resume 两段式恢复期间查不到 → 该回合跳过，下回合补上。
- [声明 simple 规避义务] → 兜底词表质询 + 归档时 scenario-trace block 对账仍在；声明制换的是误报归零，不是放弃核验。
- [提醒被无视] → steer 每会话每 change 一次不轰炸；session 重启仍缺文档会再次提醒（持续存在而非一次性）。

## Migration Plan

无数据迁移。部署即生效（pi 重载扩展）。回滚 = 删 entry-gate.ts + 还原 spec-gate.ts/lib（git revert 单 change）。

## Open Questions

（无）
