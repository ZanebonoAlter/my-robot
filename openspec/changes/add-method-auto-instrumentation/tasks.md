## 1. 选型决策（路线②自写，已定）

- [x] 1.1 选型决策：采用**路线②（自写 `go/ast` 织入器）**。决策人：用户（2026-07-26）。理由：① 排除/幂等/范围完全自控（路线① go-instrument 排除能力不确定 + 依赖外部 CLI）；② 织入器本即 change 核心交付物，自写成本=核心资产；③ 不依赖外部 CLI 版本。路线①原型取消。
- [ ] 1.2 可丢弃原型（按《开发执行规范》§3）：`go/ast` + `golang.org/x/tools/go/packages` 写最小织入器（识别 exported 方法 + 首参 ctx + 注入 span + 排除已有 span），试跑 1-2 个包验证排除 + 幂等；通过则直接演进为 §2 正式实现

## 2. 织入器正式实现

- [ ] 2.1 按选定路线实现织入器（排除规则、span 命名 `TypeName.MethodName`、named err record、`/*line*/` 标记）
- [ ] 2.2 织入器单测：签名识别、排除逻辑、幂等、named err 注入
- [ ] 2.3 织入器对全目标包跑一次，`go build ./...` + 受影响包 `go test` 通过

## 3. 构建集成

- [ ] 3.1 加 `//go:generate` 指令（或 Makefile `instrument` target）
- [ ] 3.2 CI 构建流程加 `go generate ./...` 必跑步骤
- [ ] 3.3 验证：新增一个 service 方法 → 跑 generate → 自动有 span

## 4. 迁移与共存验证

- [ ] 4.1 现有 6 个 go-instrument 方法纳入新流程统一（airouter 已纳入织入范围，`Router.Chat` 在覆盖内）
- [ ] 4.2 手写 `workflow.*` / `scheduler.*` 保留，验证排除规则不重复注入
- [ ] 4.3 集成测试：一个 service 方法调用链的 trace 含自动 span + 手写 span 共存正确

## 5. 测试

后端受影响包：织入器自身包 + 全 `internal/*/service/`（被改写）。

- [ ] 织入器单测 → PASS
- [ ] `go test ./internal/reader/service → PASS`（改写后行为不变）
- [ ] `go test ./internal/topicgraph/service → PASS`
- [ ] `go test ./internal/platform/airouter → PASS`（airouter 纳入织入范围）
- [ ] 抽查 2-3 个 service 包 `go test` → PASS

## 6. 文档

<!-- doc-impact: architecture -->

- [ ] `docs/reference/architecture/tracing.md`：新增「方法级 AST 自动织入」分层描述 + 构建集成说明；更新「下一步建议」（勾掉 go-instrument 脚本化项）
- [ ] `docs/reference/standard/backend/`：若新增构建步骤（go generate），补开发 / 构建规范
- [ ] 无 flow 影响（可观测层不改业务流），按《开发执行规范》§12.2 豁免溯源

## 7. 验证

- [ ] `cd backend-go && go vet ./... → 零警告`
- [ ] `cd backend-go && golangci-lint run ./... → 零失败`
- [ ] 织入器单测 → PASS
- [ ] `cd backend-go && go build ./... → 成功`
- [ ] `cd backend-go && go generate ./... → 成功且幂等（再跑一次无 diff）`
- [ ] `cd backend-go && go test ./internal/reader/service → PASS`
- [ ] `cd backend-go && go test ./internal/topicgraph/service → PASS`
- [ ] `cd backend-go && go test ./internal/platform/airouter → PASS`
- [ ] grep 复核：手写 span 方法（`workflow.*`）未被重复注入 → `grep -rn 'tracer.Start\|otel.Tracer' <手写方法文件>` 命中数不变
- [ ] 端到端：调用一个自动织入的 service 方法 → `GET /api/traces/recent` 含对应 `TypeName.MethodName` span

## 8. 变更留痕

- 2026-07-26：apply 前事实核查修正（属《开发执行规范》§8 局部澄清/纠错）：
  - 方向 A（`add-otel-infra-instrumentation`）时态更正为"已落地并 archive"（proposal 背景 / design 背景 + §5）
  - 织入范围扩展至 `internal/platform/airouter/*.go`（proposal B 段 / design §2 / spec），统一覆盖 `Router.Chat`，消解原 4.1 与 service 范围的矛盾
  - Why 段现状盘点修正：daily_report 7→8 处；删除幽灵类型 `SummaryQueue`（实际 `TagQueue`）；article_tagging span 归属澄清；scheduler 改述为 `TraceSchedulerTick` 封装入口
  - §下一步建议引用条号 3→2
  - 补量化基线（service 层 ~84/155 方法符合 ctx 首参约束）+ admin/dataenrichment 首次注入风险点名 + 排除规则对封装 helper 的理论盲区说明
  - 修正章节引用错误：《开发执行规范》§104 → §3（原型要求实际在 §3「复杂设计验证」）
  - 选型定型：用户拍板路线②（自写 go/ast），路线①原型取消（design §1 + tasks §1 同步）
