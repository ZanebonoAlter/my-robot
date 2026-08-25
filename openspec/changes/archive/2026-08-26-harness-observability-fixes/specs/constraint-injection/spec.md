# constraint-injection Delta

## MODIFIED Requirements

### Requirement: 每 turn system prompt 强制注入

`before_agent_start` SHALL 按档位命中将约束文档追加进 system prompt：未激活档仅注入索引文档；激活档注入 baseDocs + 业务域声明（proposal.md `constraint-domains` 标记）+ 关键词命中（仅最近用户输入，change 文本不参与）+ JIT 路径命中（write/edit 路径）。**关键词命中与 JIT 命中 SHALL 会话内只增不减**（命中不因输入滚动窗词条滚出而移除、注入顺序稳定，保 system prompt 前缀缓存——系统提示变一字则全部历史缓存报废）。注入块 SHALL 标注「与 AGENTS.md 优先级宪法冲突时以宪法为准」。

关键词命中源 SHALL 仅含最近用户输入（滚动窗），change 产物全文（proposal/design/tasks）MUST NOT 触发关键词命中——harness/AI 类 change 的正当词汇与业务域关键词本质重叠，隐式推断必致误注入。ASCII 关键词 SHALL 按词边界整词匹配（CJK 关键词保持子串匹配）。

业务域声明 SHALL 解析 proposal.md 头部的 `<!-- constraint-domains: <域>, ... -->` HTML 注释标记（可多行合并），域名合法值 SHALL 为 `docs/reference/flow/*.md` 的 basename；未知域名 SHALL 宽容忽略并在状态提示；无声明 SHALL 不注入任何 flow 约束节并在状态栏提示（不阻断）。声明域注入每回合从文件重解析，SHALL NOT 进入只增不减粘性集合。

JIT pathSignals SHALL 复用文档既有 `doc-impact-applies` frontmatter 标签生成（standard 与 flow 文档统一单一真相源，不发明第二套映射），`.pi/constraint-injection.json` MUST NOT 手写 pathSignals。

节级注入 SHALL 设最小字节下限（`minSectionBytes`，配置缺省 512）：提取的节内容低于下限时 SHALL 回退注入该文档全文（fail-safe，与「配置的节不存在时回落全文」同族），避免文档编辑中间态注入残缺节。回退全文时 `constraint.inject` 记账 bytes SHALL 为实际注入的全文字节数（不虚标节字节数）。

注入块（文档节 + change 级文件）总量 SHALL 受 `budgetBytes` 预算限制（配置缺省默认 32768 字节）。超预算时 SHALL 按命中信号强度分层降级（先降推断信号、后降声明信号：关键词命中节 → JIT 命中节 → change 级文件 digest 收紧 → 域声明节 → change 级文件降占位；baseDocs 与注入块 header SHALL NOT 降级）。降级 MUST NOT 真丢文档——每个命中文档至少保留一行占位（标题 + `read` 全文路径），模型可自行补取。降级决策 SHALL 确定性（相同命中集合 + 相同文档内容 + 相同预算 → 相同降级结果，顺序按命中信号分层与首次命中顺序稳定）。超预算发生时注入块头部 SHALL 列出已降级路径（模型可见省略通知）。记账时被降级文档的 `constraint.inject` payload MUST 附降级标记与降级后字节数；预算降级到占位行的 explore-findings SHALL NOT 记 `pin.read`（模型未见 pin 标题）。

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

#### Scenario: 节残缺时回退全文

- **WHEN** 某 flow 文档「业务约束与不变量」节处于编辑中间态仅 133B（低于 minSectionBytes 下限），全文完整
- **THEN** 注入回退为该文档全文，constraint.inject 记账 bytes 为全文字节数（残缺节不进 system prompt）

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

#### Scenario: 超预算分层降级

- **WHEN** budgetBytes=16384 且当前命中文档节合计 43K（超预算）
- **THEN** 依分层顺序降级（关键词命中节先于 JIT 命中节、change 级文件先 digest 收紧、域声明节后降）直到总量的预算内，baseDocs（索引）与注入块 header 不降级

#### Scenario: 降级永不真丢

- **WHEN** 预算极小（如 budgetBytes=2048）且命中 3 个 flow 节
- **THEN** 每个命中文档至少保留一行「标题 + read 路径」占位，无任何文档从注入块中完全消失

#### Scenario: 模型可见省略通知

- **WHEN** 本回合发生降级
- **THEN** 注入块头部列出全部已降级路径，模型可据此 read 对应文档补取全文

#### Scenario: 降级确定性（缓存友好）

- **WHEN** 命中集合与文档内容、预算均未变，连续两个回合重建注入块
- **THEN** 两次降级结果与注入块字节完全一致

#### Scenario: 降级记账

- **WHEN** daily-report 约束节（7.8K）本回合被降级为占位行
- **THEN** 产生一条 constraint.inject，payload 附降级标记与降级后字节数（区别于未降级回合）

### Requirement: smoke test 覆盖纯函数

extension 的纯逻辑（档位匹配、栈判定、速览提取、**节提取**、**节最小字节判定与全文回退**、**命中粘性**、落点三级解析、命令匹配）SHALL 有可脱离 pi harness 运行的 smoke test（node 直跑 .cjs 模式，同源项目 `tests/*.smoke.cjs` 实践）。

#### Scenario: smoke test 直跑

- **WHEN** 执行 smoke test 脚本（不启动 pi）
- **THEN** 全部断言通过，退出码 0

#### Scenario: 节提取回落

- **WHEN** 配置了 `section` 的文档内不存在该 `## 节名`（文档结构变更未同步配置）
- **THEN** 回落全文注入（不报错、不静默跳过，fail-safe）

#### Scenario: 节低于最小字节下限回退全文

- **WHEN** smoke test 对节提取纯函数传入低于 `minSectionBytes` 的节内容与完整全文
- **THEN** 判定结果为回退全文（与节不存在回落同族），断言通过
