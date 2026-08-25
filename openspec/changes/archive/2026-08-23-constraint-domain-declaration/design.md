# Design — 约束注入业务域显式声明

## Context

constraint-injection 的关键词命中机制在激活档位时吃「change 全文 + 最近 5 条输入」，纯子串匹配。实测 `harness-facts-tier-a` change 文档的正当用词（`stage` 含 "tag" 子串、pin_finding 的 `topic` 参数、`digest` 摘要模式、「模型零参与」）误命中 4 个业务域约束节并粘性注入整会话。根因：harness/AI 类 change 的正当词汇与业务域关键词本质重叠，词表修补无法归零。

主 spec 已有两条既有约定影响本设计：

1. 「文档命中 SHALL 复用 standard 文档既有 `doc-impact-applies` 标签生成 JIT pathSignals（单一真相源，不发明第二套映射）」——现状只落地 `ai-logging.md` 一条标签，`.pi/constraint-injection.json` 的 `jitDocs` 手写重复了同一路径（violates 单一真相源）。
2. flow 文档均含「代码入口」散文节（不可机器读），域名即文档 basename（semantic-board / topic-graph / daily-report / content-enrichment / data-enrichment / ai-summary / discovery / reading / scheduler）。

## Goals / Non-Goals

**Goals:**

- 误伤归零（构造性）：change 文本退出关键词命中源，域注入由 proposal 显式声明驱动
- 漏报有兜：JIT 路径命中扩展到全部业务域代码目录，治实现期范围漂移
- 单一真相源：域→代码路径映射收口到 `doc-impact-applies` frontmatter 标签（flow 文档补标签），json 手写 jitDocs 退役
- 存量兼容：无声明的存量 change 明确收敛为「不注入 flow 约束节 + widget 提示」，不静默、不阻断

**Non-Goals:**

- 不做声明的自动预填（`openspec new` 模板属 CLI 外部工具；doc-impact.sh suggest 扩展列为开放问题）
- 不改档位识别 / change 绑定 / pin_finding 落点逻辑（与本问题正交）
- 不动栈判定（`detectStacks` 吃 changeText 保留——栈信号是路径类词，误伤无害且当前配置无 conditionalDocs 生效）
- 不加「归档时校验声明覆盖度」的对账（属 doc-impact.sh verify 扩展，另行 change）

## Decisions

### D1 声明标记：proposal.md 头部 HTML 注释，域名单 = flow 文档 basename

格式：`<!-- constraint-domains: daily-report, topic-graph -->`，置于 proposal.md 标题行附近，逗号分隔，可多行（解析时合并）。

- **为什么是注释而非固定小节/frontmatter**：与 doc-impact 门禁的 `<!-- doc-impact: ... -->` 注释风格统一（都是机器读元数据，不占正文结构，spec-gate/verify 好解析）；proposal.md 无既有 frontmatter 约定，不发明。
- **域名合法值** = `docs/reference/flow/*.md` 的 basename（9 个）。未知域名**宽容忽略** + widget 提示（fail-safe：拼错域名不该炸注入）。
- **无声明 = 合法状态**：纯 harness/工具链 change 本就不涉及业务域，不注入 flow 约束节；widget 显示「无域声明」提示，不阻断。

### D2 命中源三层分工：声明为主，JIT 兜漂移，输入窗管对话

| 层 | 命中源 | 注入时机 | 粘性 |
| --- | --- | --- | --- |
| 声明（主） | proposal.md `constraint-domains` 标记 | 每回合重解析（proposal 编辑立即生效） | 否（声明删域即撤） |
| JIT（兜） | write/edit 路径 × `doc-impact-applies` 标签 | tool_execution_start 命中即入列 | 是（会话内只增不减） |
| 输入窗（对话） | 最近 N 条用户输入 × keywordDocs 强词 | input 事件命中即入列 | 是（现状不变） |

- **change 文本退出 `matchKeywordDocs` 命中源**（`matchText` 仅剩 `recentInputs.join()`）——这是消除误伤的关键一刀。change 文本保留给 `detectStacks`（栈信号为路径类词，无误伤面）。
- 声明注入**不进粘性集合**：粘性的动机是保前缀缓存（词条滚出窗不让注入块缩水），而 proposal 声明是持久文件不存在滚出问题；声明是每回合从文件重解析，编辑 proposal 增删域立即生效，方向上只有「尾部追加」无「中段消失」，前缀缓存天然安全。
- 注入顺序：baseDocs → 声明域 → keyword 粘性 → JIT 粘性（声明域紧跟基础文档，语义上最相关）。

### D3 JIT 映射单一真相源：`doc-impact-applies` 标签扩展到 flow 文档，json jitDocs 退役

- 9 个 flow 文档头部补 frontmatter 标签（如 daily-report.md：`doc-impact-applies: backend-go/internal/topicgraph/, backend-go/internal/admin/scheduler/`），路径取自各文档「代码入口」节的实际目录（flow 文档与映射同文件，改代码入口时顺手同步）。
- extension 首次 JIT 检查时扫描 flow + standard 文档头部标签构建映射（mtime 缓存，与 configCache 同模式），`.pi/constraint-injection.json` 删除 `jitDocs` 手写条目（airouter 条目已由 ai-logging.md 标签覆盖，无信息丢失）。
- **为什么不留在 json**：json 一套、flow「代码入口」节一套，正是「第二套映射」；spec 的 SHALL 已指明方向，本次补齐落地。
- 扫描范围：`docs/reference/flow/*.md` + `docs/reference/standard/**/*.md` 头部（前 15 行内匹配 `^doc-impact-applies:`），约 20 个文件，读头部 + mtime 缓存开销可忽略。

### D4 泛词清理原则：输入窗只留「人类语言里指向业务域的词」

- **删除代码标识符类英文词**：`tag`/`Tag`/`tags`/`board`/`Board`/`tagmanagement`/`topic`/`Topic`/`topicgraph`/`digest`/`Digest`/`morning`/`AI`/`ai-`/`llm`/`LLM`/`模型`/`摘要`——这些词在代码/工具链语境高频出现（正是误伤源），且聊到该域的人会用中文说「标签」「看板」「话题」「日报」「提示词」。
- **保留 CJK 日常词**：版块/板块/看板/标签、话题/图谱/关系图、日报/每日报告/每日推送、抓取/爬虫/正文/回填、数据增强/认知闭环/增强项、偏好/画像/订阅源/RSS、阅读页/稍后读、定时/调度/定时任务。CJK 词在 harness 类对话中不会自然出现，子串匹配足够安全。
- **英文词保留**（强领域、无歧义）：`firecrawl`、`airouter`、`content_completion`、`dataenrichment`、`discovery`、`scheduler`、`cron`、`job`——等等，`scheduler`/`cron`/`job`/`discovery` 在 harness 对话（聊 extension/任务队列）也可能出现；但这些词用户主动打进输入窗时通常确在聊调度/发现。**原则裁决：凡在 harness-facts-tier-a 事故中实际撞车的词一律删，其余保留**——`scheduler`/`cron`/`job` 未撞车（该 change 文档未含），且整词边界已加，误伤面大幅缩小。
- **ASCII 关键词加词边界**：`matchKeywordDocs` 对纯 ASCII 关键词用 `new RegExp(\`\\b${escaped}\\b\`)` 整词匹配（修 `stage`→`tag` 类子串误伤）；CJK 关键词保持 `includes`（`\b` 不识别 CJK 边界）。已删词的防线 + 词边界 = 双保险。

### D5 声明缺失与存量 change：明确收敛 + widget 提示

- 激活档位且绑定的 change proposal 无标记/空标记 → 不注入任何 flow 约束节，widget 追加「⚠ 无域声明（纯工具链 change 可忽略）」。
- 存量活跃 change（harness-facts-tier-a / fix-quality-audit-p0 / watch-keyword-and-quickadd）行为变化：误注入消失（预期收敛）；确实涉及业务域的（watch-keyword-and-quickadd 涉 topic-watch）需补声明——实现任务里逐个补。

### D6 事实库记账扩展（harness-facts-tier-a 衔接）

- `InjectedDocEntry.reason` 枚举新增 `"declaration"`；声明注入的文档按此记账，账本可区分「声明驱动 vs 关键词/JIT 驱动」，为后续验证声明覆盖率留数据。

## Risks / Trade-offs

- [漏声明 + 改动路径不在任何 `doc-impact-applies` 标签内 → 约束静默缺席] → 归档门禁 doc-impact verify（flow 域启发式）兜底对账；本仓 change 均走 §0.6 流程，评审时声明可见。极端情况下约束缺席的后果是 lint/测试门禁仍拦截大部分违规（quality-gate 独立于约束注入）。
- [flow 文档标签漏配或路径写错 → JIT 兜底失效] → 烟测断言 9 个 flow 文档标签存在且路径在仓库真实存在；标签与「代码入口」节同文件降低漂移概率。
- [声明域文档不存在该节（如 flow 文档重构删节）] → 既有 fail-safe：节不存在回落全文注入（现状行为，不改）。
- [词边界正则 × 输入窗 5 条 × 关键词 ~40 个，每回合开销] → 微基准可忽略（<1ms 量级）；不改命中粘性机制，缓存安全不受影响。
- [行为变化：依赖关键词命中的既有工作流（用户不写声明、靠聊关键词触发注入）] → 输入窗命中保留且词表更精准；harness 类 change 的误注入正是要消除的行为。权衡已显式接受。

## Migration Plan

1. extension 改造 + json 词表清理 + 9 个 flow 文档补标签（一个 change 内原子落地）
2. 存量活跃 change 补声明（watch-keyword-and-quickadd 补 `constraint-domains: topic-graph`；另两个纯 harness change 不补，靠 widget 提示自然引导）
3. 回滚：单 commit revert 即恢复原命中行为，无数据迁移

## Open Questions

- doc-impact.sh `suggest` 是否扩展「按 git diff 预填 constraint-domains 声明」（复用 D3 标签做反向映射）？——倾向做但非本 change 必须，量级小可顺手；tasks 里列为可选任务。
- 未激活档（research 语境）是否也提示补声明？——不提示：research 无 change 语境，提示是噪音。已裁决不做，列此存档。
