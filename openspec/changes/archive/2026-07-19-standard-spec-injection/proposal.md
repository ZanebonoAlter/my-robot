# Proposal — standard-spec-injection

## Why

doc-impact context 已实现「业务规范（what）」的 apply 前置注入——改代码前自动 dump 相关 flow 的业务约束。但「执行规范（how）」——standard 代码规范——仍靠 agent 自觉读，无注入、无强制：

1. **how 注入缺失**：doc-impact context 只注入 flow 业务约束（what），standard 代码规范（怎么写、红线）不在注入范围。agent 改 airouter 代码前不会自动看到 ai-logging 的 R1-R4 红线。
2. **standard 不可精准解析**：现有 standard 文档是散文 + 表格，注入机制无法按条目提取——要注入就得灌全文（token 黑洞）或每文档写专门提取逻辑（不可持续）。
3. **硬约束无强制力**：ai-logging 的 R1（禁绕过 airouter）等红线，`scripts/` 与 `.golangci.yml` 零校验，全靠 agent 自觉。
4. **脚本 proliferation 风险**：若每条硬规则写一个 grep 脚本（airouter、model tag、响应格式……），N 规则 N 脚本，每次 grep 输出灌入上下文 = token 黑洞。

## What Changes

1. **standard spec 化**（参考 openspec spec 机制）：把 standard 文档重构成 `## Requirements` → `### Requirement: <name>`（带级别 MUST/SHOULD + 适用代码范围 + SHALL）→ `#### Scenario: WHEN/THEN` 结构。每个 Requirement 是可被注入机制解析、按需提取的结构化条目。
2. **doc-impact context 扩展（how 注入）**：context 子命令除注入 flow 业务约束（what），新增按代码改动注入相关 standard 的 `## Requirements`（how）。形成「业务规范理解任务 + 执行规范写对代码」双源注入。
3. **MUST 条目融入注入**：硬约束（如禁绕过 airouter）作为 MUST 级 Requirement，注入时 🛑 标记突出，**不单独写脚本**——避免 proliferation 与 token 黑洞。远期若需硬拦，从 spec 派生 golangci 自定义 linter（spec 驱动框架，非每规则一脚本）。
4. **试点**：先 spec 化 `ai-logging.md`（已有 R1-R4 半结构，成本最低）+ context 注入它的 Requirements，验证 how 注入闭环。

## Capabilities

### New

- `standard-spec-format`：standard 文档 SHALL 遵循 spec 结构（Requirements / Scenarios + 级别 + 适用范围），可被注入机制解析。

### Modified

- `doc-impact-gate`：context 子命令扩展——除 flow 业务约束（what），按代码改动注入相关 standard Requirements（how）。

## Impact

- 纯文档 + 脚本 change：spec 化 standard 文档 + 扩展 `doc-impact.sh context`。
- 工作流影响：apply 第 1 步 context 输出从「只 flow 业务约束」变为「flow 业务约束 + standard 执行规范」双源，agent 改代码前同时看到 what 与 how。
- **范围边界（重要）**：本 change 仅试点 spec 化 `ai-logging.md` + 扩展 context。standard/ 下其余 10 个文档（code-style / lint / package-layout / testing / theming / interaction-conventions / commit-pr）**维持散文现状**，后续 change 逐步 spec 化；check-standards 本 change 不加 standard spec 结构校验，避免一次性爆 FAIL。
- 渐进：试点 ai-logging 验证闭环后，铺开 change 再迁其余文档 + 加 check-standards 校验。
- 不引入独立 lint 脚本；token 成本靠 spec 结构化 + 按需注入控制。
