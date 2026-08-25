# Design: harness-facts-tier-a

## Context

另一项目的 harness 事实库（设计文档 `docs/research/harness事实库.md`、实现 `docs/research/harness-telemetry.ts` + `docs/research/lib/harness-log.ts`）已验证核心采集可行。本仓库 `.pi/extensions/` 现有 5 个扩展（constraint-injection / quality-gate / quota-gate / spec-gate / test-scope-guard），全部零记账——harness 层事实（注入、门禁、派发、pin）跨会话不可查。本 change 迁移基础模块并落地调研 A 级四件套（`docs/research/harness-survey/findings.md` §三）。

约束：pi 扩展改不了 session 事件词汇（不像 dsh 插件可扩展 SessionEventMap），独立 SQLite 账本是既定架构决策；模型全程零参与。

**勘误**：proposal 中 pin.read "按 `###` 标题" 应为 `## ` 二级标题——`pin_finding` 落盘格式为 `\n## ${title}\n`（constraint-injection.ts L763），`digestFindings` 亦按 `^##\s+` 解析，两侧一致。

## Goals / Non-Goals

**Goals**

- 事实库基础设施落地本仓库：`lib/harness-log.ts`（唯一写入方）+ `harness-telemetry.ts`（钩子采集），六类事件全部有采集点
- A2/A3/A4/A1 四件套，与源实现保持同构（便于两项目双向回流）
- 烟测覆盖：安全开库、失败分类纯函数、事件写入/查询

**Non-Goals**

- 不做 dump 命令、`harness_trace` 工具、pin.touch（模型主动翻源文件检测）——二期
- 不改任何注入/门禁/派发的既有行为，只加侧写
- 不建 usage 物化列/新表（SQL 聚合够用）
- 不做跨项目库共享（每仓库独立 `.pi/harness/events.db`）

## Decisions

### D1: 迁移落位与事件采集点

| 文件 | 职责 | 事件 |
| --- | --- | --- |
| `.pi/extensions/lib/harness-log.ts`（迁移+D2 改造） | 开库/建表/TTL/保险丝/`logEvent`/`queryBySession`/`queryByChange` | 唯一写入方 |
| `.pi/extensions/harness-telemetry.ts`（迁移+A4 改造） | `session_start` / `tool_call` / `tool_result` 钩子 | session.start、subagent.dispatch（含失败白名单） |
| `.pi/extensions/lib/failure-classify.ts`（新增） | 纯函数：错误文本 → `{stage, category, exitLike, diag}` | A4 的映射器，独立成模块仅为可测 |
| `.pi/extensions/lib/active-change.ts`（新增，自 constraint-injection L120-153 提取） | `detectActiveChange(cwd)` mtime 兜底判定 | quality-gate 与 constraint-injection 共享 |
| constraint-injection.ts 插桩 | planInjection 返回结构化 docEntries；pin_finding 成功落盘后自报 | constraint.inject（每回合每文档）、pin.write、pin.read（D5） |
| quality-gate.ts 插桩 | 每条门禁命令执行后自报 | gate.check（D3） |

`node:sqlite` 实验 warning 外科抑制（摘除监听→require→装回过滤包装）从源实现原样迁移；Node ≥22 可用，pi 运行于 v24。lib/ 无 index.ts，pi 只自动加载 `.pi/extensions/*.ts` 顶层文件，lib 子目录不构成扩展（源实现已验证）。

### D2: 安全开库（A2）——检查在写之前，拒绝而非重建

```
DatabaseSync(file) 打开（不存在则创建空库）
  ├─ 读 application_id / user_version / sqlite_master 计数（均只读）
  ├─ app_id=0 且 无用户表    → 新库：SET app_id=0x53594E54("SYNT")、version=1、建表建索引
  ├─ app_id=0 且 有用户表    → 拒绝（他人未登记库；CREATE IF NOT EXISTS 会静默污染对方 schema）
  ├─ app_id=0x53594E54 且 version≤1 → 正常开（未来 version>1 走迁移）
  ├─ app_id=0x53594E54 且 version>1 → 拒绝（未来版本的库，降级写入会弄坏）
  └─ 其他 app_id             → 拒绝（明确的他人库）
拒绝 = console.error + close + 返回 null（现有 fail-loud 路径，logEvent 返回 false，telemetry 一次性 notify）
```

关键顺序：**所有写 PRAGMA（含 `journal_mode=WAL`）必须在检查通过之后**——dsh 明确指出改 journal mode 本身就是写操作，会在别人库里留下 -wal/-shm。失败策略选"拒绝"不选 dsh 派生库的"重置重建"：本库是 append-only 审计账本，重置等于销毁审计史；丢失风险由 TTL 与 100MB 保险丝管。`user_version=1` 同时是未来 schema 迁移的锚点。

### D3: gate.check（A3）——每命令一条，成功失败都记

quality-gate.ts 三处门禁循环（后端 gates 数组、change-scope domain tests、前端 pnpm lint）在每次 `pi.exec` 返回后自报：

```json
{ "kind": "gate.check", "change": "<detectActiveChange 结果，无则 null>",
  "payload": { "cmd": "golangci-lint", "phase": "turn_end", "ok": false,
               "ms": 45230, "diag": "截断摘要（A4 规范，≤512B）" } }
```

- `cmd` 用现有 label（golangci-lint / go vet / go build / 具体 go test 命令 / pnpm lint）
- `ok: true` 也记（统计失败率需要分母）；diag 仅失败时非 null
- change 绑定取 `lib/active-change.ts` 的 `detectActiveChange`（mtime 兜底，与 constraint-injection 同源）；失败→修复闭环靠相邻 gate.check 的 ok 翻转表达，修复回合不单独记
- 防递归：logEvent 是纯 INSERT，不触碰 turn_end 触发条件
- 备选（否决）：change=NULL 靠 session_id JOIN constraint.inject 反查——审计查询复杂化，且共享检测函数本就该提取

### D4: 失败白名单（A4）——纯函数映射，宁可 unknown 不透传

`lib/failure-classify.ts` 导出 `classifyFailure(input: { errorText, details, started }): FailureFact`：

- **stage**：`started=false`（agentStarts 无此 toolCallId，如 quota-gate 拦截）→ `dispatch`；`started=true` 且 isError → `run`；`details.status` 显示 agent 完成但结果装配失败 → `result`
- **category**（有序正则表，首个命中胜出；不命中→ unknown，**不复制原值**）：

| category | 信号特征 |
| --- | --- |
| quota-block | quota-gate block reason 特征：额度/剩余/窗口/重置/block |
| timeout | timeout / timed out / 超时 / ETIMEDOUT |
| gate-fail | 增量门禁 / quality-gate / 门禁未通过 |
| model-error | rate limit / 429 / 5xx / provider / overloaded |
| tool-error | exit code / not found / permission denied / ENOENT / EACCES |
| unknown | 以上皆不中（含无错误文本） |

- **diag**：首个非空错误行、剥控制字符、压成单行、≤512 字节（截断加 `…`）
- **exitLike**：可提取的数字退出码，否则 null
- dsh 纪律："诊断是展示文本不是协议"——程序不得按 category 名做逻辑分支（统计/展示除外）

**效果待观察**（用户指定）：落地一个月后用 `GROUP BY category` 回看分布与排障命中率，若 unknown 占比 >50% 或类别无区分度，评估扩充关键词或放弃分类只留 diag。

### D5: pin 使用遥测（A1）——注入侧采集 + 会话内去重 + 分语境身份

- **采集点**：constraint-injection 在实现档注入 explore-findings.md 时（`injectChangeFile` 成功后），对注入原文按 `^##\s+(.+)$` 解析标题，每个标题自报一条 `pin.read`：`{title, change, doc: 相对路径, digested: bool}`（digest 模式下标题仍保留，照常记）
- **会话内去重（必须）**：实现档每回合 before_agent_start 都会重建注入块，不去重会每回合重复计数。方案：constraint-injection 模块级 `Set<sessionId|title>`，`session_start` 清空；命中则跳过。usage 聚合以"每会话首次注入"计数，语义 = "该会话用过这个 pin"
- **身份策略**：change 语境复合键 `(change, title)`（生命周期被 change 锁死，断链损失可忽略）；research 语境 pin_finding 写盘时在标题行后追加锚点 `<!-- pin:<8hex> -->`（`Math.random` 16 进制 8 位，文件内碰撞概率可忽略），pin.read 暂不覆盖 research 文档（无自动注入路径），锚点为二期 pin.touch 预留
- **usage 查询**（不建新表不建物化列）：

```sql
SELECT json_extract(payload,'$.title') AS pin, COUNT(*) AS used, MAX(ts) AS last_used
FROM events WHERE kind='pin.read' AND change=:c GROUP BY pin ORDER BY used DESC;
-- 从未被注入的 pin：pin.write 的 title 集合 LEFT JOIN 上表 used IS NULL
```

- **constraint.inject 自报**（迁移基础，非 A1）：planInjection 返回值增加结构化 `docEntries: {path, mode, reason, bytes}[]`（docList 仅渲染用不变），before_agent_start 送达时逐条记账——注入原因（规则/关键词/JIT/档位绑定）是排查"为什么没注入某规范"的唯一数据源

### D6: TTL 与保留期

| kind | 保留期 | 理由 |
| --- | --- | --- |
| session.start | 90 天 | 会话锚点 |
| constraint.inject / subagent.dispatch / gate.check / pin.read | 30 天 | 诊断型 |
| pin.write | 永久 | 审计型，量小珍贵 |

保险丝 100MB 删最老一半（原样迁移）。`HarnessEventKind` 类型扩为六值。

### D7: 边界与卫生

- `.gitignore` 追加 `.pi/harness/`
- 写入仍为 WAL + synchronous=NORMAL + busy_timeout=5000，事件点同步单条 INSERT
- 单写者假设（pi 主/子线程同进程），WAL 兜底独立进程

## Risks / Trade-offs

- [A4 关键词映射脆弱——quota-gate 文案变更/新错误形态不命中] → unknown 兜底是诚实信号（dsh 纪律），配合观察项一个月后校准；不做启发式硬猜
- [pin.read 按 `## ` 解析遇手工编辑的 explore-findings 引入噪音标题] → 手工小节本就是被注入内容，记账语义成立；噪音可接受
- [detectActiveChange mtime 兜底在多 change 并行时会绑错] → 与 constraint-injection 既有行为一致（同源函数），不引入新误差面；bound 档位优先逻辑保留在原处
- [gate.check 写放大——每代码回合最多 ~6 行] → 量级每天几百行，30 天 TTL，可忽略
- [quality-gate 在非 git 仓库静默放行导致 gate 事件缺失] → 既有行为（无门禁运行=无账可记，语义自洽）
- [constraint-injection 返回结构改造引入回归] → docList 渲染路径不动，只增量返回 docEntries；烟测覆盖注入主路径

## Migration Plan

1. 无存量数据迁移——`.pi/harness/events.db` 是新文件，首次开库自动初始化（D2）
2. 回滚 = 删除 `.pi/harness/` 目录 + revert 扩展文件；无外部依赖
3. 部署后即生效（pi 重启加载扩展），无用户可见行为变化，无手动操作

## Open Questions

- A4 分类效果（用户指定观察项）：落地后一个月回看 `GROUP BY category` 分布与实际排障引用率，决定扩充/简化/放弃——不阻塞本 change
- pin.touch（模型主动 read pin 关联源文件）与 dump 命令、harness_trace：二期另立 change
