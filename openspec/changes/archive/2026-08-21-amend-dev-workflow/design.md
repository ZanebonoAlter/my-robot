# Design: amend-dev-workflow

## Context

- 现状：`docs/experience/` 混放踩坑复盘与调研资料（extensions-research 实为调研）；调研驱动型 change 的数据源不留存（add-change-scope 首次补了 research.md 但无规则依据）。
- `开发执行规范.md` §2 为严格 TDD（"没有失败测试不写产品代码"+红旗清单），源于 superpowers test-driven-development skill 的单人 red-green 节奏；主线程派发子线程执行的编排下，"看到失败再写实现"意味着每个功能至少两轮完整子线程往返，token 成本高且实现中途调整即返工。
- 参考：deepseek-harness 用 spec/快照/Agent Notes 分离"决策 why"与"实现 when"（见 `../add-change-scope/research.md` §3.1/3.6）；openspec spec.md 的 Scenario 本身就是结构化行为用例，目前未被当作用例资产使用。

## Goals / Non-Goals

**Goals:**
- 调研数据可追溯：任何 change 的采纳决策能回查原始数据与关键代码，新会话不重爬仓库。
- 用例设计与实现解耦：把"想清楚测什么"前置到 specs 阶段，"写测试代码"与"写实现"并行自由。
- 测试覆盖不因顺序解绑而下滑。

**Non-Goals:**
- 不做完整 Agent Notes 决策记录体系（openspec change 已承担决策记录职责）。
- 不改测试基建（testutil/testcontainer/test 分层等归 `test-infrastructure` spec，不动）。
- 不动 doc-impact.sh（context 子命令去留归 `port-constraint-injection`）。

## Decisions

### 1. 调研两级落点，以「change 归属」为判据

| 情形 | 落点 | 理由 |
|---|---|---|
| 调研直接驱动某 change | `openspec/changes/<name>/research.md` | 随 change 归档永久保留；proposal 引用形成数据链；不会污染无关会话 |
| 无 change 归属（通用调研/选型对比） | `docs/research/<topic>.md` | 跨 change 复用；topic 命名 kebab-case |
| 踩坑/事故/复盘 | `docs/experience/`（维持现状定位） | 事后教训与事前数据分家 |

**替代方案**（否决）：全部集中一个 research 库再靠链接关联——change 归档后链接断层数据流浪；全部进 change——通用调研被单个 change 埋掉无法复用。

research.md 内容要求：调研方式、关键发现、**关键代码摘录（带源路径+快照日期）**、采纳/不采纳决策表。前两者自由，后两者 MUST（这正是防语义偏移的核心）。

### 2. 用例先行替代严格 TDD：设计前置、顺序解绑、对账兜底

流程：

```
specs 阶段（已有）: Scenario（WHEN/THEN）= 黑盒行为用例 ← 用例设计发生在这里
     ↓ 复杂逻辑？（状态机≥3状态 / 算法 / 多模块协议）
     是 → 派子线程枚举白盒用例（分支表/边界值清单）→ 落 change 目录用例文档
     否（简单 CRUD）→ 直接进实现
     ↓
测试代码 + 实现顺序解绑（可先可后可同 PR 交织），不再强制"先看到失败"
     ↓
归档门禁: tasks.md 验证节列 Scenario→测试文件映射（人工核对 + grep 佐证）
```

**为什么不是纯"后补测试"**：用例设计（specs 阶段）必须先于实现——想清楚测什么再动手，这个前置不放松；放松的只是"测试代码必须先跑红"这一仪式。dsh 的等价实践：快照测试与实现同 PR、按 diff 选最小验证集（research.md §3.6）。

**两条底线（不可豁免，写进 Requirement）**：
- bug 修复先写复现测试（先复现才能证明修的是这个 bug，与流程效率无关，是正确性问题）
- 断言判据主线程定（ui-verify skill 已有教训：让子代理判断"功能对不对"会说瞎话；子线程只做机械枚举）

**替代方案**（否决）：保留严格 TDD 但只在"复杂 change"放宽——判据模糊（什么算复杂），执行期必然漂移回全靠自觉。

### 3. §2 红旗清单改写而非删除

原红旗（"先写实现后补测试/测试立即通过没看到失败"）与新流程冲突，替换为新红旗：动手前 specs 无 Scenario / 白盒用例文档缺失（复杂逻辑档）/ 归档验证节无 Scenario→测试映射 / bug 修复无复现测试。保留"红旗即停"的执行力度。

### 4. 迁移顺手做：extensions-research → docs/research/extensions

`git mv` 一次到位，路径引用仅 docs 内部（grep 确认无代码引用），无风险。

## Risks / Trade-offs

- [顺序解绑后测试被拖到最后一刻烂尾] → 归档门禁映射表对账：无映射的 Scenario 视为未覆盖，FAIL。
- [子线程白盒用例枚举质量参差] → 判据红线（主线程定断言）+ 主线程核验子线程产物（§0.6 既有纪律）。
- [research.md 变成随手粘贴的垃圾场] → 内容要求里"关键代码摘录必带源路径+快照日期"写入 spec，check 类比照格式。
- [docs/research/ 与 docs/experience/ 边界再次模糊] → §0.5 文档归属规则显式两行判据（事前数据 vs 事后教训）。

## Migration Plan

纯文档 + 目录迁移，无回滚复杂度。旧严格 TDD 条文与新条文同 PR 替换，无过渡期。

## Open Questions

（无）
