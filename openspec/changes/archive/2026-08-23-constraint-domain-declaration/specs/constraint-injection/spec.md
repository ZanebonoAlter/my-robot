# constraint-injection Delta — 业务域显式声明

## MODIFIED Requirements

### Requirement: 每 turn system prompt 强制注入

`before_agent_start` SHALL 按档位命中将约束文档追加进 system prompt：未激活档仅注入索引文档；激活档注入 baseDocs + 业务域声明（proposal.md `constraint-domains` 标记）+ 关键词命中（仅最近用户输入，change 文本不参与）+ JIT 路径命中（write/edit 路径）。**关键词命中与 JIT 命中 SHALL 会话内只增不减**（命中不因输入滚动窗词条滚出而移除、注入顺序稳定，保 system prompt 前缀缓存——系统提示变一字则全部历史缓存报废）。注入块 SHALL 标注「与 AGENTS.md 优先级宪法冲突时以宪法为准」。

关键词命中源 SHALL 仅含最近用户输入（滚动窗），change 产物全文（proposal/design/tasks）MUST NOT 触发关键词命中——harness/AI 类 change 的正当词汇与业务域关键词本质重叠，隐式推断必致误注入。ASCII 关键词 SHALL 按词边界整词匹配（CJK 关键词保持子串匹配）。

业务域声明 SHALL 解析 proposal.md 头部的 `<!-- constraint-domains: <域>, ... -->` HTML 注释标记（可多行合并），域名合法值 SHALL 为 `docs/reference/flow/*.md` 的 basename；未知域名 SHALL 宽容忽略并在状态提示；无声明 SHALL 不注入任何 flow 约束节并在状态栏提示（不阻断）。声明域注入每回合从文件重解析，SHALL NOT 进入只增不减粘性集合。

JIT pathSignals SHALL 复用文档既有 `doc-impact-applies` frontmatter 标签生成（standard 与 flow 文档统一单一真相源，不发明第二套映射），`.pi/constraint-injection.json` MUST NOT 手写 pathSignals。

#### Scenario: 实现档注入生效

- **WHEN** implementation 档激活且 proposal.md 声明 `constraint-domains: daily-report`
- **THEN** system prompt 追加含 daily-report.md「业务约束与不变量」节的约束块，模型无法绕过

#### Scenario: 未激活档仅索引

- **WHEN** 会话未识别任何档位（普通问答/research 语境）
- **THEN** 仅注入约束索引文档，不做域声明注入与关键词命中

#### Scenario: JIT 路径细化

- **WHEN** implementation 档激活且 agent edit `backend-go/internal/topicgraph/service/daily_report_orchestrator.go`
- **THEN** 命中 daily-report.md 头部 `doc-impact-applies` 标签所含路径前缀，该文档「业务约束与不变量」节会话内追加注入（json 无手写 pathSignals）

#### Scenario: flow 文档节级注入

- **WHEN** 档位激活且声明域或最近输入命中某 flow 域
- **THEN** 注入内容为该 flow 文档「业务约束与不变量」节（非全文），节尾附全文路径指引

#### Scenario: change 文本不触发关键词命中

- **WHEN** implementation 档激活且 change 的 proposal/design/tasks 全文含 `stage`、`topic`、`digest`、`模型` 等历史撞车词，但 proposal 未声明对应域、最近输入亦未提及
- **THEN** 不注入 semantic-board / topic-graph / daily-report / ai-summary 任一域约束节

#### Scenario: ASCII 关键词词边界整词匹配

- **WHEN** 最近输入含英文单词 "stage"（不含独立词 "tag"）
- **THEN** 不命中 semantic-board 域关键词

#### Scenario: 命中只增不减（缓存稳定）

- **WHEN** 档位激活且用户输入「日报」命中 daily-report 域，后续输入使该词滚出最近输入窗
- **THEN** 该文档仍留在注入块中，块内既有内容与顺序字节不变

#### Scenario: 无声明不注入并提示

- **WHEN** implementation 档激活且绑定 change 的 proposal.md 无 `constraint-domains` 标记
- **THEN** 不注入任何 flow 约束节，状态栏 widget 提示「无域声明」，不阻断会话

#### Scenario: 未知域名宽容忽略

- **WHEN** proposal.md 声明 `constraint-domains: daily-report, not-a-domain`
- **THEN** 注入 daily-report 域约束节，忽略 `not-a-domain` 并在状态提示
