## Why

「新增/修订 `docs/reference/` 文档」的注册点散落在 6 处隐式规则里：flow 五位一体结构（flow/README.md）、`doc-impact-applies` 头部标签语法（constraint-injection.ts 实现 + 各文档头部实例）、新 flow 域须登记 constraints-index.md 域名表、「业务约束与不变量」节名固定（注入按节名抓取）、§12.2 变更溯源表格式（E 段校验）、domain 白名单/双主题/防孤立引用（check-standards.sh A-D 段硬校验）。存量文档靠互相抄存活，元知识从未写下来——结果是「已有的可以（照抄），新增的不行」：写错节名注入静默失效、忘登记域 constraint-injection 不认、归档时 check-standards 炸了才发现。需要一份标准文档把「每个文件夹放什么、怎么写、哪道门禁拦你」清单化。

## What Changes

- **新增 `docs/reference/standard/shared/doc-authoring.md`**（reference 文档编写标准，细节唯一权威源）：
  - 目录职责表：api / architecture / database / flow / standard / 顶层散文件各放什么不放什么，何时进 reference 何时留 research（research 目录为 explore 阶段自动落盘，无硬约束，仅说明定位不设 checklist）
  - 三种头部注释速查：`doc-impact-applies`（flow/standard 文档头部，JIT 命中）/ `constraint-domains`（proposal.md 头部，域声明注入）/ `<!-- doc-impact: 域列表 -->`（tasks.md 文档节，归档对账）——各含语法、真实实例、**写错的后果**（如节名不叫「业务约束与不变量」→ 注入静默失效）
  - flow 域文档标准模板：五位一体节名约定、互补文档引用行、「变更溯源」节初始态
  - 新增 checklist ×2（新 flow 域 / 新 standard 文档）：逐项标注哪道门禁会拦（check-standards A-G 哪段、spec-gate 哪项）——把「归档时炸了才知道」变成「动手前就知道」
  - **自举闭环**：自身头部带 `doc-impact-applies` 标签、登记进 constraints-index 执行规范表，编辑 reference 文档时自身被 JIT 注入
- **`docs/reference/开发执行规范.md`**：§0.6 编排表附近加一行引用（「新增/修订 reference 文档见 standard/shared/doc-authoring.md」），细节不进正文（防规范文档膨胀）
- **`docs/reference/constraints-index.md`**：执行规范表登记 doc-authoring.md

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `docs-reference-layer`: 新增「文档编写标准」requirement——新增/修订 reference 文档 SHALL 遵循 doc-authoring.md（目录职责、头部注释语法、flow 模板、注册点 checklist）；既有「Reference directory structure」requirement 不变

## Impact

- `docs/reference/standard/shared/doc-authoring.md`（新建，主产物）
- `docs/reference/开发执行规范.md`（+1 行引用）
- `docs/reference/constraints-index.md`（+1 行登记）
- 纯文档 change：豁免代码测试，验证走 grep 一致性校验——**校验标准文档描述的注册点语法与 constraint-injection.ts / check-standards.sh / doc-impact.sh 实际实现一致**（防标准文档自己写错）；check-standards 自身按其 D 段（防孤立引用）对新文档生效
- 无 constraint-domains 声明（不涉业务域，纯工具链/文档 change）
