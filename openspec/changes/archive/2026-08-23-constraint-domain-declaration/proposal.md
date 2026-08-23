# 约束注入业务域显式声明

## Why

constraint-injection 在激活档位时把 change 全文（proposal/design/tasks）作为关键词命中源，配合纯子串匹配与泛关键词（`tag`/`topic`/`digest`/`模型`/`摘要`），导致与业务域无关的 change 误注入业务约束节。实测 `harness-facts-tier-a`（AI 工具链 change）文档里的 `stage`（含 "tag" 子串）、pin_finding 的 `topic` 参数、`digest` 模式、「模型零参与」等正当用词，误命中并粘性注入了 semantic-board / topic-graph / daily-report / ai-summary 四个域的业务约束节整个会话。词表重叠是本质问题：泛词收紧只能减少误伤，无法归零——harness/AI 类 change 的正当词汇必然与业务域关键词碰撞。

改为 change 显式声明涉及的业务域（声明即注入），关键词命中源收窄到对话输入窗，代码路径 JIT 兜实现期范围漂移，三层信号共同保证「注对的不多、该注的不漏」。

## What Changes

- **proposal.md 新增业务域声明标记**：头部固定格式注释（如 `<!-- constraint-domains: semantic-board, daily-report -->`，格式见 design），声明哪些域就注入哪些域的「业务约束与不变量」节；无声明则不注入 flow 约束节
- **关键词命中源收窄**：change 全文从 `matchKeywordDocs` 命中源中移除，仅保留最近 N 条对话输入窗（用户对话里主动聊到业务域仍可命中，保持现有粘性机制不变）
- **JIT 路径兜底扩展**：`.pi/constraint-injection.json` 的 `jitDocs.pathSignals` 从仅 airouter 一条扩展为 9 个业务域的代码目录映射（写/编辑某域代码路径 → 该域约束节会话内追加注入，治实现期范围漂移忘改声明）
- **泛关键词清理**：keywordDocs 中易误伤的泛词（`模型`、`摘要`、`topic`、`digest`、`tag`、`board` 等）删除或收紧为复合词；英文关键词加词边界整词匹配（修 `stage`→`tag` 类子串误伤）
- **未声明提示**：激活档位且 change 无域声明时，constraint-injection 状态栏 widget 提示「无域声明」，不阻断

## Capabilities

### New Capabilities

（无——域声明解析与注入属 constraint-injection 既有能力的行为修正）

### Modified Capabilities

- `constraint-injection`: 「每 turn system prompt 强制注入」Requirement 的命中源规则变更——change 文本不再参与关键词命中；新增业务域声明解析（proposal.md 固定标记 → 声明域约束节注入）与声明缺失提示；JIT 路径命中扩展为全业务域覆盖

## Impact

- `.pi/extensions/constraint-injection.ts`：`planInjection` 命中源逻辑、proposal 声明解析函数、`matchKeywordDocs` 词边界
- `.pi/constraint-injection.json`：`keywordDocs` 泛词清理、`jitDocs` 补 9 业务域 pathSignals
- `.pi/extensions/tests/`：constraint-injection 纯函数烟测（声明解析、词边界匹配、无声明回落）
- `docs/reference/开发执行规范.md` §0.6 / §11：propose 产物清单与归档对账补「业务域声明」一项
- `openspec/specs/constraint-injection/spec.md`：delta 同步主 spec
- **部署后影响**：既有活跃 change（harness-facts-tier-a 等）proposal 无声明标记 → 注入行为从「误注入 4 域」变为「不注入 flow 约束节（有 widget 提示）」，属预期收敛；新 change 按 propose 流程写声明后行为不变
