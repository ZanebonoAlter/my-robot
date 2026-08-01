## Context

订阅源发现（已归档 `preference-vector-feed-discovery` 交付主体）推荐 RSSHub 路由时，需填参的路由（`RequiresParameters=true`，如 `/realtime/:category?`）体验有缺口：

- **数据层已齐全**：`rsshub_routes.parameters`（jsonb）已存目录自带的参数说明；前端 `buildRouteParamSpecs(routePath, parameters)`（`app/utils/routeParams.ts`，纯函数+单测）已解析出 `{name, required, description}`；DiscoveryCard 已渲染填参表单；`store.accept(id, {categoryId, parameters})` 已支持参数传后端。
- **缺口**：目录 `parameters` 只是「参数名 → 中文说明」半成品（实测 `{"category":"類別，見下表，默認為首頁"}`），**可选值枚举在 RSSHub 官方文档页、不在目录数据**；前端既无可选值数据、也无文档链接 → 用户看到"见下表"不知填啥，推荐即弃。

约束：单用户系统、无鉴权；后端 Go/Gin/GORM + PostgreSQL；前端 Nuxt4/Vue3；复用 `aisettings`/`rsshub_routes` 既有体系，不引入新依赖。

## Goals / Non-Goals

**Goals:**
- 用户在推荐卡片填参时：高频参数能**直接点选**已知可选值（下拉），冷门/缺数据能**跳官方文档**查全表，不再瞎填
- 可选值数据**真实可信**：只来自人工/抓取字典，**LLM 绝不生成参数值**
- 改造**最小侵入**：复用现有 `buildRouteParamSpecs` + DiscoveryCard 脚手架，向后兼容（无字典数据时退化为现状）

**Non-Goals:**
- 推荐多样性 / 来源透明度（"同步内容太单一""看不懂来源"）——后置，另开讨论
- RSSHub 文档页**自动抓取**——首版不做，仅留 `source` 字段与数据模型兼容，作为后续渐进增强
- 精致的字典录入 UI——首版走 admin 接口 + SQL
- 修改 LLM 推荐链路本身（LLM 仍只负责选路由）

## Decisions

### D1 数据来源：人工字典打底 + 抓取渐进（方案3）
- **选**：DB 字典表，人工录入高频路由可选值；未来叠加文档抓取写入（`source` 区分）
- **alt**：①纯文档抓取（文档是 VitePress SPA、"见下表"表结构不统一、抓取质量参差 → 阻塞首版）②纯 LLM 生成（**禁止**：LLM 不知真实枚举会编造，踩中"不可信"）③塞 jsonb 进 `rsshub_routes`（难增量维护、难标来源）
- **why**：契合"固定配置项让用户选"、首版不被抓取风险卡住、冷门有文档兜底不致"配不了"

### D2 字典存储：单独表 `route_param_options`
- 字段：`route_id`(FK) + `param_name` + `value` + `label` + `source(manual/scraped)`，UNIQUE(route_id, param_name, value)
- **why 单独表**：可增量维护、可标来源、未来抓取直接 INSERT；alt（jsonb 列）改一条路由要重写整对象、难追溯

### D3 卡片渲染分流
- `RouteParamSpec` 加 `options?: {value,label}[]` 与 `docUrl?`
- 有 `options` → 下拉点选；无 → 裸输入框兜底任意值；表单底部统一「官方文档」按钮（不管有没有 options 都给）

### D4 文档链接规则
- `docUrl = {doc_base}/routes/{namespace}#{slug}`，`slug` 基于 path 推导
- `doc_base` 走 `aisettings` 配置（默认 `https://docs.rsshub.app`）——实测该站国内 `ERR_CONNECTION_RESET`，**可配换镜像**最稳

### D5 LLM 边界铁律
- 参数可选值**只来自字典**；LLM 职责 = 仅推荐路由。写进 spec Requirements 强制约束

### D6 高频界定（字典 seed）
- 跑现有 `feed_recommendations`，取 `RequiresParameters=true` 且被推荐/接受过的路由按命中 Top-N，人工录值；之后按新增命中增量补

### D7 API 携带：recommendation 响应附带 `param_options`
- `getRecommendations` 的 route 对象带 `param_options`（按 param_name 分组），前端一次拿全、不二次请求

## Risks / Trade-offs

- **[字典覆盖有限]** 首版只高频路由有 B（点选），冷门只有 C（文档链接） → 可接受（C 兜底不卡死），后续抓取扩覆盖
- **[docUrl 锚点不稳]** RSSHub 文档锚点 slug 可能随文档改版漂移 → `doc_base` 可配 + 链接生成为主（可达性由用户侧/镜像解决），Open Questions 待实测 slug 规则
- **[人工字典维护成本]** 需定期补 → seed 流程 + 按命中增量，低成本；且无字典路由退化为现状不影响可用
- **[accept 参数拼接]** 沿用现有 accept 把 parameters 拼最终 feed URL，需确认后端正确消费 param_options 选定值（Open Questions 验证）

## Migration Plan

- 新表 `route_param_options`（GORM AutoMigrate，纯新增、无破坏性）
- 后端 recommendation 响应加 `param_options`（**向后兼容**：旧前端忽略不报错）
- 前端 `buildRouteParamSpecs` 扩展（**向后兼容**：无 param_options 参数时退化为现状 `{name,required,description}`）
- 字典 seed：首次跑查询导入高频路由可选值（人工校对后入库）
- **回滚**：删表 + 前端退回（全向后兼容设计，回滚安全）

## Open Questions

1. **RSSHub 文档锚点 slug 精确规则**：`docs.rsshub.app` 访问失败（网络），slug 推导规则待实测确认（apply 前用镜像/GitHub 源验证）
2. **accept 后端参数拼接现状**：需读 `recommendation_service` 确认 `parameters` → 最终 feed URL 的拼接逻辑，确保 param_options 选定值被正确消费（apply 步骤1 建立上下文时确认）
