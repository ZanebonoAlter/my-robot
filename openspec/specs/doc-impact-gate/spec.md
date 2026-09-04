# doc-impact-gate Specification

## Purpose
TBD - created by archiving change docs-harness-consolidation. Update Purpose after archive.
## Requirements
### Requirement: 文档影响声明（apply 启动时）

每个 openspec change 在 apply 启动时 SHALL 运行 `bash scripts/doc-impact.sh suggest` 获取预勾选菜单，并将确认后的文档域声明以机器可读注释写入 tasks.md「文档」节第一行：

```markdown
<!-- doc-impact: flow api configuration -->
```

文档域 SHALL 为以下固定选项之一或多个：`flow` / `api` / `database` / `architecture` / `standard` / `configuration` / `deployment`；无文档影响时 SHALL 声明 `none` 并附理由。

#### Scenario: 声明写入 tasks.md

- **WHEN** 一个 change 进入 apply 阶段
- **THEN** 其 tasks.md「文档」节第一行存在 `<!-- doc-impact: ... -->` 注释
- **AND** 声明的每个文档域在下方有对应的具体文档 checkbox

#### Scenario: 纯代码 change 声明 none

- **WHEN** 一个 change 确认无文档影响
- **THEN** tasks.md 声明 `<!-- doc-impact: none(理由) -->`

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

### Requirement: 归档前对账（verify）

归档门禁 SHALL 运行 `bash scripts/doc-impact.sh verify <change-dir>` 对每个 active change 对账，以下任一条件 SHALL 判 FAIL：

- tasks.md 缺 `doc-impact` 声明注释
- 声明的文档文件未出现在 git 改动集合（`git diff --name-only <base>` + 未跟踪文件）中
- 反向启发式命中未声明域（如 `internal/*/handler/` 改动但声明中无 `api`）
- 声明 `none` 但任一启发式命中
- 声明的文档文件路径不存在

#### Scenario: 声明了未更新

- **WHEN** tasks.md 声明 `docs/reference/api/feeds.md` 但 git 改动集合中无此文件
- **THEN** verify 输出 `声明了未更新: docs/reference/api/feeds.md` 且退出码非零

#### Scenario: 疑似遗漏

- **WHEN** git 改动集合含 `backend-go/internal/reader/handler/foo.go` 且声明中无 `api`
- **THEN** verify 输出 `疑似遗漏: 改了 handler 未声明 api` 且退出码非零

#### Scenario: 历史存量豁免

- **WHEN** change 归档日期早于 doc-impact-gate 生效 cutoff
- **THEN** check-standards.sh F 段跳过该校验

### Requirement: 文档死链检查

归档门禁 SHALL 对导航层文档（`docs/README.md`、`docs/reference/*.md` 一级、`docs/reference/flow/README.md`、`docs/reference/architecture/map.md`）执行 markdown 相对链接死链检查，失效链接 SHALL 判 FAIL。

#### Scenario: README 死链

- **WHEN** `docs/README.md` 含相对链接 `](getting-started.md)` 且 `docs/getting-started.md` 不存在
- **THEN** check-standards.sh G 段输出 FAIL 并列出该链接

### Requirement: 归档命令硬门禁（spec-gate）

pi 扩展 `.pi/extensions/spec-gate.ts` SHALL 拦截 bash 工具中匹配 `openspec archive` 的命令，在放行前强制执行四项检查：① `scripts/doc-impact.sh verify <change-dir>` 退出码为 0；② `scripts/check-standards.sh --change <change>` 无失败；③ 该 change 的 tasks.md 含「测试/文档/验证」尾三节及 doc-impact 标记；④ `scripts/scenario-trace.sh <change-dir>` 退出码为 0。`check-standards.sh --change <change>` SHALL 保留仓库级 A-E、G-H 标准检查，但 F 段只校验该目标 change 的 doc-impact，MUST NOT 因其他 active change 的 doc-impact 失败而失败。未传 `--change` 的手动 `check-standards.sh` SHALL 继续校验全部 active change。目标 change 不存在或自身 doc-impact 失败时，范围校验 MUST 失败并给出明确原因。任一失败 MUST block 并输出中文 reason（列失败项 + 修复指引）。豁免通道：命令显式带 `--force` 或环境变量 `SPEC_GATE_BYPASS=1`（MUST 记 warning，不得静默放行）。开关：`SPEC_GATE_ENABLE`（默认开启）。

#### Scenario: 门禁未过时归档被拦截
- **WHEN** agent 执行 `openspec archive <change>` 且目标 change 的 doc-impact 对账失败
- **THEN** 命令被 block，reason 列出目标 change 的失败项与修复指引

#### Scenario: 三项全过时放行
- **WHEN** 目标 change 的 doc-impact、任务结构、Scenario 映射以及 `check-standards.sh --change <change>` 的仓库级检查均通过
- **THEN** 归档命令正常执行

#### Scenario: 无关 active change 不阻断目标归档
- **WHEN** 目标 change 的 doc-impact 对账通过，而另一 active change 的对账失败
- **THEN** `check-standards.sh --change <目标change>` 的 F 段只报告目标 change 通过，spec-gate 不因另一 change 的失败阻断归档

#### Scenario: 目标 change 对账失败仍阻断归档
- **WHEN** 归档目标的 tasks.md 缺声明或其 doc-impact 对账失败
- **THEN** `check-standards.sh --change <目标change>` 返回非零，spec-gate 阻断归档并指向目标 change 的失败原因

#### Scenario: 手动全仓巡检保持全量语义
- **WHEN** 开发者不带参数执行 `bash scripts/check-standards.sh`
- **THEN** F 段继续遍历全部 active change 并报告每个 change 的 doc-impact 状态

#### Scenario: 归档目标不存在
- **WHEN** 调用 `check-standards.sh --change` 时目标 change 目录不存在
- **THEN** 脚本返回非零并输出目标 change 不存在，不执行静默的全仓回退

#### Scenario: 归档门禁使用目标范围
- **WHEN** agent 执行不带 bypass 的 `openspec archive <change>`
- **THEN** spec-gate 将该 change 传入 `check-standards.sh --change`，其余归档检查与原有阻断语义不变

#### Scenario: 显式豁免留痕
- **WHEN** 命令带 `--force` 或 `SPEC_GATE_BYPASS=1`
- **THEN** 归档放行且记录一条 warning（不静默）

### Requirement: quota-gate fail-open 落盘可观测

quota-gate 在额度查询失败而放行（fail-open）时 MUST 追加一条 custom_message 落盘记录（含失败原因），使"查询失败放行"与"未触发"在会话记录中可区分。

#### Scenario: 查询失败放行留痕
- **WHEN** quota 查询接口失败且按 fail-open 策略放行派发
- **THEN** 会话记录中出现一条 custom_message 说明"quota 查询失败已放行"及原因

### Requirement: 全量测试软守卫（test-scope-guard）

pi 扩展 `.pi/extensions/test-scope-guard.ts` SHALL 在检测到非归档语境下的全量 `go test ./...` 命令时发出软提醒（notify，不 block）。模式开关 `TEST_SCOPE_GUARD=soft|hard|off`，默认 soft。

#### Scenario: 日常误跑全量测试触发提醒
- **WHEN** 会话语境非归档且命令命中全量 `go test ./...`
- **THEN** 收到软提醒（建议只跑影响包），命令不被阻断

