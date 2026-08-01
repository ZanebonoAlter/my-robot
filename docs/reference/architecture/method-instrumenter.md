# 方法级 OTel span 自动织入器（AST 织入器）

> 给 service 层方法**自动**加 OpenTelemetry span，开发者不用手写一行埋点代码。
> 源码：`backend-go/cmd/instrumenter/` · 架构定位见 [tracing.md](tracing.md) §2

## 它解决什么问题

方法级 span 以前的状况是"半手动、不可持续"：

- `go-instrument`（外部 CLI）**手动跑过一次**，给 6 个方法注入了 span（`FeedService.RefreshFeed`、`FirecrawlService.ScrapePage`、`ContentCompletionService.*`、`Router.Chat`），但**没有脚本化、没有接构建**——新增方法不会自动有 span，全靠人记。
- 其余业务方法靠手写 span（`workflow.daily_report.*`、`scheduler.*`）。

织入器在**构建前**扫描 service 包源码，自动给符合条件的方法注入 span。新增方法只要符合规则，跑一次 `make instrument` 就有 span，不再依赖人。

## 织入规则（什么方法会被注入）

| 规则 | 说明 |
| ------ | ------ |
| **目标范围** | `internal/*/service/**/*.go` + `internal/platform/airouter/*.go`（`tagmanagement/service/board` 故意排除，不在本期范围）|
| **签名硬约束** | 首参必须是命名 `ctx context.Context`；无 ctx / ctx 非首参 / `_ context.Context`（丢弃）→ **跳过** |
| **可见性** | 只织入 exported（方法名/函数名首字母大写）|
| **span 命名** | receiver 方法 → `TypeName.MethodName`（解指针/泛型包装，取裸类型名）；包级函数 → `pkg.FuncName` |
| **named err** | 返回值列表含 named `err error` → 额外注入 `defer func(){ if err != nil { span.SetStatus(Error); span.RecordError(err) } }()` |
| **ctx 传播** | 方法体引用了 `ctx` → `ctx, span := ...`（把带 span 的 ctx 传给下游）；方法体没用 `ctx` → `_, span := ...`（避免 `ineffassign`）|
| **排除（幂等 + 共存）** | 方法体已含 `otel.Tracer(`/`tracer.Start(`/`tracing.Tracer(` → **跳过**，不重复注入 |

排除规则是关键：它同时保证**幂等**（织入器对同一文件跑两次不重复注入）和**与手写 span 共存**（手写的 `workflow.*`、历史 go-instrument 方法不会被二次注入）。

## 怎么用

### 本地：改完 service 方法后跑一次

```bash
cd backend-go
make instrument          # 或直接：go run ./cmd/instrumenter
```

织入器会打印改了哪些文件（`instrumented: backend-go/internal/...`），幂等——重复跑无 diff。

### CI：构建前必跑（幂等校验）

```bash
make instrument-check    # 织入后再跑必须无 diff，有未织入代码则失败
```

`instrument-check` 的逻辑：跑一次织入器，然后检查 `internal/` 有没有 git diff。如果织入器产生了 diff，说明源码里有"该织入却没织入"的方法（之前没跑全），CI 失败，提示开发者本地跑 `make instrument` 后提交。这保证了提交到主干的 service 源码**总是已织入状态**。

### 开发者工作流

1. 在 service 包新增方法 `func (s *Foo) Bar(ctx context.Context, id uint) (err error) { ... }`
2. 跑 `make instrument`
3. `git add` 织入后的源码（注入的 span 代码是要提交的，不是生成物）
4. 现在 `Bar` 自动有 `Foo.Bar` span，调用时 `otel_spans` 会记录

> 注入的 span 代码**直接写进源文件并提交**（不是 build-time 生成物）。这是有意为之：源码即真相，IDE 能看到、调试器能断点、`go build` 不依赖织入器在场。

## 产物长什么样

**注入前**：

```go
func (s *FeedService) RefreshFeed(ctx context.Context, feedID uint) (err error) {
	feed, err := s.repo.Get(feedID)
	...
}
```

**注入后**：

```go
func (s *FeedService) RefreshFeed(ctx context.Context, feedID uint) (err error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "FeedService.RefreshFeed")
	defer span.End()

	defer func() {
		if err != nil {
			span.SetStatus(otelCodes.Error, "error")
			span.RecordError(err)
		}
	}()

	feed, err := s.repo.Get(feedID)
	...
}
```

- 有 named `err` → 自动补错误记录 defer
- 方法体用了 `ctx`（本例传给下游）→ 保留 `ctx, span :=`；若方法体没用 ctx，会变成 `_, span :=`

## 与其他埋点的边界

| 层 | 谁负责 | 形态 |
| ------ | ------ | ------ |
| **DB 操作** | `GORMTracePlugin`（方向 A） | 自动，`gorm.<op>` span，零业务代码 |
| **出站 HTTP** | `httpclient` 工厂 otelhttp 包装（方向 A） | 自动，`HTTP POST/GET` span |
| **方法本身** | **本织入器** | 自动，`TypeName.Method` span |
| **HTTP 入口** | `otelgin` 中间件 | 自动，`GET /api/...` root span |
| **scheduler tick** | `tracing.TraceSchedulerTick` | 手写封装，`scheduler.<name>.cycle` |
| **业务语义 span** | 手写（`workflow.daily_report.*` 等） | 带 attributes/events 的业务 span |

织入器只管"方法本身有没有 span"，**不管业务 attributes/events**（那是独立 change 的事）。手写业务 span 不会被织入器覆盖（排除规则保护）。

## 数据库里怎么验证

注入的 span 运行时会写入 `otel_spans` 表。方法级 span 的 `name` 形如 `TypeName.Method`（首字母大写 + 点），和基础设施 span（`gorm.*`、`HTTP *`，全小写/含空格）好区分：

```sql
-- 看有哪些方法级 span、各调了多少次
SELECT name, count(*) FROM otel_spans
WHERE name ~ '[A-Z][a-zA-Z0-9]*\.[A-Z][a-zA-Z0-9]*'
GROUP BY name ORDER BY 2 DESC;

-- 看某条 trace 的方法 span 耗时（按时间排序）
SELECT name, duration_ms, status_code FROM otel_spans
WHERE trace_id = '<trace_id>' AND name LIKE '%.%'
ORDER BY start_time_unix_nano;
```

也可以走查询 API：`GET /api/traces/recent`、`GET /api/traces/:trace_id/timeline`（见 [tracing.md](tracing.md) §查询 API）。

## 排错 FAQ

**Q: 我新加的方法没被注入 span？**
检查四点：① 首参是不是命名的 `ctx context.Context`（`_ context.Context` 和 `ctx context.Context` 之外的都不算）；② 方法名首字母是不是大写（exported）；③ 方法体里是不是已经有 `otel.Tracer(`/`tracer.Start(`/`tracing.Tracer(`（有就跳过）；④ 是不是跑过 `make instrument`。

**Q: 织入器注入后，某个方法报 `ineffassign: ctx`？**
不会了。织入器会分析方法体是否引用 `ctx`：引用了就 `ctx, span :=`（传播），没引用就 `_, span :=`（避免无效赋值）。如果还报，说明方法体里对 `ctx` 的引用在织入器看不到的分支——提 issue。

**Q: 织入器会不会把我的源码改坏？**
织入器注入后会用 `go/format`（gofmt）重排整文件，保证语法正确；改完跑 `go build ./...` + 受影响包 `go test` 兜底。织入器幂等，`make instrument` 重跑可还原。如果织入器报错中断，源码可能处于半改状态——`git checkout -- internal/` 还原后重跑。

**Q: 为什么不用 `//go:generate` 散布到每个包？**
早期版本试过在方法体插 `/*line*/` 指令保调试定位，但它会重置编译器行号计数，**破坏 golangci-lint 的 `//nolint` 行号匹配**（让附近的 `//nolint:unused` 失效、误报死代码）。权衡后：调试定位收益小（gofmt 重排后行号本就变），副作用大，所以砍掉了 `/*line*/`，改用集中的 `make instrument`。`//go:generate` 散布到 7 个包的路径容易写错（各包深度不同），集中 Makefile 更稳。

## 实现要点（给改织入器的人）

源码 `backend-go/cmd/instrumenter/`：

| 文件 | 作用 |
| ------ | ------ |
| `main.go` | 入口；`defaultTargets`（7 个目标包）；`go/packages` 加载 → 逐文件 `instrumentSource` → 原地写回 |
| `instrumenter.go` | 核心织入逻辑：`instrumentSource`（parse → splice → finalize gofmt）、`analyze`（收集注入点）、`injectionBlock`（生成注入代码文本）、`hasExistingSpan`（排除/幂等）、`usesCtxParam`（ctx 传播判断）、`firstCtxParam`/`hasNamedError`/`baseTypeName`（签名分析）|
| `instrumenter_test.go` | 14 个单测：签名识别、排除、幂等、named err、import 补全、包级函数、值接收者、丢弃 ctx、ctx 传播、无 line directive |

织入用的是**字节级 splice**（在原始 src 上按 offset 插文本），不是 AST 重写后 print——这样未改动的代码字节不变，diff 最小。注入完再 `parser.ParseFile` + `astutil.AddImport` 补 import + `format.Source` gofmt。

跑单测：`cd backend-go && go test ./cmd/instrumenter/`
