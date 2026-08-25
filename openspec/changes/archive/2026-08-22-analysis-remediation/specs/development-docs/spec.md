# development-docs Delta

## ADDED Requirements

### Requirement: 开发执行规范 §0.6 编排含后台并行派发指引

`docs/reference/开发执行规范.md` §0.6 编排六步 SHALL 包含子代理并行派发指引：无依赖关系的任务组用后台方式（run_in_background）并行派发，主线程在等待期间进行验收准备或文档起草，统一用收口机制（get_subagent_result）验收；存在数据/产物依赖的任务（如 design 未完成时的实现派发）MUST 保持串行。

#### Scenario: 指引内容存在且可执行
- **WHEN** 读取开发执行规范 §0.6 派发章节
- **THEN** 明确写有并行派发方式、等待期主线程建议动作、收口验收机制、串行红线

#### Scenario: §12.2 溯源表无断链
- **WHEN** 校验开发执行规范 §12.2 溯源表中的归档链接
- **THEN** 所有链接指向存在的归档条目（scheduler-cron 断链已修复）
