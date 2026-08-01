## Why

订阅源发现推荐需填参的 RSSHub 路由（如 `/realtime/:category?`）时，前端只给一个裸输入框 + 目录自带的一句中文说明（"類別，見下表，默認為首頁"），用户不知道"下表"有哪些值、该填什么——推荐即弃、订阅失败。

根因：RSSHub 目录 `/api/namespace` 的 `parameters` 只是「参数名 → 中文说明」的半成品，**可选值枚举在官方文档页、不在目录数据里**；当前前端既无可选值数据、也无文档链接兜底。已归档的 `preference-vector-feed-discovery` 交付了发现主体（向量粗筛 + LLM 精排 + 卡片状态机），参数配置体验是缺口。

## What Changes

- **新增「路由参数可选值字典」数据源**：DB 表 `route_param_options`（`route_id` + `param_name` + `value` + `label` + `source`），人工维护高频路由可选值；`source`（manual/scraped）为未来文档抓取自动写入预留
- **推荐卡片 API 附带字典**：`getRecommendations` 返回的 route 对象带 `param_options`（按 `param_name` 分组），前端一次拿全、不二次请求
- **DiscoveryCard 填参表单分流**：有可选值的参数渲染为**下拉点选**；无可选值保持**裸输入框**兜底任意值；表单统一提供**「官方文档」链接**兜底（不管有没有可选值都给）
- **buildRouteParamSpecs 扩展**：`RouteParamSpec` 加 `options` 与 `docUrl`，纯函数 + 单测
- **管理端字典 CRUD**：首版走 admin 接口 + SQL 录入（不强求精致 UI）
- **字典 seed**：跑现有 `feed_recommendations`，取 `RequiresParameters=true` 且被推荐/接受过的路由 Top-N，人工录值入库
- **铁律**：参数可选值**只来自字典（人工/scraped 真实数据），绝不由 LLM 生成**；LLM 边界 = 仅推荐路由

## Capabilities

### New Capabilities
- `feed-discovery`: 订阅源发现推荐卡片的参数配置体验——参数可选值字典数据源、卡片填参点选/输入分流、官方文档链接兜底、LLM 参数值生成禁令。（已归档 `preference-vector-feed-discovery` 交付发现主体；本 capability 聚焦参数配置子能力）

### Modified Capabilities
<!-- 无：preference-vector-feed-discovery 已归档且未 sync 到 specs/，本次以新聚焦 capability 承载参数配置 requirements -->

## Impact

- **后端**：
  - `internal/models/`：新增 `RouteParamOption` model（`route_param_options` 表）+ `RSSHubRoute` 关联
  - `internal/admin/service/`：字典查询（按 route_id 批量取可选值，注入 recommendation 响应）
  - `internal/admin/handler/` + `routes.go`：字典 CRUD（admin）+ recommendation 响应扩展 `param_options`
  - migration：新表
- **前端**：
  - `app/utils/routeParams.ts`：`buildRouteParamSpecs` 扩展（`options` + `docUrl`）+ 单测
  - `app/features/discovery/components/DiscoveryCard.vue`：参数区点选/输入分流 + 官方文档链接
  - `app/types/discovery.ts` + `app/api/discovery.ts`：route.param_options 类型
- **文档**：
  - `docs/reference/flow/discovery.md`：参数配置交互 + 字典维护流程
  - `docs/reference/configuration.md`：字典配置项、`doc_base` 可配
- **依赖**：无新增第三方库
