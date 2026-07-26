## Why

当前链路追踪（`otelgin` + `go-instrument` + 手动业务 span）已覆盖 HTTP 入站和部分业务方法，但**两个基础设施数据层是黑洞**：

1. **DB 操作零 span**：每个 GORM 查询/写入不进 trace。一次请求跑了多少 SQL、哪条慢、链路在 SQL 上耗多少时间——全看不到。现仅有 `SlowLogger` 记慢查询，且与 trace 割裂。
2. **出站 HTTP 零 span**：`airouter`（LLM/Embedding）、`firecrawl`、`fingenius` 等外部调用全是裸 `&http.Client{}`，链路一到"调外部"就断。日报/打标签的核心耗时（LLM 调用）在 trace 里直接消失。

同时，trace 配置（采样、开关）全写死在 `tracing.DefaultConfig()`，无环境变量控制，`otel_spans` 全量入库无采样。

补 `otelgorm` + `otelhttp` + `ParentBased` 采样，让"DB + 出站 HTTP"这两层**零业务侵入**自动进 trace，配合现有 otelgin / go-instrument / 业务 span，凑齐完整链路。

## What Changes

### A. DB 操作自动 span（otelgorm）

引入 `gorm.io/plugin/opentelemetry`，在 db 初始化处 `db.Use(gormotel.NewPlugin())` 一行挂载。所有 GORM 操作自动成 span（`SpanKind=Client`，attributes 含 `db.statement` / `db.operation`），挂在当前 trace 父节点下。**零业务代码改动。**

### B. 出站 HTTP 自动 span（otelhttp 统一工厂）

新建 `internal/platform/httpclient`，提供 `httpclient.New(opts...)`，内部用 `otelhttp.NewTransport` 包装 transport + 自动透传 `traceparent`。将现有裸 `&http.Client{...}`（`airouter` / `dataenrichment` / `admin` 共约 10 处）替换为工厂调用。出站调用自动成 `SpanKind=Client` span。**改动集中在客户端构造层，不进 domain 业务逻辑。**

### C. 采样策略 + 配置外化

`InitTracerProvider` 增加 `ParentBased(TraceIDRatioBased(ratio))` 采样器。`tracing.Config` 新增 `SampleRatio`（默认 `1.0`）、`InstrumentGORM` / `InstrumentHTTP` 开关，从环境变量读取（`TRACE_SAMPLE_RATIO` 等）。语义：入站请求按 `ratio` 决定整条链路记不记，被采链路下所有子 span（DB / 出站 HTTP / 业务）完整保留，不在中间断链。

### D. Out of scope（明确不做，避免重复）

- **go-instrument 脚本化 / 扩大覆盖**：`docs/reference/architecture/tracing.md` §下一步建议第 3 条已列为未决项，属独立 change。本 change 不动 go-instrument 已注入的 6 个方法。
- **前端浏览器 trace**：单用户项目，需配 collector 收集，性价比低。
- **异步 trace helper**：`tracing.md` §5 已确认 helper 不存在，属独立 change。
- **go-instrument 已生成 span 的 attributes / events 补全**：属业务语义丰满，独立 change。

## Capabilities

### New Capabilities

无。本 change 扩展现有 tracing 能力，不引入新能力域。

### Modified Capabilities

- `otel-business-tracing`：新增「基础设施自动埋点（otelgorm / otelhttp）」与「采样策略与配置外化」两类 requirement。与现有业务 span 注入 requirement 互补——前者补 go-instrument 管不到的 DB / 出站 HTTP 层。

## Impact

- **代码**：
  - `internal/platform/database`：db 初始化加 gormotel 挂载（1 行 `db.Use`）
  - `internal/platform/httpclient`：新建包（factory + otelhttp transport）
  - `internal/platform/airouter`、`internal/dataenrichment/service`、`internal/admin/service`：裸 `&http.Client{}` → `httpclient.New()`（约 10 处）
  - `internal/platform/tracing/provider.go`：加 sampler；`config.go`：新增字段；`cmd/server/main.go`：配置从环境变量加载
- **依赖**：新增 `gorm.io/plugin/opentelemetry`；`go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` 从 indirect 升 direct
- **API**：无变化（`/api/traces/*` 接口不变，返回的 span 数据更丰富）
- **数据**：`otel_spans` 表行数显著增加（每个请求多若干 DB / 出站 HTTP span）；**无 schema 变更**（复用现有表，gormotel / otelhttp span 经同一 `DatabaseSpanExporter` 落库）；7 天 retention 兜底
- **配置**：新增环境变量（`TRACE_SAMPLE_RATIO` 等），不配则默认全采，行为等同现状
