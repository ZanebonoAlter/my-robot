# standard-spec-format (delta)

## ADDED Requirements

### Requirement: Standard 文档 spec 结构

每个 `docs/reference/standard/**/*.md` SHALL 包含 `## Requirements` 节，其下以 `### Requirement: <name>` 组织规范条目。每个 Requirement SHALL 标注 `**级别**: MUST` 或 `SHOULD`，并附至少一个 `#### Scenario: WHEN/THEN`。

文档头 SHALL 含 HTML 注释 `doc-impact-applies: <代码路径模式>` 声明适用范围，供 `doc-impact.sh context` 关联代码改动。

规范散文（Anti-Patterns、注意事项）SHALL 并入对应 Requirement 的 Scenario（`AND NOT` 句式）或作为 Requirement 正文，而非独立成节，保证注入机制提取 `## Requirements` 节即得全部规范条目。

#### Scenario: spec 结构齐全

- **WHEN** 校验任一 standard 文档
- **THEN** 存在 `## Requirements` 节，其下 Requirement 条目含级别 + 至少一个 Scenario

#### Scenario: 适用范围声明

- **WHEN** doc-impact context 按代码改动匹配 standard
- **THEN** 文档头 `doc-impact-applies` 命中改动路径时，其 `## Requirements` 节被注入

#### Scenario: MUST 级别标注

- **WHEN** 一条规范是硬约束（违反即不合规）
- **THEN** 其 Requirement 标 `**级别**: MUST`，注入时前缀 🛑 突出
