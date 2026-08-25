# Proposal: 测试用例设计标准（test-case-design）

## Why

测试用例的编写目前没有设计层规范：`standard/*/testing.md` 管「怎么跑」（框架/分层/DSN 红线），`case-first-testing` spec 管「Scenario 即用例 + 复杂档白盒用例」，但从「测什么、测到什么程度算够、验收措辞怎么写才算可判定」全靠现场发挥。以 `watch-keyword-and-quickadd` 为解剖样本可见四类缺口：

- **黑盒用例与任务验收脱节**：任务 1.3 验收提「空串降级」，spec.md 无对应 Scenario（scenario-trace 对账范围外，纯靠临场）
- **边界全集无人枚举**：keyword 表达式解析的边界（纯空格 / `|` 开头结尾 / 连续 `||` / 全角空格 / 14 天窗口边界）spec 与 tasks 均未覆盖；case-first-testing 明明要求算法类 change 产出白盒用例文档，该 change 目录里没有——**现行规范已被违反但无人拦**
- **验收措辞随性**：纯函数任务验收写「SQLite 单测覆盖」（分层错配，照抄会真的去建 SQLite 库）
- **可用性用例缺位**：误输入反馈 / 空态 / 错误态类用例前后端规范均无此类目

根因：规范即便存在也没在「写 specs / 写 tasks / 写测试代码」这三个时刻被 agent 看到——「知道」层（注入）缺位。

## What Changes

- 新建 `docs/reference/standard/shared/test-design.md`：**用例设计标准唯一权威源**（跨前后端），**故事锚点四问句**结构——单元模型（测试单元 = 用户故事 = spec 一个 Requirement + 其 Scenario 节拍；双轨：方法级单测允许且鼓励，交付账本在故事层）、问句①节拍完备（每 Scenario 有测试落点）、问句②变体走查（输入 / 前置数据状态 / 时间窗口 / 幂等重复 / 可用性五组固定条目清单，不适用划除留痕）、问句③层选择（「能讲完这个故事的最便宜层」：函数单测 → testcontainer PG → handler → 组件测试 → opencli 端到端；**前端交互故事主链路至少一个 opencli 落点**）、问句④效果核对（真实库量化核对格式）、验收措辞规范（可机械判定 + 禁用词表）、白盒用例补充件（复杂档降级为补充）；正文含 ≤3KB「JIT 注入摘要」速查节
- **constraint-injection 零代码改动**：新文档头部带 `doc-impact-applies: openspec/changes/, _test.go, .spec.ts, .test.ts | section=JIT 注入摘要` 标签，靠既有 JIT 机制实现「编辑 specs/tasks/测试代码时自动注入摘要」
- **spec-gate 新增检查⑤（warn 不 block）**：⑤a 复杂档白盒用例文档缺失提醒（任务文本含算法/状态机/协议关键词 + change 目录无 `test-cases*.md` → warning 留痕）；⑤b 分层错配措辞（「纯函数」任务验收含「SQLite」→ warning 留痕）。均不阻断归档，扫描逻辑抽纯函数供冒烟测试
- `standard/backend/testing.md` / `standard/frontend/testing.md` 各补「用例设计」小节引用 shared 权威源；`constraints-index.md` 执行规范表更新
- **回归走查与旧资产可见性（并入批）**：test-design.md 加「问句⓪ 回归走查」（涉及 MODIFIED/REMOVED Requirements 的 change 必答：test-cases.md 继承与调整表——旧 Scenario × 处置 × 旧测试文件 × 动作）；新建 `scripts/test-assets.sh <capability>` 反向索引（主 specs 现状节拍 + archive 含 test-cases*.md 的历史 change + 历史 Scenario→测试映射重建，只读只判不猜）；摘要节加一句提醒
- **试点**：规范落地后以 `watch-keyword-and-quickadd`（历史遗留 change，43 任务未动）为首个用户验证注入与提醒效果；本 change 的 specs/tasks 自身按新标准编写（首个吃狗粮样本）

## Capabilities

### New Capabilities

- `test-case-design`: 用例设计标准——权威源文档（shared/test-design.md）、设计规范 JIT 注入（写 specs/tasks/测试代码时刻可见）、归档措辞 warn 检查（spec-gate 检查⑤）

### Modified Capabilities

（无——`case-first-testing` / `constraint-injection` / `scenario-trace-gate` 的 requirements 均不变。本 change 是新增独立能力，全部消费既有机制：JIT 靠 doc-impact-applies 标签单一真相源，warn 留痕复用 spec-gate-warning sendMessage 模式。）

## Impact

- **新文档**：`docs/reference/standard/shared/test-design.md`
- **新脚本**：`scripts/test-assets.sh`（capability 测试资产反向索引，只读）
- **改文档**：`standard/backend/testing.md`、`standard/frontend/testing.md`（补引用节）、`constraints-index.md`（how 表加行）、`开发执行规范.md` §2/§4.1（按需补一句引用，不改门禁语义）
- **改代码**：`.pi/extensions/spec-gate.ts`（新增 warn 级检查⑤ + 纯函数扫描逻辑）；新增 `.pi/extensions/tests/spec-gate.smoke.cjs`（冒烟）
- **不改**：`.pi/extensions/constraint-injection.ts`（零改动）
- **风险**：① JIT 摘要占注入预算（32KB 中新增 ~3KB）——摘要节体积受控 + 既有分层降级兜底；② 禁用词误伤——warn 级 + reason 措辞含「确非如此可忽略」；③ `openspec/changes/` 信号面较宽（编辑任何 change 文件都注入）——摘要为通用设计规范，宽命中即设计意图
- **无业务域声明**（纯工具链/文档 change，`<!-- constraint-domains -->` 留空属预期）
