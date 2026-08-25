# Proposal: amend-dev-workflow

## Why

两个流程缺口在 `add-change-scope` 调研中暴露：① 调研产物无落点规则——原始采纳数据、关键代码摘录不留存，新会话重新爬仓库或靠记忆推测，导致语义偏移（本次 deepseek-harness 调研差点只留结论不留数据）；② 严格 TDD（red-green-refactor）在 agent 工作流中 token 成本高——实现中途设计调整会导致先写的测试返工，而用例设计（what to test）与测试代码顺序（when to write）本是两件事。

## What Changes

- **调研落点两级规则**：
  - change 强相关调研 → `openspec/changes/<name>/research.md`（随 change 归档永久保留；须含关键代码摘录+源路径+快照日期；proposal 引用）
  - 无 change 归属调研 → `docs/research/<topic>.md`（新建目录，跨 change 复用的独立调研池）
  - `docs/experience/` 回归纯"踩坑教训/事后复盘"定位，不再放调研快照；现有 `experience/extensions-research/` 迁至 `docs/research/extensions/`
- **测试流程改为「用例先行」**（替代严格 TDD）：
  - spec.md 的 Scenario（WHEN/THEN）即黑盒行为用例，specs 阶段完成用例设计
  - 复杂逻辑（状态机≥3状态/算法/多模块交互协议）派子线程细化白盒用例（分支表/边界值清单）落 change 目录；简单 CRUD 跳过
  - 测试代码与实现顺序解绑（可先可后可同步），但测试代码 MUST 同 change 落地，归档验证节列 Scenario→测试文件映射
  - **底线不变**：bug 修复必须先写复现测试；断言判据（什么算对）主线程定，子线程只做机械枚举
- **文档更新**：`docs/reference/开发执行规范.md` §2 重写（TDD→用例先行）、§0.6 编排步骤 1 补调研落点、步骤 3 前补用例细化；§0.5 文档归属规则补 `docs/research/`；根 `AGENTS.md` 对应行同步。

## Capabilities

### New Capabilities

- `research-retention`: 调研产物两级落点（change research.md / docs/research/）与 experience 定位分离
- `case-first-testing`: 用例设计先行 + 测试代码顺序解绑的测试纪律（含两条不可豁免底线）

### Modified Capabilities

（无——`test-infrastructure` 管测试基建机制不受影响；`development-docs` 只管文档准确性，其条目不涉流程内容。）

## Impact

- 纯流程文档 change + 一次目录迁移（`experience/extensions-research` → `docs/research/extensions`），无产品代码、无数据迁移。
- 依赖：无（`port-constraint-injection` 依赖本 change 定下的文档结构，反向无依赖）。
- 风险：放宽 TDD 后测试覆盖下滑 → 用「Scenario→测试映射进归档验证节」对账兜底；子线程枚举用例质量参差 → 断言判据主线程定的红线 + 主线程核验。
