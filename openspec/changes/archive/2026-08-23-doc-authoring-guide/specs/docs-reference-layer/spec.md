# docs-reference-layer 变更（doc-authoring-guide）

## ADDED Requirements

### Requirement: 文档编写标准

新增或修订 `docs/reference/` 下的文档时，SHALL 遵循 `docs/reference/standard/shared/doc-authoring.md`——它是目录职责、头部注释语法、flow 域文档模板、注册点 checklist 的唯一权威源。开发执行规范 §0.6 SHALL 以引用方式指向该文档（细节不进规范正文）。

doc-authoring.md SHALL 包含：

- **目录职责表**：reference 各子目录与顶层文件的定位与边界（api / architecture / database / flow / standard / 顶层散文件；research/ 为 explore 阶段自动落盘区，只说明定位、不设注册约束）
- **三种头部注释速查**：`doc-impact-applies`（flow/standard 文档头部，编辑路径 JIT 注入）、`constraint-domains`（proposal.md 头部，域声明注入）、`<!-- doc-impact: 域列表 -->`（tasks.md 文档节，归档对账）——各含语法、仓库内真实实例、写错的后果
- **flow 域文档标准模板**：五段式节名 + 互补文档引用行 + 「变更溯源」节初始态
- **新增 checklist**（新 flow 域 / 新 standard 文档两份）：逐项标注完成与否会被哪道门禁拦截（check-standards.sh 哪一段、spec-gate 哪一项）
- **自举注册**：doc-authoring.md 自身 SHALL 按其声明的标准登记（头部 `doc-impact-applies` 标签 + constraints-index.md 执行规范表登记），编辑 reference 文档时其约束节可被 JIT 注入

#### Scenario: 新增 flow 域有 checklist 可循

- **WHEN** 一个 change 需要新增 flow 域文档（如 `flow/new-feature.md`）
- **THEN** 执行者按 doc-authoring.md 的「新增 flow 域 checklist」逐项完成注册（含 constraints-index.md 域名表登记、doc-impact.sh 域菜单对应关系），无需从存量文档反推隐式规则

#### Scenario: 头部注释写错的后果可查

- **WHEN** 执行者不确定 `doc-impact-applies` 的 section 取值或「业务约束与不变量」节名的约束来源
- **THEN** doc-authoring.md 给出语法 + 真实实例 + 写错后果（如节名不符时 JIT 注入静默失效、域未登记时 constraint-injection 不识别）

#### Scenario: 归档前可知会触发哪些门禁

- **WHEN** 执行者按 checklist 新增文档后准备归档
- **THEN** checklist 中逐项标注的门禁（check-standards.sh A-G 段 / spec-gate 四项）与实际拦截行为对应，注册点缺失在归档前即可预判而非门禁炸了才发现

#### Scenario: 标准文档自举与一致性

- **WHEN** agent 编辑 `docs/reference/` 下文档且档位激活
- **THEN** doc-authoring.md 的约束节经其自身 `doc-impact-applies` 标签被 JIT 注入 system prompt
- **THEN** 文档描述的注册点语法与 constraint-injection.ts / doc-impact.sh / check-standards.sh 实际实现一致（本 change 及后续修订以 grep 一致性校验保障）
