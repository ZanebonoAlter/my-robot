# Design — standard-spec-injection

## 1. 两类规范区分（注入语义）

| | 业务规范 | 执行规范 |
| --- | --- | --- |
| 回答 | 做什么（what） | 怎么写（how） |
| 载体 | flow「业务约束与不变量」 | standard「## Requirements」 |
| 注入目的 | 理解任务 | 写对代码 |

doc-impact context 现只注入 what（左列）；本 change 补 how（右列），同一注入框架双源。

## 2. standard spec 化结构（参考 openspec spec）

每个 standard 文档（如 `ai-logging.md`）SHALL 用如下结构替换散文：

```
# <规范名>

<!--
doc-impact-applies: backend-go/internal/platform/airouter/, backend-go/internal/dataenrichment/
-->

## Requirements

### Requirement: 所有 AI 调用必须经 airouter
**级别**: MUST
业务代码 SHALL 通过 `airouter.Router.Chat/Embed` 调用 LLM，禁止直连 `/chat/completions`。

#### Scenario: 新增 LLM 调用
- **WHEN** 业务代码需要调用 LLM
- **THEN** 走 `airouter.Router.Chat(req)` / `Router.Embed(req)`
- **AND NOT** 直接 `http.Post("/chat/completions")`

### Requirement: 必须记录调用字段
**级别**: MUST
...
```

要素：

- 文档头 HTML 注释 `doc-impact-applies: <代码路径模式>` —— 文档级适用范围（逗号分隔多 token，路径前缀匹配，详见 §4）
- `## Requirements` —— 注入机制只提取此节
- `### Requirement: <name>` —— 每条规范一个条目
- `**级别**: MUST/SHOULD` —— MUST 注入时 🛑 突出
- `#### Scenario: WHEN/THEN` —— 可执行的行为约定（Anti-Patterns 并入 `AND NOT` 句式）

## 3. context 注入扩展（how 维度）

`doc-impact.sh context` 现逻辑：git diff → domain → flow「代码入口」节匹配 → dump flow「业务约束」节。

扩展为双源：

1. **业务规范（what）**：原逻辑不变，dump flow 业务约束。
2. **执行规范（how）**：遍历 `docs/reference/standard/**/*.md`，凡文档头 `doc-impact-applies` 命中本次改动的代码路径 → dump 其 `## Requirements` 节；MUST 条目前加 🛑。

输出分两段，各自标头：

```
──── 业务规范（理解任务：what）────
<flow 业务约束>

──── 执行规范（写代码：how）────
🛑 [MUST] 所有 AI 调用必须经 airouter
   WHEN ... THEN ...
[SHOULD] ...
```

## 4. 关联机制（代码 → standard）

文档级 `doc-impact-applies` 标签。改了 `backend-go/internal/platform/airouter/router.go` → 匹配 `ai-logging.md` 的 `applies: backend-go/internal/platform/airouter/`。

**匹配语义**：逗号分隔多 token；每个 token 做**路径前缀匹配**——以 `/` 结尾视作目录前缀（匹配其下所有文件），否则整路径前缀。token 须用**仓库根相对路径**（与 `git diff --name-only` 输出一致，如 `backend-go/internal/...`）。**本 change 不支持 glob 通配符**（`*`/`?`），起步够用，远期再加。

起步用文档级（简单，对称 flow 的 domain 关联）。条目级精准标签（每个 Requirement 独立 applies）为远期，非本 change。

### 标签对照（勿混）

| 标签 | 出现位置 | 语义（方向） | 消费者 |
| --- | --- | --- | --- |
| `<!-- doc-impact: ... -->` | change 的 tasks.md | **change → 文档域**：声明本次改动影响哪些文档域（menu 8 选项） | `doc-impact.sh verify` 对账 |
| `<!-- doc-impact-applies: ... -->` | standard 文档头 | **文档 → 代码**：声明本规范适用哪些代码路径 | `doc-impact.sh context` how 注入 |

两者名字相近、方向相反，agent 编写/阅读时须按上表区分。

## 5. 粒度与 token 控制

- 只注入命中文档的 `## Requirements` 节，不灌 Anti-Patterns 散文 / 已接入清单 / 资料来源。
- spec 化时控制每个 Requirement 精炼（几行 + Scenario），避免长篇。
- 无命中则 how 段输出「未识别到相关执行规范」。

## 6. MUST 处理（不搞独立脚本）

MUST 条目（如禁绕过 airouter）注入时 🛑 突出，agent 看到即知是硬红线。**不写独立 grep 脚本**（避免 N 规则 N 脚本 token 黑洞）。

远期若需归档门禁硬拦，从 spec 的 MUST 条目派生 golangci 自定义 linter（一次性框架，读 spec 生成检查），非每规则一脚本。本 change 不做。

## 7. 试点：ai-logging.md

`ai-logging.md` 已有 R1-R4 + Anti-Patterns 半结构，spec 化成本最低：

- R1-R4 → 4 个 Requirement（级别 MUST）+ Scenario
- Anti-Patterns 按条目归宿拆解：
  - 「绕过 airouter 直连」→ R1 的 `AND NOT`
  - 「AICallLog 不存 prompt」「operation 留空/无意义值」→ R2 的 `AND NOT`
  - 「agent loop 不传 session_id」→ R3 的 `AND NOT`
  - 「新增 AI 功能不更新已接入清单」→ 不属 R1-R4，独立 `### Requirement: 规范清单维护`（级别 SHOULD）
- 文档头加 `doc-impact-applies: backend-go/internal/platform/airouter/, backend-go/internal/dataenrichment/`（LLM 调用方）
- context 扩展后验证：改 airouter 代码 → context 输出含 ai-logging Requirements

## 8. check-standards 校验（本 change 不做）

spec 化铺开后可加 check-standards 校验「每个 standard 文档有 `## Requirements` 节」（类似 flow 五段式）。本 change 先靠试点验证，铺开 change 再加校验。

## 9. 过渡期行为（非 spec 化文档）

本 change 仅 spec 化 ai-logging.md，standard/ 下其余 10 个文档（code-style / lint / package-layout / testing / theming / interaction-conventions / commit-pr …）暂维持散文现状。context how 注入遇到**无 `## Requirements` 节**的 standard 文档时**静默跳过**（不报错、不灌散文），待后续 change 逐步 spec 化。配合 §8，check-standards 本 change 也不校验 standard spec 结构，避免一次性爆 FAIL。
