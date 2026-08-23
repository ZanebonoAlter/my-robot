# Tasks: 测试用例设计标准（test-case-design）

> 垂直切片：A 权威源文档（核心交付物）→ B spec-gate 检查⑤ → C 前后端挂接 → D JIT 注入回归。本 change 的 tasks 自身按新标准编写（首个吃狗粮样本：验收全部可机械判定）。
>
> 📝 变更留痕（2026-08-24）：①方向对齐——标准从「方法边界清单」重构为「故事锚点四问句」（design D6）；②单元模型升级——测试用例升格为 change 目录独立完整文档 test-cases.md（spec Scenario 是断言片段讲不了完整故事，由用例文档串主链路+变体+层落点），白盒降为其中复杂档附加节（design D6 补记）。
>
> ⚠️ 依赖：无外部 change 依赖；constraint-injection / spec-gate 机制现状见 design.md Context。

## 1. 权威源文档 shared/test-design.md（A）

- [x] 1.1 撰写主体（故事锚点结构）：单元模型（测试单元=Requirement 用户故事，由 change 目录 test-cases.md 串成完整故事：主链路表步/动作/来源 Scenario/期望/层/落点；涉及行为的 change 必须有，纯文档/工具链豁免；双轨）、用例文档模板、问句①节拍完备、问句②变体走查（五组固定条目清单）、问句③层选择（最便宜层表+opencli 主链路落点）、问句④效果核对、验收措辞规范（可机械判定+禁用词表）、白盒附加（复杂档模板）。验收：`grep -c '^## '` 文档 ≥8，每节标题逐字命中清单
- [x] 1.2 撰写「JIT 注入摘要」节：故事锚点结构速查版（单元模型一句 + 四问句各一条 + 变体五组条目 + 层表 + 措辞黑名单 + opencli 提醒），节头部注明「速查版，权威全文见本文档主体，改主体必同步本节」。验收：`awk` 截取该节字节数 ≤3072
- [x] 1.3 文档头部（前 15 行内）加标签 `<!-- doc-impact-applies: openspec/changes/, _test.go, .spec.ts, .test.ts | section=JIT 注入摘要 -->`。验收：`grep -n "doc-impact-applies" 文档` 命中且含四信号与 section
- [x] 1.4 验收措辞规范节内含禁用词表（⑤a 复杂档关键词 / ⑤b 纯函数×SQLite 两组），表侧注明「.pi/extensions/spec-gate.ts 内置同表常量，改此表必同步」。验收：grep 禁用词表小节存在且含同步注释

## 2. spec-gate 检查⑤（B · warn 级）

- [ ] 2.1 抽纯函数 `scanAcceptanceWording(tasksMd, changeDirFiles) []string`（不 import fs/不触网）：⑤a 任务行命中复杂档关键词常量表 且 文件列表无 `test-cases*.md`/`*-test-cases.md`；⑤b 同一任务行（`- [ ]`/`- [x]` 起）描述含「纯函数」且含「SQLite」。常量表旁注释指向 shared/test-design.md 权威源。验收：esbuild bundle 成功 + 冒烟断言通过（见 2.3）
- [ ] 2.2 集成进 spec-gate 检查链：检查⑤结果仅走 `spec-gate-warning` custom_message 留痕（display: true），SHALL NOT 进入 block 判定；豁免路径（--force/SPEC_GATE_BYPASS）在⑤之前短路；⑤整体 try/catch fail-open。验收：代码 review 确认 warn 不进 block 分支 + 冒烟通过
- [ ] 2.3 新增 `.pi/extensions/tests/spec-gate.smoke.cjs`（esbuild 打包惯例，参照 harness-log.smoke.cjs 结构）：断言①无违例输入 → 空列表 ②⑤a 构造样本（含「解析」任务行 + 空文件列表）→ 1 条 warning 且文案含「可忽略」③⑤b 构造样本（纯函数+SQLite 同行）→ 1 条 warning ④健壮性（空串/超长行/只有标签无内容）→ 不抛异常。验收：`node spec-gate.smoke.cjs` 退出码 0
- [ ] 2.4 把 spec-gate 冒烟挂进 `.pi/extensions/tests/run-harness-smoke.sh`（esbuild 打包 + node 执行 + 清理临时 .cjs）。验收：`bash .pi/extensions/tests/run-harness-smoke.sh` 退出码 0

## 3. 前后端挂接 + 索引（C）

- [x] 3.1 `docs/reference/standard/backend/testing.md` 新增「用例设计（测什么）」节：引用 shared/test-design.md 权威源 + 后端分层判据（纯函数→`*_unit_test.go` 无 DB / repository 与迁移→testcontainer PG / handler→轻量）。验收：grep 节标题 + 引用路径命中；diff 确认既有内容零删改（仅新增节）
- [x] 3.2 `docs/reference/standard/frontend/testing.md` 新增「用例设计（测什么）」节：引用 shared + 前端判据（纯函数→单测 / 组件行为→Vitest 组件测试 / 流程→opencli）。验收：同上
- [x] 3.3 `docs/reference/constraints-index.md` 执行规范表加 test-design 行。验收：grep "test-design" 命中

## 4. JIT 注入回归（D · 依赖 1）

- [x] 4.1 扩展 `.pi/extensions/tests/constraint-injection.smoke.cjs`：断言①scanAppliesTags 结果含 test-design.md 条目且 signals 含四信号、section=「JIT 注入摘要」②该节标题在文档正文中存在（标签与正文漂移即红）③模拟编辑路径含 `_test.go` 时命中该文档（jit 命中路径逻辑）。验收：`bash .pi/extensions/tests/run-smoke.sh` 退出码 0
- [ ] 4.2 真机验证（手动）：编辑任一 `openspec/changes/*/tasks.md` 后下一回合注入块含摘要节（档位激活会话）。验收：人工确认注入块含「JIT 注入摘要」，记入本文件验证节留痕

## 5. 架构体检（§7 强制）

- [ ] 5.1 `codegraph impact scanAcceptanceWording` + `codegraph impact runChecks`（spec-gate 检查链）：波及面无 HIGH/CRITICAL 忽略。验收：impact 输出记录进本文件验证节
- [ ] 5.2 §7.2 zoom-out：检查⑤不改变注入/门禁分层秩序（注入管知道/门禁管做到；⑤是 warn 不属硬门禁），无循环依赖。验收：design D3 与实现一致，review 确认

## 6. 文档

<!-- doc-impact: standard -->

- [x] 6.1 `docs/reference/开发执行规范.md` §2「用例先行」表格下方补一句引用：用例设计最低集/边界/可用性 checklist 见 `standard/shared/test-design.md`（JIT 自动注入）。验收：grep 引用命中；§2 既有表格零删改
- [ ] 6.2 standard 域文档更新即任务 1/3 交付物，无额外动作。验收：`bash scripts/doc-impact.sh verify openspec/changes/test-case-design-standard` 退出码 0

## 7. 测试

> 冒烟为 pi 扩展层验证（无独立 typecheck 入口，esbuild bundle + node 断言即编译+行为验证）。

- [ ] T.1 `bash .pi/extensions/tests/run-smoke.sh`（constraint-injection 冒烟含 4.1 新断言）→ 退出码 0
- [ ] T.2 `bash .pi/extensions/tests/run-harness-smoke.sh`（含 2.3/2.4 spec-gate 冒烟）→ 退出码 0
- [ ] T.3 `bash scripts/scenario-trace.sh openspec/changes/test-case-design-standard` → 退出码 0（Scenario 映射表见验证节）
- [ ] T.4 `bash scripts/check-standards.sh` → 零失败（A-D 段）

## 8. 验证

- [ ] V.1 `grep -n '^## ' docs/reference/standard/shared/test-design.md` → 单元模型 + 四问句 + 验收措辞 + 白盒补充 + JIT 摘要 ≥8 节标题逐字命中（Scenario「权威源结构齐全」）
- [ ] V.2 摘要节字节数 ≤3072（awk 截取 wc -c）
- [ ] V.3 `grep -n "doc-impact-applies" docs/reference/standard/shared/test-design.md` → 四信号 + section 命中
- [ ] V.4 `grep -n "test-design" docs/reference/standard/backend/testing.md docs/reference/standard/frontend/testing.md docs/reference/constraints-index.md` → 各至少 1 命中；两份 testing.md diff 仅新增节零删改
- [ ] V.5 codegraph impact 输出（5.1）粘贴留痕于此
- [ ] V.6 真机注入确认（4.2）留痕于此：日期 + 会话 + 注入块节选
- [ ] V.7 试点计划：本 change 归档后，`watch-keyword-and-quickadd` apply 时按新标准补 spec 漏 Scenario（空串/纯分隔符/14 天边界）、建 test-cases.md 白盒边界矩阵、修 tasks 措辞（「SQLite 单测覆盖」→「单元测试覆盖（无 DB）」），作为规范首个完整试点；观察点：注入时机是否赶得上写作、⑤a 关键词误报率

### Scenario → 测试映射

| Scenario | 测试文件 |
| --- | --- |
| 权威源结构齐全 | 人工：V.1 grep 节标题逐字命中（单元模型+四问句+措辞+白盒+摘要） |
| 故事锚点与对账同锚 | 人工：单元模型节与 scenario-trace 同锚表述，V.1 节标题命中 |
| 用例文档串故事 | 人工：用例文档模板节含主链路表列（步/动作/来源Scenario/期望/层/落点），V.1 节标题命中 |
| 前端交互故事 opencli 落点 | 人工：问句③层选择节含 opencli 主链路落点要求，V.1 节标题命中 |
| 摘要节体积受控 | 人工：V.2 wc -c ≤3072 |
| 与既有规范的关系 | 人工：V.4 diff 仅新增零删改，无「怎么跑」内容复写 |
| 编辑 specs 时注入 | .pi/extensions/tests/constraint-injection.smoke.cjs |
| 编辑测试代码时注入 | .pi/extensions/tests/constraint-injection.smoke.cjs |
| research 语境不注入 | 人工：标签信号不含 docs/research/，机制既有负路径 |
| 边界——注入预算超线降级 | 人工：既有分层降级机制；V.2 保摘要 ≤3KB 不主动触线 |
| 无违例静默放行 | .pi/extensions/tests/spec-gate.smoke.cjs |
| 白盒用例缺失 warn 留痕 | .pi/extensions/tests/spec-gate.smoke.cjs |
| 纯函数任务提 SQLite warn 留痕 | .pi/extensions/tests/spec-gate.smoke.cjs |
| 检查⑤异常不阻断归档 | .pi/extensions/tests/spec-gate.smoke.cjs |
| 豁免通道兼容 | 人工：豁免路径短路先于检查⑤，任务 2.2 代码 review 确认 |
| 后端判据可查 | 人工：V.4 grep 命中 |
| 索引可见 | 人工：V.4 grep 命中 |
| 既有权威内容不变 | 人工：V.4 diff 比对 |
