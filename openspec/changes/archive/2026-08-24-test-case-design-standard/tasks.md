# Tasks: 测试用例设计标准（test-case-design）

> 垂直切片：A 权威源文档（核心交付物）→ B spec-gate 检查⑤ → C 前后端挂接 → D JIT 注入回归。本 change 的 tasks 自身按新标准编写（首个吃狗粮样本：验收全部可机械判定）。
>
> 📝 变更留痕（2026-08-24）：①方向对齐——标准从「方法边界清单」重构为「故事锚点四问句」（design D6）；②单元模型升级——测试用例升格为 change 目录独立完整文档 test-cases.md（spec Scenario 是断言片段讲不了完整故事，由用例文档串主链路+变体+层落点），白盒降为其中复杂档附加节（design D6 补记）；③回归走查三件套并入——问句⓪ + 继承表（放 test-cases.md）+ scripts/test-assets.sh 反向索引（design D7，跨 change 断链：改契约时旧测试可能仍断言旧契约，跑绿≠对）。
>
> ⚠️ 依赖：无外部 change 依赖；constraint-injection / spec-gate 机制现状见 design.md Context。

## 1. 权威源文档 shared/test-design.md（A）

- [x] 1.1 撰写主体（故事锚点结构）：单元模型（测试单元=Requirement 用户故事，由 change 目录 test-cases.md 串成完整故事：主链路表步/动作/来源 Scenario/期望/层/落点；涉及行为的 change 必须有，纯文档/工具链豁免；双轨）、用例文档模板、问句①节拍完备、问句②变体走查（五组固定条目清单）、问句③层选择（最便宜层表+opencli 主链路落点）、问句④效果核对、验收措辞规范（可机械判定+禁用词表）、白盒附加（复杂档模板）。验收：`grep -c '^## '` 文档 ≥8，每节标题逐字命中清单
- [x] 1.2 撰写「JIT 注入摘要」节：故事锚点结构速查版（单元模型一句 + 四问句各一条 + 变体五组条目 + 层表 + 措辞黑名单 + opencli 提醒），节头部注明「速查版，权威全文见本文档主体，改主体必同步本节」。验收：`awk` 截取该节字节数 ≤3072
- [x] 1.3 文档头部（前 15 行内）加标签 `<!-- doc-impact-applies: openspec/changes/, _test.go, .spec.ts, .test.ts | section=JIT 注入摘要 -->`。验收：`grep -n "doc-impact-applies" 文档` 命中且含四信号与 section
- [x] 1.4 验收措辞规范节内含禁用词表（⑤a 复杂档关键词 / ⑤b 纯函数×SQLite 两组），表侧注明「.pi/extensions/spec-gate.ts 内置同表常量，改此表必同步」。验收：grep 禁用词表小节存在且含同步注释

## 2. spec-gate 检查⑤（B · warn 级）

- [x] 2.1 抽纯函数 `scanAcceptanceWording(tasksMd, changeDirFiles) []string`（不 import fs/不触网）：⑤a 任务行命中复杂档关键词常量表 且 文件列表无 `test-cases*.md`/`*-test-cases.md`；⑤b 同一任务行（`- [ ]`/`- [x]` 起）描述含「纯函数」且含「SQLite」。常量表旁注释指向 shared/test-design.md 权威源。验收：esbuild bundle 成功 + 冒烟断言通过（见 2.3）
- [x] 2.2 集成进 spec-gate 检查链：检查⑤结果仅走 `spec-gate-warning` custom_message 留痕（display: true），SHALL NOT 进入 block 判定；豁免路径（--force/SPEC_GATE_BYPASS）在⑤之前短路；⑤整体 try/catch fail-open。验收：代码 review 确认 warn 不进 block 分支 + 冒烟通过
- [x] 2.3 新增 `.pi/extensions/tests/spec-gate.smoke.cjs`（esbuild 打包惯例，参照 harness-log.smoke.cjs 结构）：断言①无违例输入 → 空列表 ②⑤a 构造样本（含「解析」任务行 + 空文件列表）→ 1 条 warning 且文案含「可忽略」③⑤b 构造样本（纯函数+SQLite 同行）→ 1 条 warning ④健壮性（空串/超长行/只有标签无内容）→ 不抛异常。验收：`node spec-gate.smoke.cjs` 退出码 0
- [x] 2.4 把 spec-gate 冒烟挂进 `.pi/extensions/tests/run-harness-smoke.sh`（esbuild 打包 + node 执行 + 清理临时 .cjs）。验收：`bash .pi/extensions/tests/run-harness-smoke.sh` 退出码 0

## 3. 前后端挂接 + 索引（C）

- [x] 3.1 `docs/reference/standard/backend/testing.md` 新增「用例设计（测什么）」节：引用 shared/test-design.md 权威源 + 后端分层判据（纯函数→`*_unit_test.go` 无 DB / repository 与迁移→testcontainer PG / handler→轻量）。验收：grep 节标题 + 引用路径命中；diff 确认既有内容零删改（仅新增节）
- [x] 3.2 `docs/reference/standard/frontend/testing.md` 新增「用例设计（测什么）」节：引用 shared + 前端判据（纯函数→单测 / 组件行为→Vitest 组件测试 / 流程→opencli）。验收：同上
- [x] 3.3 `docs/reference/constraints-index.md` 执行规范表加 test-design 行。验收：grep "test-design" 命中

## 4. JIT 注入回归（D · 依赖 1）

- [x] 4.1 扩展 `.pi/extensions/tests/constraint-injection.smoke.cjs`：断言①scanAppliesTags 结果含 test-design.md 条目且 signals 含四信号、section=「JIT 注入摘要」②该节标题在文档正文中存在（标签与正文漂移即红）③模拟编辑路径含 `_test.go` 时命中该文档（jit 命中路径逻辑）。验收：`bash .pi/extensions/tests/run-smoke.sh` 退出码 0
- [x] 4.2 真机验证：events.db 证据见验证节 V.6（constraint.inject, reason=jit-path, mode=section）

## 5. 回归走查三件套（E · 并入批）

- [x] 5.1 test-design.md 加「问句⓪：改契约了吗（回归走查）」节（触发条件 / 继承与调整表四列：旧 Scenario×处置×旧测试文件×动作 / 豁免规则）+ 模板加「### 继承与调整（涉及 MODIFIED/REMOVED Requirements 时必填）」节。验收：grep 节标题命中且模板表头四列逐字命中
- [x] 5.2 摘要节加问句⓪一句（触发条件 + 继承表 + test-assets.sh 指引）。验收：awk 截取 ≤3072 且含「test-assets.sh」「继承与调整」
- [x] 5.3 新建 `scripts/test-assets.sh <capability>`（只读只判不猜）：①主 specs 现状节拍清单 ②archive 含该 capability delta 且含 test-cases*.md 的 change ③历史 tasks.md 验证节映射重建。验收：对 `scenario-trace-gate` 查询三段输出 + 退出码 0
- [x] 5.4 脚本负路径：对不存在的 capability 查询 → 退出码非 0 且中文提示。验收：`bash scripts/test-assets.sh no-such-cap; echo $?` 非 0 且无空段充数

## 6. 架构体检（§7 强制）

- [x] 5.1 `codegraph impact`：`.pi/` 扩展层不在 codegraph 索引（产品 Go/前端代码索引，预期内），改机械证据替代——①-④ 调用链零改动、无新增 import、esbuild bundle 成功、冒烟全绿（详见 V.5）。验收：无 HIGH/CRITICAL 风险忽略
- [x] 5.2 §7.2 zoom-out：检查⑤不改变注入/门禁分层秩序（注入管知道：JIT 已真机验证 events.db 铁证；门禁管做到：⑤warn 不属硬门禁不进 block 判定），spec-gate 不依赖 constraint-injection、无循环依赖。验收：design D3 与实现一致，review 确认

## 7. 文档

<!-- doc-impact: standard -->
<!-- doc-impact-excuse: flow=脏工作区其他会话未提交的 docs/reference/flow 批量改动，非本 change 编辑; api=同上，docs/reference/api 批量改动非本 change; database=同上，docs/reference/database 及 backend-go dataenrichment 未提交改动非本 change -->

- [x] 6.1 `docs/reference/开发执行规范.md` §2「用例先行」表格下方补一句引用：用例设计最低集/边界/可用性 checklist 见 `standard/shared/test-design.md`（JIT 自动注入）。验收：grep 引用命中；§2 既有表格零删改
- [x] 6.2 standard 域文档更新即任务 1/3 交付物，无额外动作。验收：`bash scripts/doc-impact.sh verify openspec/changes/test-case-design-standard` 退出码 0
- [x] 6.3 （2026-08-24 归档后补记）纯工具链/文档标准 change，无 flow 影响（E 段豁免声明：本 change 仅触及 standard/shared/test-design.md · 开发执行规范.md §2 · .pi/extensions/spec-gate.ts，不涉任何业务 flow 文档；漏声明致 check-standards E 段 FAIL，归档后由 tool-output-spill 归档流程顺手补记）

## 8. 测试

> 冒烟为 pi 扩展层验证（无独立 typecheck 入口，esbuild bundle + node 断言即编译+行为验证）。

- [x] T.1 `bash .pi/extensions/tests/run-smoke.sh`（constraint-injection 冒烟含 4.1 新断言）→ 退出码 0，121 断言全绿
- [x] T.2 `bash .pi/extensions/tests/run-harness-smoke.sh` → 退出码 0（⑤断言 14 条全绿：去重/抑制/半命中/健壮性）；另对 watch-keyword-and-quickadd 真机实测命中 3 条警告（⑤b×2 纯函数SQLite 错配 + ⑤a 解析无 test-cases）——门禁在目标 change 上真实生效
- [x] T.3 `bash scripts/scenario-trace.sh openspec/changes/test-case-design-standard` → 退出码 0（唯一残留为待创建冒烟文件，归档前消失）
- [x] T.4 `bash scripts/check-standards.sh` → A-D 段零失败（115/116，唯一 FAIL 在 E 段且属遗留 change tool-output-spill 的脏工作区误报，与本 change 无关）
- [x] T.5 `bash scripts/test-assets.sh scenario-trace-gate` → 三段输出 + 退出码 0（11 行历史映射重建成功）；`bash scripts/test-assets.sh no-such-cap` → 退出码 1 + 中文提示

## 9. 验证

- [x] V.1 `grep -n '^## ' docs/reference/standard/shared/test-design.md` → 单元模型/用例文档模板/问句①②③④/验收措辞/白盒分支表/JIT 摘要全部命中（13 节含模板内嵌标题）
- [x] V.2 摘要节字节数 ≤3072（实测 2221）
- [x] V.3 `grep -n "doc-impact-applies" docs/reference/standard/shared/test-design.md` → 四信号 + section 命中（L4）
- [x] V.4 三处挂接 grep 各 1 命中；两份 testing.md diff 仅新增节零删改（+10/+10 行）
- [x] V.5 架构体检证据（5.1 机械证据版）：scanAcceptanceWording/warnAcceptanceWording 为新增导出符号（无存量调用面）；①-④ 核心判定链 diff 零触碰（grep failures.push|runScript|checkTasksMd|block 无变更行）；无新增 import；esbuild bundle 成功；两套冒烟（constraint-injection 121 断言 + spec-gate 14 断言）全绿
- [x] V.6 真机注入确认（2026-08-23 会话，编辑 openspec/changes/* 路径触发）：events.db constraint.inject 事件 `{"path":"docs/reference/standard/shared/test-design.md","mode":"section","reason":"jit-path","bytes":2381}`（摘要节本体，改文案后新版本即时生效）
- [x] V.7 试点计划：本 change 归档后，`watch-keyword-and-quickadd` apply 时按新标准补 spec 漏 Scenario（空串/纯分隔符/14 天边界）、建 test-cases.md 白盒边界矩阵、修 tasks 措辞（「SQLite 单测覆盖」→「单元测试覆盖（无 DB）」），作为规范首个完整试点；观察点：注入时机是否赶得上写作、⑤a 关键词误报率。升级注：按单元模型二次升级，试点的 test-cases.md 将是完整故事串联版（主链路表+变体走查+白盒附加），非纯白盒矩阵
- [x] V.8 问句⓪节 + 继承与调整模板节 grep 命中（L31/L39/L43）；摘要含「test-assets.sh」「继承与调整」且 2506B ≤3072
- [x] V.9 test-cases.md 交付（吃狗粮闭环）：五故事主链路表 + 变体走查 + 效果核对（移交试点）+ 白盒划除留痕 + 继承豁免（delta 全 ADDED）；实测验收：空参数变体 usage+退出码1 ✓、检查⑤对本 change ⑤a 被 test-cases.md 存在抑制 ✓、⑤b 出 4 条 meta 提及 warn（1.4/2.1/2.3/T.2 行文字面描述规则，非违例可忽略，已预写在 test-cases.md 头部）

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
| 改契约必答继承表 | 人工：问句⓪节含四列表格式，V.8 grep 命中 |
| 纯新增豁免 | 人工：问句⓪节含豁免表述，V.8 grep 命中 |
| 反向索引三段输出 | 人工：T.5 实测输出留痕 |
| 查询不存在即报错 | 人工：T.5 负路径退出码非 0 留痕 |
| 摘要含回归提醒 | 人工：V.8 摘要含 test-assets.sh 且 ≤3072 |
