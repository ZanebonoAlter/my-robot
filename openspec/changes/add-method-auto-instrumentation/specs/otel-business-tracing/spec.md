## ADDED Requirements

### Requirement: Service-layer methods are auto-instrumented via build-time AST weaving

系统 SHALL 提供一个 AST 织入器（复用 `go-instrument` 或自写 `go/ast` 工具，apply 阶段选型定），在构建前（`go generate` 或 Makefile target）自动扫描 `internal/*/service/**/*.go` 与 `internal/platform/airouter/*.go` 的 exported 方法，为每个首参为 `ctx context.Context` 的方法注入 OpenTelemetry span（span 名 `TypeName.MethodName`），不要求开发者手写埋点代码。

织入 SHALL 幂等：方法体已含 `otel.Tracer` / `tracer.Start` / `tracing.Tracer` 调用的，SHALL 跳过（避免与现有 go-instrument 注入方法、手写业务 span 重复注入）。

当方法返回值含 named `err error` 时，织入 SHALL 额外注入错误记录（`span.RecordError(err)` + `span.SetStatus(Error)`）。

#### Scenario: 新 service 方法自动获得 span

- **WHEN** 开发者在 `internal/<x>/service/` 下新增 exported 方法 `FooBar(ctx context.Context, ...) error`，并运行 `go generate`
- **THEN** 该方法 SHALL 被注入 span（名 `<TypeName>.FooBar`），含 named err 错误记录，且 `otel_spans` 在调用时记录该 span

#### Scenario: 幂等不重复注入

- **WHEN** 织入器对同一文件连续运行两次
- **THEN** 第二次 SHALL 不产生重复 span 代码（检测到已有 `otel.Tracer` / `tracer.Start` 即跳过）

#### Scenario: 与手写业务 span 共存不冲突

- **WHEN** 某方法已手写 `tracing.Tracer(...).Start(...)`（如 `workflow.daily_report.generate`）
- **THEN** 织入器 SHALL 跳过该方法，不注入第二个 span

#### Scenario: 无 ctx 首参的方法被跳过

- **WHEN** 某 exported 方法首参不是 `context.Context`
- **THEN** 织入器 SHALL 跳过该方法（不注入 span），避免无 ctx 传递的无效 span

#### Scenario: 构建集成

- **WHEN** CI / 本地构建执行 `go generate ./...`（或 Makefile instrument target）
- **THEN** 所有目标 service 包 SHALL 被织入器扫描并更新，构建产物含最新 span 埋点
