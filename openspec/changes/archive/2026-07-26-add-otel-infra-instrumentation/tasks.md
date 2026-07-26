## 0. 实现期调整（记录）

- **gormotel → 自写 GORMTracePlugin**：原方案用 `gorm.io/plugin/opentelemetry`，apply 发现其 v0.1.16 强制 gorm v1.30（项目 v1.25.12），超 change 范围。改为自写 `internal/platform/tracing/gorm_plugin.go`（~90 行，gorm 保持 v1.25）。见 design 决策 1。
- **自循环 bug 修复**：初版 plugin 给所有 GORM 操作建 span，导致 `DatabaseSpanExporter` 写 `otel_spans` 触发 span→export→写入自循环（实测 10 分钟 gorm.create 98 万次）。修复：plugin before/after 跳过 `otel_spans` 表自身（`(OtelSpan{}).TableName()`）。
- **裸 client 清单补漏**：最早 grep 被 `head -30` 截断，漏掉 `internal/reader/service/`（firecrawl/readability/rss_parser）3 处，apply 中补上。

## 1. DB 自动埋点（自写 GORMTracePlugin）

- [x] 1.1 不引入 gormotel（避免 gorm 升级）；自写 `internal/platform/tracing/gorm_plugin.go`（GORM Plugin + create/query/update/delete/row/raw 的 before/after callback）
- [x] 1.2 `connect_postgres.go` 的 `gorm.Open` 后 `db.Use(tracing.NewGORMPlugin())`（受 `cfg.Tracing.InstrumentGORM` 控制）
- [x] 1.3 跳过 `otel_spans` 表自身，避免 exporter 自循环；span 经现有 `DatabaseSpanExporter` 落库（无新表）

## 2. httpclient 统一工厂

- [x] 2.1 新建 `internal/platform/httpclient`：`New(opts ...Option) *http.Client`，functional opts（`WithTimeout` / `WithTransport`）
- [x] 2.2 `InstrumentHTTP=true` 时 `otelhttp.NewTransport` 包装；`false` 时返回未包装 client
- [x] 2.3 单测 6 个：transport 包装 / 开关切换 / opts —— 全 PASS

## 3. 替换裸 `&http.Client{}`

- [x] 3.1 `airouter/openai_compatible.go` 2 处（Chat/Embed）
- [x] 3.2 `airouter/fallback.go`
- [x] 3.3 `airouter/test_connection.go`
- [x] 3.4 `dataenrichment/service/fingenius_client.go` 2 处
- [x] 3.5 `dataenrichment/service/tool_registry.go`
- [x] 3.6 `admin/service/catalog_extras.go`
- [x] 3.7 `admin/service/catalog_sync_service.go`
- [x] 3.8 `reader/service/{firecrawl,readability,rss_parser}` 3 处（补漏）
- [x] 3.9 grep 复核：`grep -rn '&http.Client{' internal --include='*.go' | grep -v _test | grep -v httpclient` → 无残留 ✓

## 4. 采样策略 + 配置外化

- [x] 4.1 `tracing/config.go` + `config/config.go`：`SampleRatio` / `InstrumentGORM` / `InstrumentHTTP` + viper 默认 + env 覆盖
- [x] 4.2 `tracing/provider.go`：`WithSampler(ParentBased(TraceIDRatioBased(cfg.SampleRatio)))`
- [x] 4.3 `cmd/server/main.go`：traceCfg 从 `config.AppConfig.Tracing` 加载 + `httpclient.SetInstrumentation`
- [x] 4.4 采样行为经 E2E 实测验证（ratio=1.0 默认全采，链路完整；见 §5）

## 5. 端到端验证（实测）

- [x] 5.1 访问 `POST /api/discovery/ask`（含 DB + 出站 LLM），trace 树含：HTTP Server → Router.Embed/Router.Chat（业务）→ HTTP POST（出站 Client，挂在 Router.Chat 下）+ gorm.query/create/row（DB Client）。**全链路打通** ✓
- [x] 5.2 自循环修复验证：TRUNCATE 后 6 秒 `gorm.create=0`、total=27（scheduler 空转），无指数膨胀 ✓

## 6. 测试

后端受影响包全 PASS：

- [x] `go test ./internal/platform/tracing → PASS`
- [x] `go test ./internal/platform/database → PASS`（8s，含 DB 集成）
- [x] `go test ./internal/platform/httpclient → PASS`（6 单测）
- [x] `go test ./internal/platform/config → PASS`
- [x] `go test ./internal/platform/airouter → PASS`
- [x] `go test ./internal/reader/service → PASS`
- [x] `go test ./internal/dataenrichment/service → PASS`
- [x] `go test ./internal/admin/service → PASS`

## 7. 文档

<!-- doc-impact: architecture configuration -->
<!-- doc-impact-excuse: database=接入点挂载非schema; flow=其他change脏文件 -->

- [x] `docs/reference/architecture/tracing.md`：新增 §1b「基础设施自动埋点（DB + 出站 HTTP）」+ 代码结构表补 `gorm_plugin.go`/`httpclient` + 下一步建议勾掉「外部调用 CLIENT span」项（已补）
- [x] `docs/reference/configuration.md`：后端环境变量表补 `TRACE_SAMPLE_RATIO` / `TRACE_INSTRUMENT_GORM` / `TRACE_INSTRUMENT_HTTP` + 默认值表补采样默认
- [x] 无 flow 影响（可观测层不改业务流），按《开发执行规范》§12.2 豁免溯源

## 8. 验证

- [x] `cd backend-go && go build ./... → 成功`（EXIT 0）
- [x] `cd backend-go && go vet ./... → 零警告`（EXIT 0）
- [x] `cd backend-go && gofmt -l` 改动文件 → 无未格式化（存量 test 文件未格式化非本次引入，surgical 不修）
- [x] `cd backend-go && go test` 受影响包 → 全 PASS（见 §6）
- [x] `grep -rn '&http.Client{' backend-go/internal --include='*.go' | grep -v _test | grep -v httpclient/httpclient.go` → 无残留
- [x] `go.mod` gorm 保持 v1.25.12（未升级）；otel 1.42→1.44（小升级，兼容）；otelhttp 转 direct
- [x] E2E：访问接口 → `otel_spans` 含 `gorm.*`（DB Client）+ `HTTP POST`（出站 Client）+ HTTP Server 父子树完整（见 §5）
