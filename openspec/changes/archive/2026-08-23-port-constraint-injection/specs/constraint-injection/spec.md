# constraint-injection Specification（delta）

## ADDED Requirements

### Requirement: 档位识别与 change 绑定

extension SHALL 通过 `input` 事件识别阶段命令设置会话内档位（`requirements` / `implementation`），命令集覆盖本仓 openspec 斜杠命令（`/opsx-*`、`/skill:openspec-*`）与对应 skill 文件读取（`tool_execution_start` 的 read 路径命中 skill 目录）。

档位 SHALL 绑定活跃 change，绑定规则分档位：**输入提及优先**（命令参数/输入文本/写 change 目录文件时修正）；**mtime 最新兜底仅 implementation 档**（apply/verify/archive 无参语境）；requirements 档（explore/propose 新想法语境）SHALL NOT mtime 兜底——未明确提及时不绑定（关键词命中源不含无关 change 文本，pin_finding 落 research 库）。未激活档 SHALL 不显示、不分析任何 change。绑定 change 归档或删除后 SHALL 自动回落未激活档；agent 写 `openspec/changes/<name>/` 下文件时 SHALL 修正绑定。

#### Scenario: 斜杠命令激活档位

- **WHEN** 用户输入 `/opsx-apply add-change-scope`
- **THEN** 档位切换为 implementation 并绑定 change `add-change-scope`

#### Scenario: skill 读取激活档位

- **WHEN** agent 自动 read `.agents/skills/openspec-propose/SKILL.md`
- **THEN** 档位切换为 requirements（skill 路径 signal 命中）

#### Scenario: change 归档后回落

- **WHEN** 档位绑定的 change 目录已不存在（已归档）
- **THEN** 下一次 `before_agent_start` 前档位回落未激活，不再注入该 change 相关约束

#### Scenario: explore 新会话不绑定无关 change

- **WHEN** 新会话读 openspec-explore skill 激活 requirements 档，输入未提及任何 change，且存在 mtime 更新的其他 change 目录
- **THEN** 注入块活跃变更显示「无」，关键词命中源不含无关 change 文本，pin_finding 落 research 库而非 mtime 最新 change

#### Scenario: requirements 档提及后正常绑定

- **WHEN** requirements 档下用户输入提及某 change 名（如 `/opsx-continue <name>`）
- **THEN** 档位绑定该 change，其探索发现与约束照常注入

### Requirement: 每 turn system prompt 强制注入

`before_agent_start` SHALL 按档位命中将约束文档追加进 system prompt：未激活档仅注入索引文档；激活档注入 baseDocs + 关键词命中（change 文本 + 最近用户输入）+ JIT 路径命中（write/edit 路径）。**关键词命中与 JIT 命中 SHALL 会话内只增不减**（命中不因输入滚动窗词条滚出而移除、注入顺序稳定，保 system prompt 前缀缓存——系统提示变一字则全部历史缓存报废）。注入块 SHALL 标注「与 AGENTS.md 优先级宪法冲突时以宪法为准」。

文档命中 SHALL 复用 standard 文档既有 `doc-impact-applies` 标签生成 JIT pathSignals（单一真相源，不发明第二套映射）。

#### Scenario: 实现档注入生效

- **WHEN** implementation 档激活且 change 文本含后端 domain 信号
- **THEN** system prompt 追加含命中 standard/flow 文档的约束块，模型无法绕过

#### Scenario: 未激活档仅索引

- **WHEN** 会话未识别任何档位（普通问答/research 语境）
- **THEN** 仅注入约束索引文档，不做 change 文本关键词命中

#### Scenario: JIT 路径细化

- **WHEN** implementation 档激活且 agent edit `backend-go/internal/platform/airouter/router.go`
- **THEN** 命中该路径标签的 standard 文档（如 ai-logging.md）会话内追加注入

#### Scenario: flow 文档节级注入

- **WHEN** 档位激活且 change 文本/最近输入命中某 flow domain 关键词（如「板块」命中 semantic-board）
- **THEN** 注入内容为该 flow 文档「业务约束与不变量」节（非全文），节尾附全文路径指引

#### Scenario: 命中只增不减（缓存稳定）

- **WHEN** 关键词「板块」命中的文档已入注入块，后续输入使「板块」滚出最近输入窗
- **THEN** 该文档仍留在注入块中，块内既有内容与顺序字节不变

### Requirement: pin_finding 落点解析

extension SHALL 注册 `pin_finding` 工具持久化探索发现，落点与 research-retention 规则对齐，三级解析：

1. 显式 `change` 参数（目录存在）或档位激活（档位绑定优先；mtime 兜底仅 implementation 档）→ 活跃 change 的 `explore-findings.md`，implementation 档自动注入；
2. 无档且传 `topic` → `docs/research/<topic>/explore-findings.md`；
3. 无档无 topic → 通用池单文件 `docs/research/explore-findings.md`。

任何落点 SHALL NOT 写入 `docs/experience/`。

#### Scenario: 实现阶段自动注入发现

- **WHEN** requirements 档 pin 了「告警表结构」发现，随后档位切换为 implementation
- **THEN** 该 explore-findings.md 内容随 system prompt 注入，实现阶段无需重探

#### Scenario: research 语境不落 change

- **WHEN** 无激活档位且未传 change 参数时调用 pin_finding
- **THEN** 落点为 `docs/research/` 下（传 topic 落 `<topic>/`，未传落通用单文件），不写入 `openspec/changes/`

#### Scenario: research 语境无 topic 落通用池

- **WHEN** 无激活档位且未传 change 与 topic 参数时调用 pin_finding
- **THEN** 落点为 `docs/research/explore-findings.md` 通用池单文件

### Requirement: smoke test 覆盖纯函数

extension 的纯逻辑（档位匹配、栈判定、速览提取、**节提取**、**命中粘性**、落点三级解析、命令匹配）SHALL 有可脱离 pi harness 运行的 smoke test（node 直跑 .cjs 模式，同源项目 `tests/*.smoke.cjs` 实践）。

#### Scenario: smoke test 直跑

- **WHEN** 执行 smoke test 脚本（不启动 pi）
- **THEN** 全部断言通过，退出码 0

#### Scenario: 节提取回落

- **WHEN** 配置了 `section` 的文档内不存在该 `## 节名`（文档结构变更未同步配置）
- **THEN** 回落全文注入（不报错、不静默跳过，fail-safe）
