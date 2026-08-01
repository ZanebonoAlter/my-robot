# docs-reference-layer (delta)

## ADDED Requirements

### Requirement: Flow directory positioning

`docs/reference/flow/` SHALL 定位为「需求说明 + 链路设计 + 业务约束 + 代码索引 + 变更溯源」五位一体的大功能活文档，并承接原 user-guide 的"系统能做什么"说明职责。

每个 flow 文档 SHALL 遵循五段式固定结构：

1. `## 需求说明` — 功能解决什么问题（面向使用视角）
2. `## 链路设计` — mermaid 流程图 + 状态流转
3. `## 业务约束与不变量` — 状态机/幂等/去重/限额等业务红线
4. `## 代码入口` — 后端 handler/service + 前端 feature 入口
5. `## 变更溯源` — archive change 链接表（见《开发执行规范》§12.2）

#### Scenario: Flow 文档五段式结构

- **WHEN** check-standards.sh A 段校验任一 `docs/reference/flow/*.md`（README 除外）
- **THEN** 该文档存在「需求说明」「链路设计」「业务约束与不变量」「代码入口」「变更溯源」五个二级标题

### Requirement: Business constraint ownership

业务约束 SHALL 按类型归属固定位置，不散落多处：

| 约束类型 | 归属 |
| --- | --- |
| 业务不变量（状态机、去重、限额） | flow 文档「业务约束与不变量」节 |
| 跨功能传导耦合（改 A 影响 B） | `architecture/coupling-map.md` |
| 代码写法约束 | `standard/` |
| 数据/测试安全红线 | `standard/backend/testing.md` |

#### Scenario: 新增业务不变量

- **WHEN** 一个 change 引入新的业务不变量（如状态机新状态约束）
- **THEN** 该约束记录在受影响 flow 文档的「业务约束与不变量」节，而非仅存在代码注释中

#### Scenario: apply 时作为约束上下文注入

- **WHEN** 一个 change 的 apply 阶段跑 `doc-impact.sh context` 且其代码改动命中某 flow 的 domain
- **THEN** 该 flow 文档「业务约束与不变量」节内容被 dump 为 apply 上下文（见 doc-impact-gate capability），约束从「被动记录」转为「主动注入」
