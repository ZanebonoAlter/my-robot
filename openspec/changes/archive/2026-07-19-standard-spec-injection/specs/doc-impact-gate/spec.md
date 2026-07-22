# doc-impact-gate (delta)

## MODIFIED Requirements

### Requirement: 业务约束上下文获取（apply 前置）

apply 启动时 SHALL 运行 `bash scripts/doc-impact.sh context` 获取**双源**上下文：

- **业务规范（what，理解任务）**：按 git diff 命中 domain，dump 相关 flow 文档「业务约束与不变量」节。
- **执行规范（how，写对代码）**：按 git diff 命中代码路径，匹配 standard 文档头 `doc-impact-applies` 标签，dump 命中文档的 `## Requirements` 节；MUST 级条目前缀 🛑。

本要求是**前置必读**而非归档断言——context 输出不构成归档门禁 FAIL 条件，但 §0.6 编排强制 apply 阶段执行。

#### Scenario: 命中执行规范

- **WHEN** git diff 含 `backend-go/internal/platform/airouter/router.go` 且 `ai-logging.md` 头 `doc-impact-applies` 含 `backend-go/internal/platform/airouter/`
- **THEN** context 执行规范段输出 ai-logging.md 的 `## Requirements` 节，MUST 条目带 🛑

#### Scenario: 无执行规范命中

- **WHEN** git diff 未命中任何 standard 文档的 `doc-impact-applies` 标签
- **THEN** context 执行规范段输出「未识别到相关执行规范」且退出码 0

#### Scenario: 双源分列

- **WHEN** context 同时命中业务规范与执行规范
- **THEN** 输出分「业务规范（what）」「执行规范（how）」两段，各自标头，agent 同时看到任务理解与代码红线
