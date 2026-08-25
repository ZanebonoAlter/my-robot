## MODIFIED Requirements

### Requirement: 测试代码顺序解绑与归档对账

测试代码与实现代码的编写顺序 SHALL 解绑（可先可后可交织），不再强制"先看到测试失败再写实现"。约束为：

- 测试代码 MUST 与实现同 change 落地，不得延后到归档后补。
- tasks.md 验证节 SHALL 列出 Scenario→测试文件的映射表（机器可读格式见 `scenario-trace-gate` capability）；归档时未映射的 Scenario 视为未覆盖——该对账由 spec-gate 检查④（scenario-trace.sh）机器强制，无自动化测试的 Scenario SHALL 显式映射为「人工」留痕。
- **bug 修复不受解绑豁免**：MUST 先写复现测试（先复现才能证明修复的是该 bug）。

#### Scenario: 顺序解绑

- **WHEN** 子线程实现一个新功能并同 PR 写出覆盖全部 Scenario 的测试
- **THEN** 流程合规，无需证明"测试先于实现"或"曾看到失败"

#### Scenario: bug 修复先复现

- **WHEN** change 修复一个 bug
- **THEN** 存在先于修复代码完成的复现测试，且该测试曾失败（或以其他方式证明复现了目标 bug）

#### Scenario: 归档对账

- **WHEN** change 进入归档门禁
- **THEN** scenario-trace.sh 对 delta specs 的每个待对账 Scenario 校验 tasks.md 验证节的映射表（机器可读格式见 `scenario-trace-gate` capability），未映射或映射文件不存在则归档被阻断

#### Scenario: 人工验证映射留痕

- **WHEN** 某 Scenario 无自动化测试（如纯 UI 目视确认）
- **THEN** 映射表该行的测试文件单元格以「人工」开头说明验证方式，对账通过且留痕
