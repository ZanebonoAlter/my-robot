# feed-param-options 实现计划（主线程调度）

> OpenSpec change `feed-param-options` apply 执行计划。主线程只调度 + 核验，不写实现代码。
> 规范依据：`docs/reference/开发执行规范.md` §0.6 编排六步。

## 契约锁死（前后端并行依据）

recommendation route 对象新增 `param_options` 字段，JSON 形状固定：

```json
"param_options": {
  "category": [
    {"value": "world", "label": "国际", "source": "manual"},
    {"value": "cn", "label": "国内", "source": "manual"}
  ]
}
```

- 类型：`Record<param_name, ParamOption[]>`，`ParamOption = {value, label, source}`
- 空字典 → `{}`（空对象，非 null，向后兼容）
- `source` ∈ {`manual`, `scraped`}，**永不出现 `llm`**（D5 铁律）

`docUrl` 生成：`{doc_base}/routes/{namespace}#{slug}`，slug 基于 path 推导（去前导 `/` + 参数段，`/` → `-`）。`doc_base` 默认 `https://docs.rsshub.app`。

## 现状骨架（已建立上下文）

### 后端
- `internal/models/discovery.go`：`RSSHubRoute`(L34)、`FeedRecommendation`(L73)。新 `RouteParamOption` 加此；`RSSHubRoute` 加 `ParamOptions []RouteParamOption` 关联。
- `internal/platform/database/migrator.go` `RunAutoMigrate`(L88-93)：discovery model 在默认列表。新 model 加 `&models.RouteParamOption{}` 到此处。
- `internal/platform/aisettings/config_store.go`：仿 `LoadDailyReportTimeConfig`/`LoadBoardUpgradeSuggestTimeConfig`（简单 string + 默认值）加 `rsshubDocBaseKey` + Load/Save。
- `internal/admin/service/recommendation_service.go`：`RecommendationCard`(L328 struct) + `GetRecommendations`(L342)。card 加 `ParamOptions` 字段；GetRecommendations 批量查字典注入（一次 IN 查询，禁 N+1）。
- `internal/admin/repository/repository.go`：`AdminRepository`。字典查询方法挂此（或新建 service）。
- `internal/admin/handler/discovery_handler.go`：`GetRecommendations` handler。
- `internal/admin/routes.go`：`rg.Group("n")` discovery 路由组 + 加字典 CRUD 组。

### 前端
- `app/utils/routeParams.ts`：`RouteParamSpec{name,required,description}` + `buildRouteParamSpecs(path, rawParameters)`。加 `options?:{value,label}[]` + `docUrl?`，签名加可选入参 `paramOptions?`, `docUrl?`（无则退化）。
- `app/features/discovery/components/DiscoveryCard.vue`：`paramSpecs`(L20 computed) 调 buildRouteParamSpecs；template 参数区(L66 `v-for spec in paramSpecs` + L73 `<AppInput>`)。改：有 options→select，无→AppInput；底部加官方文档链接。
- `app/types/discovery.ts`：`DiscoveryRecommendation`(L15)。加 `paramOptions: Record<string, ParamOption[]>`。
- `app/api/discovery.ts`：`RecommendationPayload`(L13) + `normalizeCard`(L38)。加 `param_options` 透传。

## 任务派发

### 派发1 — 后端（子线程 glm-5.2，前台，worktree 隔离）
任务：1, 2, 3, 4(配置), 5(CRUD), 6.2(seed SQL), 17(后端测试), 22, 23
- [ ] T1 model + AutoMigrate
- [ ] T2 字典查询 service（批量 IN + 按 param_name 分组）
- [ ] T3 RecommendationCard 扩展 + GetRecommendations 注入
- [ ] T4 rsshub_doc_base 配置 Load/Save
- [ ] T5 字典 CRUD handler + routes
- [ ] T6 后端单测（TDD：先红后绿）
- [ ] 门禁：`go test ./internal/admin/...` + golangci-lint + vet + build

### 派发2 — 前端（子线程，后端核验后派，契约已锁死可并行）
任务：4.1, 4.2, 4.3, 5.1, 5.2, 5.3, 18, 24, 25, 26, 27
- [ ] F1 types + api 对齐 param_options
- [ ] F2 buildRouteParamSpecs 扩展（向后兼容）+ 单测
- [ ] F3 DiscoveryCard 分流渲染 + 官方文档链接
- [ ] 门禁：lint(typecheck/build/test:unit 走 cmd.exe)

### 派发3 — 文档（主线程，代码定稿后）
任务：19, 20, 21, + 新增 api 任务
- [ ] flow/discovery.md
- [ ] api/discovery.md
- [ ] configuration.md
- [ ] architecture/frontend.md

### 派发4 — seed + 验证汇总（最后）
任务：6.1, 28
- [ ] 6.1 跑 feed_recommendations 查 Top-N（需 DB 运行）
- [ ] 28 归档前：doc-impact verify + check-standards

## 进度
- 后端：pending
- 前端：pending
- 文档：pending
- 验证：pending
