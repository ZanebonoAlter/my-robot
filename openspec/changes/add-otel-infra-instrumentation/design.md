## 背景

当前 trace 已有三层埋点（见 `docs/reference/architecture/tracing.md`）：otelgin（HTTP 入站）、go-instrument（6 个业务方法 AST 注入）、手动业务 span（`workflow.*` / `scheduler.*` / `Router.Chat` 业务 attributes）。但**两个基础设施数据层无 span**：

- **DB**：仅 `SlowLogger` 记慢 SQL，与 trace 割裂；一个请求跑了多少 SQL、链路在 SQL 上耗多少时间，trace 里看不到
- **出站 HTTP**：`airouter`（LLM/Embedding）、`firecrawl`、`fingenius` 全是裸 `&http.Client{}`，链路一到"调外部"就断；日报/打标签的核心耗时在 trace 里消失

且采样/开关配置写死 `tracing.DefaultConfig()`，无环境变量、全量入库。

本 change 补 DB + 出站 HTTP 两层自动 span + 采样配置，**零业务代码改动**（otelgorm 中间件 / otelhttp transport）。

## 目标 / 不做

**目标**
- GORM 操作自动成 span（`SpanKind=Client`，附 `db.statement` / `db.operation`），挂当前 trace 父节点
- 出站 HTTP 自动成 span（`SpanKind=Client`）+ 透传 `traceparent`
- 采样可配（`ParentBased(TraceIDRatioBased)`），配置外化到环境变量
- 不动 domain 业务逻辑、不动 go-instrument 已注入的 6 个方法

**不做**（见 proposal §D）：go-instrument 脚本化 / 扩覆盖、前端浏览器 trace、异步 helper、go-instrument 生成 span 的 attributes/events 补全。

## 决策

### 1. DB 层自写 GORMTracePlugin（实现期由 gormotel 调整）

- **选**：自写 `internal/platform/tracing/gorm_plugin.go`（实现 GORM `Plugin` 接口 + 注册 create/query/update/delete/row/raw 的 before/after callback），db 初始化处 `db.Use(tracing.NewGORMPlugin())` 挂载，给每个操作建 `SpanKind=Client` span（`db.system` / `db.operation` / `db.statement`），挂当前 trace。**gorm 保持 v1.25.12 不动。**
- **实现期调整（原方案 gormotel）**：design 初版选 `gorm.io/plugin/opentelemetry` 官方插件，但 apply 发现其最新版 v0.1.16 **强制 gorm v1.30**，会把项目 gorm 从 v1.25.12 大版本升级 —— 超 change 范围、有核心数据层运行时风险。故改为自写轻量 plugin（~90 行），完全可控、无第三方版本耦合。语义属性 key 改用硬编码（`attribute.String("db.operation", ...)`）避开 semconv v1.26 的 DB key 漂移。
- **备选**：只保留 SlowLogger —— 慢 SQL 与 trace 割裂，timeline 看不到 DB 耗时，核心诉求无法满足

### 2. 出站 HTTP 用统一工厂 `httpclient.New`，不替换全局 `http.DefaultTransport`

- **选**：新建 `internal/platform/httpclient`，`New(opts...)` 内部 `otelhttp.NewTransport` 包 transport，返回 `*http.Client`；现有 ~10 处裸 `&http.Client{}` 改调工厂
- **理由**：Go net/http 无全局 transport hook 机制；替换 `http.DefaultTransport` 影响面大、风险高（波及第三方库的隐式调用）。工厂显式可控，各调用点 Timeout 等自定义经 functional opts 保留。
- **备选**：替换 `http.DefaultTransport` 为 otelhttp 包装 —— 风险大，影响所有隐式依赖 DefaultTransport 的代码
- **范围与优先级**：`airouter`（LLM/Embedding，核心耗时）优先；`dataenrichment` / `admin` 其次。本 change 一次全包，避免遗漏导致链路再断。

### 3. 采样用 `ParentBased(TraceIDRatioBased(ratio))`

- **选**：root span（无父：otelgin HTTP 入站、scheduler tick、go-instrument 方法 root）按 `ratio` 采样；root 被采后，其下所有子 span（DB / 出站 HTTP / 业务）全采
- **理由**：OTel 官方推荐默认 sampler。语义恰好满足"整条链路要么完整记录要么不记"，不在中间断链。`ratio=1.0` 全采（单用户本地默认），嫌吵 / 量大调小。
- **备选**：`AlwaysOn`（等同 ratio=1.0 但不可调）
- **备选**：按 span name 过滤 GORM span —— 增加复杂度；且 GORM span 在树形 timeline（`BuildSpanTree`）里挂在业务 span 下，不平铺淹没主链路，无必要

### 4. 配置外化到环境变量

- **选**：`tracing.Config` 新增 `SampleRatio` / `InstrumentGORM` / `InstrumentHTTP`，`cmd/server/main.go` 从环境变量加载（沿用项目现有 env 加载模式）；不配则默认 `SampleRatio=1.0`、两开关 `true`
- **理由**：现状 `DefaultConfig()` 写死，无法按环境调。外化后生产可降采样、可临时关 DB trace 排障。
- **不破坏**：不配任何环境变量 → 行为等同现状（全采、全开）。

### 5. 与 go-instrument 的关系（边界）

- go-instrument 走"AST 改源码给方法加 span"，覆盖 6 个方法，属**方法级自动**
- 本 change 走"基础设施中间件自动"（gormotel / otelhttp），**不动业务源码**
- 两者互补：本 change 补的是 go-instrument **管不到**的层（每个 GORM 操作、每个出站 HTTP）
- 共享同一 `TracerProvider`，本 change 的采样策略同时作用于 go-instrument 生成的 span
- go-instrument 的扩覆盖 / 脚本化属独立 change（proposal §D 已声明 out of scope）

## 风险 / 取舍

- **[otel_spans 涨量]** → 加 otelgorm 后每个请求多若干 DB span。**缓解**：`ParentBased` 采样 + 现有 7 天 retention（`scheduler.go`）兜底；`SampleRatio` 可调。
- **[timeline 噪音]** → GORM `SELECT` 类 span 数量多。**缓解**：树形 `BuildSpanTree` 把 DB span 挂业务 span 下，不平铺；timeline 展示层折叠属后续（非本 change 范围）。
- **[出站 client 改 ~10 处]** → **缓解**：机械替换 `&http.Client{}` → `httpclient.New(...)`，Timeout 等参数透传；每处改完跑影响包测试。
- **[gormotel 记录含参数 SQL]** → 可能记敏感参数。**缓解**：GORM 默认占位符化参数；单用户本地项目风险低；必要时 gormotel 支持禁用 statement 记录。
- **[出站调用方多样性]** → 各处 Timeout / 自定义不同。**缓解**：`httpclient.New` 接受 functional opts（`WithTimeout` 等）。
