# agent-guide-slim Delta

## ADDED Requirements

### Requirement: openspec 指令单一权威源（禁止多副本）

openspec 工作流指令 SHALL 仅存在于两个机制层：`.pi/prompts/opsx-*`（手动命令入口）与 `.agents/skills/openspec-*/SKILL.md`（skill 自动触发层）。`.claude/skills/openspec-*` 与 `.claude/commands/opsx/` 副本 MUST 删除。根 AGENTS.md MUST 注明更新约束：执行 `openspec update` 时仅带 `--tools pi`，防止其他 harness 副本再生。

#### Scenario: 仓库不存在 .claude openspec 副本
- **WHEN** 检查 `.claude/skills/` 与 `.claude/commands/` 目录
- **THEN** 不存在任何 openspec 工作流指令副本

#### Scenario: AGENTS.md 含防再生说明
- **WHEN** 读取根 AGENTS.md 的 openspec 更新约定
- **THEN** 明确写有 `--tools pi` 约束及原因（防 source-command-opsx-* 与 .claude 副本再生）
