## Why

方法级 span 现状是"半手动 + 一次性"：

- `go-instrument` 是个外部 CLI（`go install` 装，不在 go.mod），**当初手动跑过一次**，给 6 个方法（`FeedService.RefreshFeed`、`FirecrawlService.ScrapePage`、`ContentCompletionService.*`、`Router.Chat`）注入了 span。**全项目无 `go:generate`、无 Makefile、无脚本调用它** —— 新增方法不会自动有 span。
- 其余业务方法手写 span：`workflow.daily_report.*` 8 处（`topicgraph/service/daily_report_*.go`）、`workflow.article_tagging` 1 处（`tagmanagement/service/core/tag_queue.go` 的 `TagQueue` —— 历史文档误称 `SummaryQueue`，实测无该类型）；9 个 `scheduler.*` 入口经 `tracing.TraceSchedulerTick` 封装创建（非散落手写，且调用点在 `admin/scheduler`、`platform/tracing`，均在织入范围外）。
- 结果：链路里"业务方法耗时"覆盖零散、不可持续、靠人记。

要"项目通用、不一个个加 span"，必须把 AST 织入**脚本化 + 规则化批量覆盖**，让新增 service 方法自动获得 span。`docs/reference/architecture/tracing.md §下一步建议` 第 2 条（「把 `go-instrument` 接入 `go generate`」）已点名此为未决项。

## What Changes

### A. 选型原型（必做，可丢弃）

按《开发执行规范》§3，AST 织入属"算法逻辑"，先做可丢弃原型对比：

- **路线①（复用）**：现有 `go-instrument` 加规则配置（目标包、排除）+ 接 `go generate`。省实现成本，但规则表达力受工具现有能力限制。
- **路线②（自写）**：`go/ast` + `golang.org/x/tools/go/packages` 写织入器，完全可控规则（排除、命名、未来 attributes）。实现成本高。

选型依据：排除规则 / 幂等 / span 名规范 / 维护成本。定案后销毁原型再进正式实现。

### B. 织入规则

- **目标**：`internal/*/service/**/*.go` + `internal/platform/airouter/*.go` 的 exported 方法（airouter 纳入以统一覆盖现有 `Router.Chat` 等 go-instrument 方法，它在 `platform/airouter/router.go` 不在 service 层）
- **硬约束**：首参 `ctx context.Context`（无 ctx 的跳过 —— OTel span 需 ctx 传递）
- **span 名**：`TypeName.MethodName`
- **named err**：返回值有 named `err error` 时自动 `RecordError` + `SetStatus(Error)`
- **排除（幂等 + 共存）**：方法体已含 `otel.Tracer(` / `tracer.Start(` / `tracing.Tracer(` 的跳过

### C. 接入构建

`//go:generate` 指令（或 Makefile target），编译前自动跑织入器覆盖全部目标包；CI 构建前必跑。

### D. 迁移与统一

现有 6 个 go-instrument 方法纳入新流程统一管理；手写 `workflow.*` / `scheduler.*` 保留（它们带业务 attributes，规则化织入管不到语义）。

### Out of scope

- 基础设施层（DB / 出站 HTTP）→ 方向 A（`add-otel-infra-instrumentation`）**已落地并 archive**（`archive/2026-07-26-add-otel-infra-instrumentation`，tracing.md §1b），本 change 在其之上补业务方法层
- 注入 span 的业务 attributes / events 补全 → 独立 change（规则化织入只保证"有 span"，不补语义）

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `otel-business-tracing`：新增「方法级自动埋点（AST 织入）」requirement。与现有业务 span 注入 requirement 互补 —— 前者解决"方法 span 自动化覆盖"，后者规范手写业务 span 的语义。

## Impact

- **代码**：
  - 新增织入器：`backend-go/cmd/instrumenter`（自写路线）或脚本（复用路线），apply 选型后定
  - `//go:generate` 指令散布于各 service 包（或集中 Makefile target）
  - **所有目标 service 包源文件被织入器改写**（注入 span 代码，带 `/*line*/` 标记）
- **依赖**：自写路线用标准库 `go/ast`；复用路线 `go install github.com/<go-instrument>`
- **数据**：`otel_spans` 行数增加（更多方法 span）；无 schema 变更
- **构建**：新增 `go generate` 步骤（CI / 本地构建前必跑）
- **风险**：源码被工具改写（git diff 噪音、review 成本）；与手写 span 冲突（靠排除规则缓解）；织入器 bug 破坏源码（靠 `go build` + 测试兜底）
- **量化基线**：service 层 exported 方法约 155 个，首参为 `ctx context.Context` 的约 84 个（airouter 另有若干），无 ctx 首参的约 71 个将被合理跳过；`admin/service`、`dataenrichment/service` 当前 0 span，本次首次注入，diff 增量最大
