## 1. 选型决策（路线②自写，已定）

- [x] 1.1 选型决策：采用**路线②（自写 `go/ast` 织入器）**。决策人：用户（2026-07-26）。理由：① 排除/幂等/范围完全自控（路线① go-instrument 排除能力不确定 + 依赖外部 CLI）；② 织入器本即 change 核心交付物，自写成本=核心资产；③ 不依赖外部 CLI 版本。路线①原型取消。
- [x] 1.2 可丢弃原型（按《开发执行规范》§3）：`go/ast` + `golang.org/x/tools/go/packages` 写最小织入器（识别 exported 方法 + 首参 ctx + 注入 span + 排除已有 span），试跑 1-2 个包验证排除 + 幂等；通过则直接演进为 §2 正式实现
  - **完成（2026-07-27）**：原型验证通过后直接演进为 `cmd/instrumenter` 正式实现（§2），不再单列原型阶段

## 2. 织入器正式实现

- [x] 2.1 按选定路线实现织入器（排除规则、span 命名 `TypeName.MethodName`、named err record、ctx 传播判断决定 `ctx, span`/`_, span`）
  - **注**：原计划的 `/*line*/` 标记在实现中**砍掉**——它会重置编译器行号计数、破坏 golangci-lint 的 `//nolint` 行号匹配（让附近 `//nolint:unused` 失效误报死代码），且 gofmt 重排后调试定位收益有限。见 method-instrumenter.md 排错 FAQ
- [x] 2.2 织入器单测：签名识别、排除逻辑、幂等、named err 注入（14 个测试，含 ctx 传播判断、无 `/*line*/`）
- [x] 2.3 织入器对全目标包跑一次（22 文件织入、45 文件 unchanged），`go build ./...` + 6 个受影响包 `go test` 通过

## 3. 构建集成

- [x] 3.1 加 Makefile `instrument` / `instrument-check` target（`backend-go/Makefile`）；选 Makefile 而非散布 `//go:generate`（各包深度不同路径易错 + 早期 `/*line*/` 会破坏 nolint）
- [x] 3.2 CI 构建流程加织入步骤：`make instrument-check`（织入后必须无 diff，有未织入代码则失败）；CI pipeline 接入属部署层
- [ ] 3.3 验证：新增一个 service 方法 → 跑 `make instrument` → 自动有 span（**待端到端**：需重启后端服务跑一次业务，查 DB 确认）

## 4. 迁移与共存验证

- [x] 4.1 现有 6 个 go-instrument 方法纳入新流程统一（airouter 已纳入织入范围，`Router.Chat` 在覆盖内）；排除规则检测已有 `otel.Tracer(` 跳过，不重复注入
- [x] 4.2 手写 `workflow.*` / `scheduler.*` 保留，排除规则不重复注入（hasExistingSpan 检测 `otel.Tracer`/`tracer.Start`/`tracing.Tracer`）
- [ ] 4.3 集成测试：一个 service 方法调用链的 trace 含自动 span + 手写 span 共存正确（**待端到端**：需重启后端服务跑业务，查 trace timeline）

## 5. 测试

后端受影响包：织入器自身包 + 全 `internal/*/service/`（被改写）。

- [x] 织入器单测 → PASS（14 测试）
- [x] `go test ./internal/reader/service → PASS`（改写后行为不变）
- [x] `go test ./internal/topicgraph/service → PASS`
- [x] `go test ./internal/platform/airouter → PASS`（airouter 纳入织入范围）
- [x] 抽查 service 包 `go test` → PASS（admin/dataenrichment/tagmanagement core 一并跑过）

## 6. 文档

<!-- doc-impact: architecture -->

- [x] `docs/reference/architecture/tracing.md`：§2 改写为「方法级 AST 自动织入」+ 代码结构加 instrumenter + 下一步建议勾掉 go-instrument 项 + §历史约束节加演进引子
- [x] 新建 `docs/reference/architecture/method-instrumenter.md`：使用方式 + 完整规则 + 排错 FAQ + 实现要点（自包含专题，人和 AI 查阅）
- [x] 无 flow 影响（可观测层不改业务流），按《开发执行规范》§12.2 豁免溯源

## 7. 验证

- [x] `cd backend-go && go vet ./... → 零警告`
- [x] `cd backend-go && golangci-lint run ./... → 0 issues`
- [x] 织入器单测 → PASS（14 测试）
- [x] `cd backend-go && go build ./... → 成功`
- [x] `cd backend-go && make instrument-check → 幂等（织入后再跑无 diff）`
- [x] `cd backend-go && go test ./internal/reader/service → PASS`
- [x] `cd backend-go && go test ./internal/topicgraph/service → PASS`
- [x] `cd backend-go && go test ./internal/platform/airouter → PASS`
- [x] grep 复核：手写 span 方法（`workflow.*`）未被重复注入（排除规则 + 实测 DB 方法级 span 仅历史 go-instrument 5 个 name，无重复）
- [ ] 端到端：调用一个自动织入的 service 方法 → `GET /api/traces/recent` 含对应 `TypeName.MethodName` span（**待端到端**：需重启后端服务跑业务）

## 8. 变更留痕

- 2026-07-26：apply 前事实核查修正（属《开发执行规范》§8 局部澄清/纠错）：
  - 方向 A（`add-otel-infra-instrumentation`）时态更正为"已落地并 archive"（proposal 背景 / design 背景 + §5）
  - 织入范围扩展至 `internal/platform/airouter/*.go`（proposal B 段 / design §2 / spec），统一覆盖 `Router.Chat`，消解原 4.1 与 service 范围的矛盾
  - Why 段现状盘点修正：daily_report 7→8 处；删除幽灵类型 `SummaryQueue`（实际 `TagQueue`）；article_tagging span 归属澄清；scheduler 改述为 `TraceSchedulerTick` 封装入口
  - §下一步建议引用条号 3→2
  - 补量化基线（service 层 ~84/155 方法符合 ctx 首参约束）+ admin/dataenrichment 首次注入风险点名 + 排除规则对封装 helper 的理论盲区说明
  - 修正章节引用错误：《开发执行规范》§104 → §3（原型要求实际在 §3「复杂设计验证」）
  - 选型定型：用户拍板路线②（自写 go/ast），路线①原型取消（design §1 + tasks §1 同步）
- 2026-07-27：实现收尾 + bug 修复（验证变更是否生效时发现并修复）：
  - bug1：`/*line*/` 指令重置编译器行号计数，破坏 golangci-lint 的 `//nolint` 行号匹配（让 airouter 的 `//nolint:unused` 失效、误报 `stripThinkTags` 死代码）→ 砍掉 `/*line*/` 生成（调试定位收益小、副作用大）
  - bug2：注入的 `ctx, span := ...` 在不传播 ctx 的方法里 `ineffassign` → 加 `usesCtxParam` 分析方法体是否引用 ctx，决定 `ctx, span`/`_, span`
  - 实跑织入器覆盖 22 文件（admin/dataenrichment/reader/tagmanagement/topicgraph service + airouter），全量 lint 0 issues、6 受影响包 test PASS
  - 构建集成：`backend-go/Makefile`（`instrument`/`instrument-check` target）
  - 文档：新建 `architecture/method-instrumenter.md` + 更新 `tracing.md` §2/代码结构/下一步建议/§历史约束
