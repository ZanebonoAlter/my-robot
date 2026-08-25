# case-first-testing Specification（delta）

## ADDED Requirements

### Requirement: 用例设计先行

实现代码动手前，change 的 specs SHALL 已完成用例设计：spec.md 的 Scenario（WHEN/THEN）即黑盒行为用例。满足以下任一条件时，SHALL 额外产出白盒用例文档（分支表/边界值清单）存放于 change 目录：

- 涉及状态机（≥3 状态循环流转）
- 涉及算法逻辑
- 涉及多模块交互协议（API 契约/事件总线）

白盒用例枚举 MAY 派发子线程执行，但**断言判据（什么算对）MUST 由主线程确定**；子线程仅做机械枚举。

#### Scenario: 简单 CRUD 直接进实现

- **WHEN** change 只涉及简单 CRUD，无状态机/算法/协议
- **THEN** spec.md 的 Scenario 即全部用例设计，无需白盒用例文档，测试代码与实现顺序自由

#### Scenario: 复杂逻辑先枚举白盒用例

- **WHEN** change 涉及 3 状态循环状态机
- **THEN** change 目录存在白盒用例文档（含分支表与边界值清单），且其断言判据由主线程给出

### Requirement: 测试代码顺序解绑与归档对账

测试代码与实现代码的编写顺序 SHALL 解绑（可先可后可交织），不再强制"先看到测试失败再写实现"。约束为：

- 测试代码 MUST 与实现同 change 落地，不得延后到归档后补。
- tasks.md 验证节 SHALL 列出 Scenario→测试文件的映射表；归档时未映射的 Scenario 视为未覆盖。
- **bug 修复不受解绑豁免**：MUST 先写复现测试（先复现才能证明修复的是该 bug）。

#### Scenario: 顺序解绑

- **WHEN** 子线程实现一个新功能并同 PR 写出覆盖全部 Scenario 的测试
- **THEN** 流程合规，无需证明"测试先于实现"或"曾看到失败"

#### Scenario: bug 修复先复现

- **WHEN** change 修复一个 bug
- **THEN** 存在先于修复代码完成的复现测试，且该测试曾失败（或以其他方式证明复现了目标 bug）

#### Scenario: 归档对账

- **WHEN** change 进入归档门禁
- **THEN** tasks.md 验证节存在 Scenario→测试文件映射，spec 中每个 Scenario 均有对应测试文件
