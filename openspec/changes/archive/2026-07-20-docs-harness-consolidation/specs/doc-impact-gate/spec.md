# doc-impact-gate (delta)

## ADDED Requirements

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

apply 启动时 SHALL 运行 `bash scripts/doc-impact.sh context` 获取与本次改动相关的业务约束上下文：脚本按 git diff 命中的代码 domain，从命中的 flow 文档「业务约束与不变量」节提取内容，作为改代码前的约束上下文注入 agent。本要求是**前置必读**而非归档断言——context 输出不构成归档门禁 FAIL 条件，但 §0.6 编排强制 apply 阶段执行。

业务约束↔flow 的关联 SHALL 通过 flow 文档「代码入口」节对 domain 包名的匹配建立，不依赖硬编码 domain→flow 映射表。

#### Scenario: 命中相关 flow

- **WHEN** git diff 含 `backend-go/internal/reader/service/crawler.go` 且 `docs/reference/flow/content-enrichment.md`「代码入口」节提及 reader domain
- **THEN** context 子命令输出 content-enrichment.md「业务约束与不变量」节全文

#### Scenario: 无命中

- **WHEN** git diff 未命中任何 flow 文档「代码入口」节关联的 domain
- **THEN** context 输出「未识别到相关业务约束 flow；如改动涉及业务逻辑，请主动查阅 docs/reference/flow/」且退出码 0

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
