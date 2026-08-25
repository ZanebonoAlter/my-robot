# case-first-testing Delta — 复杂度声明制与入口门禁

## MODIFIED Requirements

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
