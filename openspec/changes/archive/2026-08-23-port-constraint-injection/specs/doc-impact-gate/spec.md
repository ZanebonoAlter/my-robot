# doc-impact-gate Specification（delta）

## MODIFIED Requirements

### Requirement: 业务约束上下文获取（apply 前置）

约束上下文 SHALL 由 `constraint-injection` extension 在 harness 层每 turn 自动注入 system prompt（见 `constraint-injection` capability），替代原 `bash scripts/doc-impact.sh context` 一次性命令。注入保持**双源**语义：

- **业务规范（what，理解任务）**：按 change 文本/关键词命中 domain，注入相关 flow 文档**「业务约束与不变量」节**（节级提取，非全文；节尾附全文路径指引）。
- **执行规范（how，写对代码）**：按 write/edit 路径（JIT）与 change 文本命中 standard 文档头 `doc-impact-applies` 标签，注入命中文档内容（已 spec 化文档可按 `## Requirements` 节提取，未 spec 化全文注入）。

本要求是**前置必读**而非归档断言——注入内容不构成归档门禁 FAIL 条件。`doc-impact.sh` 的 `suggest`（声明预勾选）与 `verify`（归档对账）子命令职责不变。

#### Scenario: 命中执行规范

- **WHEN** implementation 档激活且 agent edit `backend-go/internal/platform/airouter/router.go`，而 `ai-logging.md` 头 `doc-impact-applies` 含 `backend-go/internal/platform/airouter/`
- **THEN** 该 standard 文档内容被 extension 会话内追加注入（只增不减），后续每 turn 均可见

#### Scenario: 命中业务规范（节级）

- **WHEN** implementation 档激活且 change 文本含 semantic-board domain 关键词（如「板块」）
- **THEN** 注入块业务规范段为 semantic-board flow 文档「业务约束与不变量」节内容，附全文路径指引，不含其余四节

#### Scenario: 无执行规范命中

- **WHEN** 改动路径与 change 文本未命中任何 standard 文档的 `doc-impact-applies` 标签
- **THEN** 注入块不含执行规范段，会话正常进行（不注入空占位）

#### Scenario: 双源分列

- **WHEN** 注入同时命中业务规范与执行规范
- **THEN** 注入块分「业务规范（what）」「执行规范（how）」两段，各自标头，agent 同时看到任务理解与代码红线

#### Scenario: context 子命令退役

- **WHEN** 开发者执行 `bash scripts/doc-impact.sh context`
- **THEN** 脚本提示该子命令已由 constraint-injection extension 取代（不静默失败），suggest/verify 行为不变
