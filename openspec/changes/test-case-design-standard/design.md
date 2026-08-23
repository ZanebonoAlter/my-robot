# Design: 测试用例设计标准（test-case-design）

## Context

现有注入/门禁基础设施（见 proposal Why）：

- **constraint-injection**（`.pi/extensions/constraint-injection.ts`，1444 行）：档位（requirements/implementation）+ 关键词节级注入 + JIT 路径注入。JIT 单一真相源 = standard/flow 文档头部 15 行内的 `doc-impact-applies: path, ... | section=节名` 标签（`scanAppliesTags`，按 mtime 缓存）；命中 = 编辑路径 `p.includes(signal)` 子串匹配；一文档只支持一组 `{signals, section}`（多标签行后者覆盖前者）；命中会话内粘性只增不减；注入预算 32KB，超线分层降级 keyword → jit。
- **spec-gate**（`.pi/extensions/spec-gate.ts`，225 行）：拦 `openspec archive`，检查①doc-impact verify ②check-standards ③尾三节+doc-impact 标记 ④scenario-trace；失败 block；warning 留痕走 `pi.sendMessage({customType: "spec-gate-warning", display: true})`（steer 送达不打断回合）；豁免 `--force` / `SPEC_GATE_BYPASS=1`；自身异常 fail-open。
- **冒烟惯例**（`.pi/extensions/tests/`）：esbuild 打包 .ts → .cjs 后 node 跑断言（constraint-injection.smoke.cjs / harness-log.smoke.cjs 等）；spec-gate 目前无冒烟。
- **case-first-testing spec** 已要求算法/状态机/协议类 change 产出白盒用例文档，但无任何机制消费该要求。

## Goals / Non-Goals

**Goals:**

- 用例设计规范在「写 specs / 写 tasks / 写测试代码」三个时刻自动出现在 agent 上下文（知道层）
- 归档时对可机械判定的措辞违例 warn 留痕（兜底提醒层）
- 零/最小扩展改动：constraint-injection 完全不动，spec-gate 仅加一个 warn 检查

**Non-Goals:**

- 不做用例质量硬门禁（边界枚举「全不全」无法机械判，硬卡只会催生应付式打勾——质量靠注入 + 人审）
- 不改 scenario-trace / check-standards / quality-gate 等既有门禁语义
- 不自动生成测试用例或白盒用例文档（agent 按 checklist 自查，不代写）
- 不追溯修复所有历史 change 的 specs（试点仅 watch-keyword-and-quickadd）

## Decisions

### D1: JIT 注入用「摘要节」而非多标签行分节注入

机制现状：一文档只支持一组 `{signals, section}`。三组理想注入（specs→最低集节、tasks→措辞节、_test.go→分层判据节）需要三组标签，要么改 `scanAppliesTags` 支持数组（改公共机制、影响面大），要么合并。

**选定**：单一「JIT 注入摘要」节（三块速查合并，≤3KB），标签 `<!-- doc-impact-applies: openspec/changes/, _test.go, .spec.ts, .test.ts | section=JIT 注入摘要 -->`。宽命中即意图——编辑 specs 时看到措辞规范、编辑测试代码时看到最低集，都是该知道的事。

- 备选 1：扩展 scanAppliesTags 支持多标签行合并（signals union + 多 section）——机制更精确但改公共代码，收益不成比例，弃。
- 备选 2：拆三个小文档各挂各的节——单一权威源碎成三份，同步负担，弃。

### D2: signals 子串集合的误伤面评估

`p.includes(s)` 子串匹配下的信号选择：

- `openspec/changes/`：所有 change 制品编辑全命中（proposal/specs/tasks/design/test-cases）。宽，但摘要本就是通用设计规范，且档位语境下编辑 change 文件正是目标时刻。
- `_test.go`：后端测试文件全命中，无误伤（非测试文件不含该子串）。
- `.spec.ts` / `.test.ts`：前端 Vitest/Playwright 文件；`.spec.ts` 同时覆盖 `tests/e2e/*.spec.ts`（可接受，写 e2e 也该看设计规范）。

### D3: spec-gate 检查⑤为 warn 级 + 纯函数抽取

**warn 不 block**（用户拍板）：措辞违例误伤率不可预估（「解析」可能是简单解析也可能是算法档），warn + reason 内含「确非如此可忽略」即可。

**纯函数抽取**：扫描逻辑签名 `scanAcceptanceWording(tasksMd: string, changeDirFiles: string[]): string[]`（输入 tasks.md 文本 + change 目录文件名列表，输出违例文案数组），导出供冒烟测试 esbuild 打包直测；不 import fs、不触网。⑤a 判定 = 任务行文本命中复杂档关键词（算法|状态机|解析|协议|匹配引擎等，常量表）且文件列表无 `test-cases*.md` / `*-test-cases.md`；⑤b 判定 = 同一任务行（`- [ ]`/`- [x]` 到行尾）描述含「纯函数」且含「SQLite」。

- 备选：禁用词表从 test-design.md 运行时解析——解析脆弱（表格/代码块格式漂移即失效），弃；用「文档权威源 + 扩展内置常量 + 注释声明同步义务」（同 ARCHIVE_CONTEXT_RE 模式，词表就几条且稳定）。

### D4: 摘要节体积预算与降级路径

摘要节 ≤3KB 是硬约束（spec 已锁）。现状 implementation 档典型注入：约束索引 ~1KB + 域约束节 4-6KB + 摘要 3KB ≈ 8-10KB，远低于 32KB；极端多域命中时按既有分层降级（keyword 优先于 jit），无需新规则。

### D5: 试点 = 本 change 先吃狗粮，watch change 后续验证

- 本 change 的 specs 已按最低集写 Scenario（正常/拒绝/边界/兼容四维标注），tasks 措辞按新规范——落地后对照自查即第一例。
- `watch-keyword-and-quickadd` 的调试（补 test-cases.md / spec 补漏 Scenario / tasks 措辞修正）**不在本 change 交付物内**——那是该 change 自己的制品修正；本 change 验证节仅记录「试点执行」动作。

### D6: 标准内容按故事锚点定稿（方向对齐）

初稿曾以「方法边界清单」为重心，用户对齐纠偏：单盯一个方法的上下限跑通意义不大。定稿：测试单元锚在用户故事（Requirement 粒度），方法级单测降为快反馈双轨。四问句（节拍/变体/层/效果）吸收原六节全部内容；层选择新增 opencli 端到端层（与 §5.3 工具分流一致）。**二次升级（同日）**：spec 的 Scenario 是断言片段、讲不了完整故事——测试用例升格为 change 目录独立完整文档 `test-cases.md`（主链路表串节拍+变体+层落点），白盒从主角降为复杂档附加节。机器对账不动（scenario-trace 仍扫 tasks 验证节），检查⑤判定不变（test-cases*.md 名字恰好兼容，含义从白盒专用变宽为通用用例文档）。机制决策（D1-D4）不受影响。

## Risks / Trade-offs

- **摘要与全文双版本漂移**：摘要节是全文六节的速查版，存在改了全文忘改摘要的漂移风险。缓解：摘要节头部注明「速查版，权威全文见本文档主体，改主体必同步本节」+ 体积约束天然限制摘要独立演化。
- **禁用词双源漂移**（D3 备选说明）：词表常量与文档表格不同步。缓解：两侧注释互指 + 词表刻意保持极小（2 组规则）。
- **`解析` 类关键词误报**：简单字符串 split 也叫「解析」。接受：warn 级 + 「确非复杂档可忽略」措辞；若试点期误报率高，收紧词表（去「解析」保「算法|状态机|协议」）。
- **注入面变宽的 token 成本**：每编辑一次测试文件，会话内常驻 +3KB。接受：预算内，且这正是「知道」层的设计意图。
