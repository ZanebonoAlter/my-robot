# doc-authoring-guide 设计

## Context

见 proposal.md（Why：注册点散落 6 处隐式规则，「已有可照抄、新增无标准」）。本 change 为纯文档 change：一个新标准文档 + 两处登记/引用，无代码改动，无 harness 行为变更。

## Goals / Non-Goals

**Goals**

- 把「新增/修订 reference 文档」的全部注册点一次性清单化，逐项挂门禁标注
- doc-authoring.md 自举：按自己声明的标准完成注册，成为 standard/shared/ 的新成员后立即生效

**Non-Goals**

- 不改 harness 实现（constraint-injection.ts / check-standards.sh / spec-gate.ts 一行不动）
- 不重写存量文档（只加引用/登记行）
- 不做 D 级调研项「文档预算治理」（AGENTS.md 字数上限）——那是独立 change

## Decisions

### D1 双层结构：细节在 doc-authoring.md，执行规范只留引用行

- 开发执行规范.md 已 ~340 行，§0.6 编排表附近加一行引用；目录职责表、注释语法、模板、checklist 全部只在 doc-authoring.md
- 备选「全部塞进执行规范 §0.6」否决：规范管流程（何时做什么），标准管形态（写成什么样），混排会让两边都难维护，也违背 harness-survey D13（文档预算治理）的方向

### D2 内容以实测为准，不凭记忆写

- 三种注释的语法、check-standards.sh A-G 段语义、spec-gate 四项、doc-impact.sh 域菜单，实现期**逐一从源文件提取**（.pi/extensions/constraint-injection.ts、scripts/check-standards.sh、scripts/doc-impact.sh、.pi/extensions/spec-gate.ts），文档中引用「写错的后果」必须对应实现里的真实行为（如节名抓取逻辑）
- 这是本 change 最大的质量风险点：标准文档自己写错比没有标准更糟

### D3 自举机制的可行性先行查证

- standard/ 文档头部 `doc-impact-applies` 标签若无先例，实现期先查 constraint-injection.ts 对 standard/ 路径的 JIT 处理与 section 抓取规则，再决定 doc-authoring.md 的标签写法；若 standard 文档不适用该标签，则退化为「登记进 constraints-index 表（常驻索引可见）」，自举 Scenario 相应弱化——以实现为准，不硬凑

### D4 research 目录只写定位，不设 checklist

- 用户拍板：research/ 是 explore 阶段（pin_finding 无档语境）自动落盘区，无注册点、无门禁校验；目录职责表中一行说明即可

## Risks / Trade-offs

- [标准文档与 harness 实现漂移（后续门禁加段/注释语法变更）] → tasks 验证节含 grep 一致性校验（本 change）；后续改 harness 注册机制的 change 应在 doc-impact 声明中纳入 doc-authoring.md（checklist 中显式提醒）
- [checklist 项与门禁段落错标] → D2 实测提取 + 验证节逐条对账
- [文档过长变新负担] → 目标 ≤200 行；速查表用「语法一行 + 实例一处 + 后果一行」紧凑格式

## Migration Plan

无迁移。落地即生效：新文档进 standard/shared/（check-standards D 段防孤立引用要求 constraints-index 登记同步完成）。

## Open Questions

（无——D3 的自举可行性属实现期查证项，不影响 specs 与任务拆解。）
