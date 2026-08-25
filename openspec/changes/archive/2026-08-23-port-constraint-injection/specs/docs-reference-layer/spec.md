# docs-reference-layer Specification（delta）

## MODIFIED Requirements

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

- **WHEN** constraint-injection extension 档位激活且 change 文本/关键词命中某 flow 的 domain
- **THEN** 该 flow 文档「业务约束与不变量」节内容被注入 system prompt（见 doc-impact-gate capability），约束从「被动记录」转为「主动注入」
