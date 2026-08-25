# case-first-testing Specification

## Purpose
TBD - created by archiving change amend-dev-workflow. Update Purpose after archive.

## Requirements

### Requirement: 用例设计先行

实现代码动手前，change 的 specs SHALL 已完成用例设计：spec.md 的 Scenario（WHEN/THEN）即黑盒行为用例。满足以下任一条件时，SHALL 额外产出白盒用例文档（分支表/边界值清单）存放于 change 目录：

- 涉及状态机（≥3 状态循环流转）
- 涉及算法逻辑
- 涉及多模块交互协议（API 契约/事件总线）

白盒用例枚举 MAY 派发子线程执行，但**断言判据（什么算对）MUST 由主线程确定**；子线程仅做机械枚举。

**复杂度声明**：新 change 的 proposal.md 头部 SHALL 携带机器可读复杂度声明 `<!-- complexity: complex | simple -->`（与 `constraint-domains` 同款 HTML 注释惯例）。复杂度是设计阶段判断（propose/design 时点），不是实现事后归类；声明 complex 即自认负有白盒用例文档义务。

**入口反馈**：会话档位切入 implementation 且绑定某 change 时，若该 change 目录缺失白盒用例文档（`test-cases*.md` 前缀或 `*-test-cases.md` 后缀），harness SHALL 在动工首回合内向 agent 上下文注入一条 steer 级提醒（非阻断）：

- 声明 `complex` → 提醒直接指出「声明复杂档但缺白盒用例文档」，修复路径为补文档或改声明；
- 声明 `simple` 或未声明 → 以固定关键词表（算法/状态机/解析/协议）扫 tasks.md 任务行作兜底信号，命中 → 提醒补声明或补文档；未命中 → 静默。

同一会话内对同一 change 的该提醒 SHALL 至多注入一次；文档补齐后后续回合 SHALL 静默。词法扫描是兜底信号而非判定——关键词表 SHALL NOT 为提高召回而扩容（高频词会稀释警报价值）。

#### Scenario: 简单 CRUD 直接进实现

- **WHEN** change 只涉及简单 CRUD，无状态机/算法/协议
- **THEN** spec.md 的 Scenario 即全部用例设计，无需白盒用例文档，测试代码与实现顺序自由

#### Scenario: 复杂逻辑先枚举白盒用例

- **WHEN** change 涉及 3 状态循环状态机
- **THEN** change 目录存在白盒用例文档（含分支表与边界值清单），且其断言判据由主线程给出

#### Scenario: 声明 complex 且缺文档，动工时被提醒

- **WHEN** 档位切入 implementation 绑定 change X，X 的 proposal 声明 `<!-- complexity: complex -->` 且目录无 test-cases*.md
- **THEN** 动工首回合结束前注入一条 steer 提醒，文案含「复杂档」「白盒用例文档」与修复路径（补文档或改声明），不阻断工具调用

#### Scenario: 已有文档，动工全程静默

- **WHEN** 档位切入 implementation 绑定 change X，X 目录已存在 test-cases.md
- **THEN** 不注入任何入口提醒

#### Scenario: 声明 simple 且词表未命中，静默放行

- **WHEN** change X 声明 `<!-- complexity: simple -->`，tasks.md 任务行不含算法/状态机/解析/协议关键词
- **THEN** 不注入入口提醒

#### Scenario: 声明 simple 但任务行命中关键词，兜底质询

- **WHEN** change X 声明 `<!-- complexity: simple -->` 且目录无白盒用例文档，tasks.md 某任务行含「状态机」
- **THEN** 注入一条 steer 提醒，指出声明与任务措辞矛盾，修复路径为改声明或补文档

#### Scenario: 提醒每会话每 change 至多一次

- **WHEN** change X 已在首回合收到入口提醒，同会话后续回合 X 仍缺文档
- **THEN** 不再重复注入；若 X 补齐文档则立即恢复静默

#### Scenario: requirements 档零触发

- **WHEN** 会话处于 requirements 档（无论是否绑定 change）
- **THEN** 入口门禁不执行任何检查与提醒

#### Scenario: 归档对账时声明优先复核

- **WHEN** openspec archive 命中归档门禁，change X 声明 complex 且仍缺白盒用例文档
- **THEN** 归档措辞扫描产出「声明复杂档缺文档」的强违例提示；声明 simple 且词表命中的产出反向质询提示；两者均为 warn 级不阻断归档（四项 block 检查不变）

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
