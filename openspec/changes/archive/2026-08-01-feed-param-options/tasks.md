## 1. 后端数据模型

- [x] 1.1 新增 `RouteParamOption` model（`internal/models/`）：`route_id`+`param_name`+`value`+`label`+`source`，UNIQUE(route_id,param_name,value)；`RSSHubRoute` 加 `ParamOptions []RouteParamOption` 关联
- [x] 1.2 注册 AutoMigrate（确认迁移注册入口）

## 2. 后端字典 service + API

- [x] 2.1 `internal/admin/service/`：字典查询 service——按 route_id 集合批量取可选值，按 param_name 分组返回
- [x] 2.2 recommendation handler 响应扩展：route 对象附 `param_options`（空字典时为空集合，向后兼容）
- [x] 2.3 doc_base 配置：`aisettings` 加 `rsshub_doc_base` 键（默认 `https://docs.rsshub.app`）+ 读取 helper
- [x] 2.4 字典 CRUD admin handler + routes（首版最小 CRUD，`/api/admin/route-param-options`）

## 3. 后端测试（TDD）

- [x] 3.1 字典 service 单测：批量查、按 param_name 分组、空集合兜底
- [x] 3.2 recommendation handler 测试：响应含 param_options（有字典/无字典两路径）

## 4. 前端类型 + 工具扩展

- [x] 4.1 `app/types/discovery.ts`：route 对象加 `param_options` 类型（按 param_name 分组）
- [x] 4.2 `app/utils/routeParams.ts`：`RouteParamSpec` 加 `options?`+`docUrl?`；`buildRouteParamSpecs(path, parameters, paramOptions?, docUrl?)` 扩展，无新入参时退化为现状
- [x] 4.3 `routeParams` 单测扩展：有 options/无 options/向后兼容/docUrl 生成

## 5. 前端卡片改造

- [x] 5.1 `DiscoveryCard.vue`：参数区按 spec.options 分流（下拉点选 vs 文本输入），传入 paramOptions + docUrl
- [x] 5.2 `DiscoveryCard.vue`：表单底部「官方文档」链接按钮（docUrl）
- [x] 5.3 `app/api/discovery.ts`：响应类型对齐 param_options

## 6. 字典 seed（首版数据）

- [x] 6.1 查询：跑 `feed_recommendations` 取 `RequiresParameters=true` 且被推荐/接受过的路由 Top-N
- [x] 6.2 人工录可选值入库（SQL，`source=manual`）— 版本化迁移 `20260801_0001`（qbitai/tencent/ithome×2/36kr 共 26 值，源自 RSSHub 源码 description 表格；随服务启动自动执行，tableExists 守卫 + ON CONFLICT 幂等）

## 测试

- [x] 后端单测覆盖字典 service + recommendation 响应（任务 3.x）
- [x] 前端 routeParams 单测覆盖扩展（任务 4.3）

## 文档

<!-- doc-impact: flow api configuration architecture database -->
- [x] `docs/reference/flow/discovery.md`：参数配置交互（点选/输入分流）+ 字典维护流程
- [x] `docs/reference/api/discovery.md`：recommendation 响应 `param_options` 字段 + 字典 CRUD 端点
- [x] `docs/reference/configuration.md`：`rsshub_doc_base` 配置项 + 字典说明
- [x] `docs/reference/architecture/frontend.md`（如涉及卡片描述）：DiscoveryCard 参数区分流
- [x] `docs/reference/database/{_index,DATABASE_FIELDS}.md`：`route_param_options` 表（字段/索引/计数/迁移史）

## 验证

- [x] `cd backend-go && go test ./internal/admin/service ./internal/admin/handler` → 期望：PASS
- [x] `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` → 期望：0 error
- [x] `cd front && pnpm lint` → 期望：0 error（warnings 为既有）
- [x] `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 期望：无类型错误
- [x] `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 期望：全 PASS
- [x] `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 期望：Build complete
- [x] 归档前：`bash scripts/doc-impact.sh verify` + `bash scripts/check-standards.sh` → 期望：无遗漏/达标
