## 背景

方法级 trace 现状：`go-instrument` 外部 CLI **手动跑过一次**（6 方法），无脚本化、无构建集成；其余业务 span 手写。要"全自动项目通用"，须把 AST 织入脚本化 + 规则化。

方向 A（`add-otel-infra-instrumentation`，补 DB / 出站 HTTP）**已落地并 archive**（`archive/2026-07-26-add-otel-infra-instrumentation`，tracing.md §1b）。本 change 在其基础上补"业务方法"层：方向 A 给"方法内部调的 DB / 出站 HTTP"加 span，AST 织入给"方法本身"加 span，合起来才是全链路全自动。

## 目标 / 不做

**目标**
- service 层 exported 方法自动获得 span（`TypeName.MethodName`），新增方法无需手写
- 织入幂等（重复跑不重复注入）、与手写 span 共存
- 接入 `go generate`，构建前自动跑

**不做**：基础设施层（方向 A）；注入 span 的业务 attributes 补全；非 service 层（domain / repository / handler）暂不覆盖。

## 决策

### 1. 选型：复用 go-instrument vs 自写织入器（已定路线②）

- **路线①（复用）**：`go-instrument` 已验证能注入基本 span + named err record。加规则配置 + 接 go generate。
  - 优：省实现成本，已在本项目验证
  - 劣：规则表达力受限（排除规则、span 名格式、attributes 定制可能不够）
- **路线②（自写）**：`go/ast` + `golang.org/x/tools/go/packages`，完全可控
  - 优：规则任意（排除已有 span、命名规范、未来加 attributes）
  - 劣：实现成本高（处理签名、ctx 传递、defer 顺序、注释保留）
- **决策**（2026-07-26，用户拍板）：采用**路线②（自写）**。理由：排除/幂等/范围完全自控；织入器本即 change 核心交付物，自写成本=核心资产；不依赖外部 CLI 版本。路线①原型取消。原型仍按《开发执行规范》§3 做最小可丢弃验证（识别 exported 方法 + 首参 ctx + 注入 span + 排除已有 span），通过即演进为 §2 正式实现。

### 2. 织入规则

- **目标范围**：`internal/*/service/**/*.go` + `internal/platform/airouter/*.go` 的 exported 方法（首字母大写）。纳入 airouter 是为统一覆盖现有 `Router.Chat` 等 go-instrument 方法（在 `platform/airouter/router.go`，不在 service 层）。
- **硬约束**：首参 `ctx context.Context`（无 ctx 的跳过）
- **span 名**：`TypeName.MethodName`（receiver 方法）/ `pkg.FuncName`（包级函数）
- **named err**：返回值列表有 named `err error` 时，注入 `defer func(){ if err != nil { span.RecordError(err); span.SetStatus(Error, "") } }()`
- **排除（幂等 + 共存）**：方法体首部已含 `otel.Tracer(` / `tracer.Start(` / `tracing.Tracer(` 的，跳过（避免与现有 6 方法 + 手写 `workflow.*` 重复）
- **排除规则边界**：只认上述三种直接 API 形态。对经封装 helper（如 `tracing.TraceSchedulerTick`）创建 span 的方法有理论盲区——但当前 `TraceSchedulerTick` 调用点在 `admin/scheduler`、`platform/tracing`，均在织入范围**外**，不触发；未来若把此类入口挪进目标范围，需同步扩排除规则或加显式 `//noinstrument` 标记

### 3. 构建集成

- 各 service 包加 `//go:generate` 指令，或集中 Makefile `make instrument` target
- CI 构建前跑 `go generate ./...`，保证 span 总是最新
- 本地：改完 service 方法后跑一次 generate

### 4. 与手写 span 共存

- 手写 span（`workflow.*` / `scheduler.*` / `Router.Chat` 业务 attributes）**保留** —— 它们带业务语义，规则化织入补不到
- 织入器排除规则保证不重复注入
- 未来统一（把 workflow.* 也纳入织入 + 规则化 attributes）属独立 change

### 5. 与方向 A 的边界（方向 A 已落地）

- 方向 A：DB 操作（自写 `GORMTracePlugin`）+ 出站 HTTP（`httpclient` 工厂 otelhttp 包装）自动 span —— **已 archive 落地**（`archive/2026-07-26-add-otel-infra-instrumentation`，tracing.md §1b）
- 本 change：业务方法 span 自动化（AST 织入）
- 两者改不同地方、共享同一 `TracerProvider`，本 change 在方向 A 之上补方法层，合起来 = 全链路全自动

## 风险 / 取舍

- **[源码被工具改写]** → 注入代码带 `/*line*/` 标记保持调试定位；git diff 噪音靠 review 习惯接受；织入器改动可一键重跑还原
- **[织入器 bug 破坏源码]** → 织入后必须 `go build ./...` + `go test` 通过才提交；织入器自身单测覆盖（签名识别、排除逻辑、幂等）
- **[与手写 span 冲突]** → 排除规则检测已有 `otel.Tracer` / `tracer.Start`，跳过；集成测试验证不重复
- **[ctx 非第一参数的方法被漏]** → 硬约束首参 ctx，不符合的不织入。可接受 —— 这类方法本就不适合自动 span（无 ctx 传递）
- **[选型选错]** → 先做可丢弃原型，两条路都验证再定，避免返工
- **[CI 忘跑 generate]** → go generate 接入 CI 必跑步骤；可选本地 pre-commit hook
