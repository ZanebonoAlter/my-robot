## ADDED Requirements

### Requirement: 业务包自注册路由
每个 `internal/domain/<name>/` 业务包 SHALL 提供一个 `RegisterRoutes(rg *gin.RouterGroup)` 函数，负责将该包的 HTTP 端点挂载到传入的路由组上。

#### Scenario: Tagging 包注册路由
- **WHEN** `tagging.RegisterRoutes(api.Group("/tagging"))`
- **THEN** 所有 `/api/tagging/*` 下的端点在 tagging 包内部注册完成

#### Scenario: 子路由挂载
- **WHEN** tagging 包需要挂载子模块路由（如 analysis）
- **THEN** tagging 的 `RegisterRoutes` 内部调用 `tagginganalysis.RegisterAnalysisRoutes(rg, service)`，由子模块自行决定挂载点

### Requirement: router.go 瘦身
`internal/app/router.go` 的 `SetupRoutes()` SHALL 不再包含具体路由路径和 HTTP 方法，仅负责创建路由组并委托各业务包注册。

#### Scenario: 新增业务包
- **WHEN** 新增一个 domain 包
- **THEN** 只需在 `router.go` 中加一行 `xxxdomain.RegisterRoutes(api.Group("/xxx"))`，无需手写路由

#### Scenario: router.go 编译
- **WHEN** `router.go` 瘦身后
- **THEN** 其 import 列表从 14 个 domain 包减少，函数体从 134 行缩减至约 30 行

### Requirement: 路径前缀控制权保留
路由组的前缀（如 `/api/articles`）SHALL 仍在 `router.go` 中创建——各业务包的 `RegisterRoutes` 不应自行决定根路径前缀。

#### Scenario: 需要修改路径前缀
- **WHEN** `/api/feeds` 需要改为 `/api/sources`
- **THEN** 只需在 `router.go` 改一行 `api.Group("/sources")`，feed 包内部代码不受影响
