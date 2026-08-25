# design — constraint-injection-tier-b

## Context

constraint-injection（移植自源项目，change：port-constraint-injection）+ harness-facts-tier-a（事实库）已落地。本 change 落地 dsh 调研 B 级判定中仅有的两条可抄项（判定回写：docs/research/harness-survey/findings.md）：

- **B6 预算**：注入块无总量上限，实测 9 个 flow 节全命中 43K、最坏 55K+ 常驻 system prompt 每 turn；`digestDocThreshold` 因文档无「硬规则速览」节从未生效
- **B8 衍生（档位持久化）**：`session_start` 对 resume 清零档位，apply 中断恢复后掉回未激活

架构前提（判定 B5/B8 原建议不抄的原因）：本仓注入是**每 turn 重建式**（幂等、天然完整替换），非 dsh 追加式，无去重/对账问题。

## Goals / Non-Goals

**Goals:**

- 注入块总量有界（`budgetBytes`，默认 16384），超线分层降级，**永不真丢**（占位行 + read 指引）
- 降级确定性：相同命中集合 + 内容 + 预算 → 字节级相同结果
- 模型可见省略通知 + 事实库降级记账（与 A 级 usage 统计闭环）
- 档位状态持久化（`mode.set` 事件），resume 恢复，new/fork/reload 清零语义不变

**Non-Goals:**

- 不做 SHA-1 digest 去重 / baselineIdentity 对账（B5、B8 原建议——每 turn 重建下无对应问题）
- 不做 `</system-reminder>` 转义（B7 暂缓：无 XML 框架、注入源全可信；触发条件记 findings.md）
- 不改六类既有事件的语义与 TTL；不做 DB schema 迁移（kind 是 TEXT 列，新 kind 零迁移）
- 不做约束文档自动瘦身（只产出"常被降级"数据，瘦身是后续人的决策）

## Decisions

### D1 预算口径与配置语义

- 度量 = 渲染后注入块的 UTF-8 字节（header + 全部 sections，含占位行），与 dsh `maxBytes` 口径一致；AGENTS.md 本体、system prompt 其余部分不管
- 配置 `budgetBytes` 可选，默认 32768（实现期重估：16K 会在常态上界（findings+词汇表双 6K + 2~3 域节约 10K）触发 keyword 降级，违背「预算管尾部异常不管常态」；且烟测 REPO 场景积累 5 命中 ≈ 24K 会落入边际降级区，断言随 flow 文档演进漂移。32K = 常态上界 + 余量，仍截断实测 43~55K 病态尾部）；**非正数 = 显式禁用预算**（维持现状行为，防手滑配 0 把注入干没）

### D2 降级顺序：先推断信号后声明信号（dsh "先宽泛后具体"原则映射）

（rebase 注：cdd 引入域声明节后，降级分层必须覆盖四种命中原因。信号强度排序：keyword（对话推断，最弱）< jit（代码路径推断兜底）< domain 声明（change 作者显式声明）≈ change-file（findings/词汇表，实现档核心输入）。）

超线时降级顺序（与信号强度相反，先降最弱信号）：

1. **keyword 命中节** → 占位行（依首次命中顺序逐个降，降一个重测一次，进预算即停）
2. **jit 命中节** → 占位行（同上）
3. **explore-findings / 词汇表** → 强制 digest（无视 `findingsDigestThreshold`，用现有 `digestFindings`）
4. **域声明节** → 占位行（声明域每回合重解析，降级同样确定性）
5. **explore-findings / 词汇表** → 占位行（保留 `📌/📖` 标题 + read 路径）

baseDocs（index）与注入块 header **永不降级**。

占位行格式（实现期微调：预算数字只出现在 header 降级行与 widget，占位行不重复——每条自带 X/Y 会让字节随降级进度摄动）：`### <显示名>（原 heading 恒在）\n（预算已满，全文已省略）\n> <节名/首行预览 ≤120 字>\n> 全文可 \`read <相对路径>\``

**永不真丢不变量**：每个命中文档在注入块中至少存在一个标题占位，即使预算小到只装得下占位行集合。

### D3 降级算法：两遍装配纯函数

`applyBudget(sections, entries, budgetBytes)` 抽为纯函数（输入有序条目列表，输出降级后列表 + 降级记录）：

- 第一遍全量装配测字节，未超 → 原样返回（零开销路径，绝大多数回合）
- 超线 → 按 D2 顺序逐层降级重测
- 确定性来源：输入顺序（既有 ordered 列表 = 首次命中顺序）+ 纯函数无随机无时间依赖

**与前缀缓存的交互**：粘性命中（只增不减）保证注入块单调增长；增长推过预算线的那一回会触发降级 → 该回合前缀变化一次（缓存部分失效），此后稳定。这是预算的固有代价，dsh 同样存在（其 baselineIdentity 显式含预算派生）。不试图消除，靠确定性把抖动限制为"每次推线一次"。

### D4 降级记账与通知

- `constraint.inject` payload 增 `degraded: true` + 降级后字节数（未降级回合不增字段，既有六类事件的消费方零感知）
- 模型可见：header 增一行「已降级：a.md(x节)、b.md…」；widget 增「预算 12.3K/16K」
- 月度 SQL：`GROUP BY path` 筛 `degraded=1` 占比 → 常被挤掉的节 = 瘦身候选（闭环 A 级 usage 统计）

### D5 mode.set 记账

- 触发点三处（与现有 mode 设置分支同位）：input 命令命中、tool_execution_start skill 路径命中、change 绑定修正（提及/写 change 目录）
- payload `{ mode, boundChange }`；change 列同步写 boundChange（`queryByChange` 天然可查）
- 每次设置/修正各记一条（不 diff、不去重——审计语义是"何时设的"，恢复取"最近一条"）
- 记账失败不阻断档位激活（logEvent 已 fail-loud 返回 false，调用方静默即可——档位激活是主路径）

### D6 会话边界恢复（resume / reload / startup 三路径；真实链路返工后定稿）

**真实链路教训（6.5 实测）**：quit 后重启 pi 恢复会话，session_start 的 reason 是 **"startup"** 而非 "resume"（2026-08-23T07:46:41 事件实证），只挂 resume 的恢复在最常见路径上失效。startup 双面孔：pi 进程冷启动恢复会话 vs pi-subagents 派发（共用模块实例，@bugfix：不得清零主会话状态）。区分锹点 = 档位是否已非空（冷启动模块状态本就为空；子线程派发时主会话档位非空）。

| reason | 清零？ | 恢复取数 |
| --- | --- | --- |
| resume（/resume 切换目标会话） | 是 | 两段式：同 sessionId 最近一条（`queryBySession`）→ 全局最新（`queryLatestByKind`，兼顾 resume 生成新 id） |
| reload（同会话扩展重载，id 不变） | 是 | 同上（第 1 段必中，走同一实现） |
| startup（冷启动恢复会话 / 子线程派发） | 否（@bugfix） | 仅档位为空时：同 sessionId 第 1 段 only，**禁止全局兜底**（全新会话不得继承其他会话档位） |
| new / fork | 是 | 不恢复 |

均无/不可恢复（无记录、过期、绑定 change 目录不存在）→ 回落未激活。不设新鲜度窗口：30 天 TTL 已是天然窗口，双重窗口徒增配置。

**粘性命中集合不恢复（有意为之）**：恢复只回档位 + change 绑定，keyword/JIT 粘性命中集合不持久化——域声明节照常注入（每回合重解析）、JIT 改代码即重新命中、keyword 对话提及即命中，正确性无损；且避免恢复出一个与"只增不减"历史记录不一致的半状态（缓存反而更稳）。

### D7 烟测扩展

`tests/run-smoke.sh`（.cjs 纯函数直跑模式）增断言组：

- 预算内零降级（字节不变）
- 超线分层降级顺序正确（keyword 先于 jit、findings digest 后置）
- 永不真丢（budgetBytes=2048 极端值下每个文档仍有占位行）
- 确定性（同输入两次装配字节一致）
- resume 恢复 / 无法恢复回落 / new 不恢复（stub session_start 事件 + 内存 sqlite 或注入 fake logEvent）

## Risks / Trade-offs

- [降级后模型缺完整约束] → 占位行带节名 + read 路径（pi 有 read 工具可自取）；降级记账月度回看反哺瘦身
- [推线回合前缀缓存抖一次] → D3 确定性保证仅抖一次；预算默认值取常态命中集合的 P90 余量
- [mode.set 事件量] → 低频（每 change 数条）+ 30 天 TTL，可忽略
- [resume 误绑他档位]（D6 第 2 段）→ boundChange 存在性校验 + 单用户串行场景；误绑的后果=多注入约束，非数据损坏
- [budgetBytes 默认值拍脑袋] → 上线后按 constraint.inject 的 bytes 分布调；有记账数据后这是观测问题而非设计问题

## Migration Plan

纯增量：无 DB 迁移（kind 为 TEXT 列）、配置新增可选字段向后兼容、既有六类事件零影响。回滚 = revert `.pi/extensions/` 两文件 + 配置字段（残留 `mode.set` 行由 TTL 自然清扫）。

**归档顺序依赖（blocker 级）**：本 change 的「每 turn system prompt 强制注入」delta 已 rebase 至 constraint-domain-declaration（cdd）的目标状态（cdd 实施完未归档时基于其 delta 文本合成）。**cdd MUST 先归档，本 change 后归档**——反序会把主 spec 整体替换回不含 cdd 变更的文本，静默回滚域声明机制。归档前在 tasks 验证节核对 cdd 已在 archive/ 下。

## Open Questions

- `budgetBytes` 默认 16384 的合理性——上线两周后按 `constraint.inject` bytes 分布复评（预期常态命中 2~3 个 flow 节 + index + findings digest ≈ 12~14K，默认值留 ~20% 余量）
