# Proposal: port-constraint-injection

## Why

标准注入目前是"静态靠自觉 + 单次注入"：AGENTS.md 说"read before changing packages/"，`doc-impact.sh context` 只在 apply 启动跑一次——写到一半漂移无人管，standard 文档的 MUST 条目大部分没有机械注入牙齿。调研（[`../add-change-scope/research.md`](../add-change-scope/research.md) §四）确认 dsh 的答案是"机械门禁兜底、不指望自觉"，而用户在另一项目已实战验证的 pi extension `constraint-injection.ts`（源码见 [`docs/research/extensions/extensions/constraint-injection.ts`](../../docs/research/extensions/extensions/constraint-injection.ts)，amend-dev-workflow 迁移后路径）提供了更强的方案：harness 层每 turn 注入 system prompt，模型无法绕过。

## What Changes

- 新增 `.pi/extensions/constraint-injection.ts`：移植用户已实战的 extension（档位识别 / change 绑定 / 关键词与栈命中 / system prompt 每 turn 注入 / JIT 路径细化 / `pin_finding` 工具），适配 Syntopica 文档树与命令集；移植时新增两处局部改动——**keywordDocs 节级注入**（`section` 字段按 `## 节名` 提取，flow 只注「业务约束与不变量」节不注全文）与**关键词命中会话粘性**（与 JIT 同款只增不减，防输入滚动窗使注入块缩水 → system prompt 字节变化 → 全部历史缓存报废）。
- 新增 `.pi/constraint-injection.json` 配置：
  - 档位命令映射到本仓 openspec skills（`/opsx-*` 与 `/skill:openspec-*`，含 skill 文件路径 signal）
  - baseDocs / keywordDocs / jitDocs 映射到 `docs/reference/`（standard 按既有 `doc-impact-applies` 标签复用；flow keywordDocs 带 `section: 业务约束与不变量`；jitDocs 同支持 section，spec 化文档注 `## Requirements` 节）
  - stackSignals 适配（backend-go / front）
- 新增 `docs/reference/constraints-index.md` 超短约束索引（未激活档 indexDoc 与两档 baseDocs 共用；路径不在七域启发式内，无额外 doc-impact 声明负担）。
- **`pin_finding` 落点对齐 research-retention**：档激活 → 活跃 change 的 `explore-findings.md`；无档 → `docs/research/<topic>/`（带 topic）或 `docs/research/explore-findings.md` 通用池单文件（无 topic，沿用源实现），不碰 experience。
- **doc-impact.sh `context` 子命令退役**：注入职责移交 extension（suggest 预勾选 / verify 对账门禁保留不动）；🛑 前缀变换不移植（注入块头部已有「🔒 必须遵守」声明）。
- **context 引用面清理（17 处一次清完）**：`.pi/prompts/opsx-apply.md`（apply 模板原命令 agent 跑 context）、根 `AGENTS.md`、`docs/README.md`、`docs/reference/architecture/map.md`、`开发执行规范.md`（§0.6/分层表）、10 个 flow 文档脚注 + `flow/README.md`，数据源表述统一改指 extension（仅表述，约束内容零改动）。
- 文档：`开发执行规范.md` §0.6 步骤 1 的「跑 context」改为「extension 自动注入，无需手动跑」；§4.1 门禁分层表补"注入层"（quality-gate turn_end 门禁之前的前置层）；根 `AGENTS.md` 提及。
- smoke test：仿源项目 `tests/*.smoke.cjs` 模式，对 extension 纯函数（档位匹配/栈判定/速览提取/节提取/命中粘性/落点解析）做可脱离 pi 运行的冒烟验证。

## Capabilities

### New Capabilities

- `constraint-injection`: harness 层强制约束注入（档位 / change 绑定 / 关键词与 JIT 命中 / 每 turn system prompt 注入 / pin_finding 两级落点）

### Modified Capabilities

- `doc-impact-gate`: 「业务约束上下文获取」requirement 修改——从"apply 启动跑一次 `doc-impact.sh context`"改为"constraint-injection extension 每 turn 自动注入"（suggest/verify 不变）。
- `docs-reference-layer`: 「Business constraint ownership」的注入 scenario 修改——数据源不变（flow「业务约束与不变量」节），消费方从 context 命令改为 extension。
- `standard-spec-format`: 「Standard 文档 spec 结构」修改——`doc-impact-applies` 标签消费方从 context 改为 extension jitDocs（标签格式不变，单一真相源）；未 spec 化文档从"静默跳过"改为"全文注入"。

## Impact

- 新增：`.pi/extensions/constraint-injection.ts`（~600 行，源自实战项目改造）、`.pi/constraint-injection.json`、smoke test。
- 修改：`scripts/doc-impact.sh`（删 context 子命令）、`.pi/prompts/opsx-apply.md`（apply 模板不再命令跑 context）、`docs/reference/开发执行规范.md` §0.6/§4.1/分层表、根 `AGENTS.md`、`docs/README.md`、`docs/reference/architecture/map.md`、`docs/reference/flow/` 10 文档脚注 + README（均为数据源表述清理，约束内容零改动）。
- 依赖：**建议在 `amend-dev-workflow` 归档后 apply**（`docs/research/` 目录与 `pin_finding` research 落点依赖其落地；docs/research/extensions 路径才存在）。
- 风险：注入块与 AGENTS.md 优先级宪法打架 → 注入块头部标注"与 AGENTS.md 冲突时以 AGENTS.md 优先级宪法为准"；system prompt 膨胀 → flow 节提取（单域命中 1.7~4.6K）+ 命中粘性/JIT 只增不减（保前缀缓存）+ 未激活档仅注索引。
