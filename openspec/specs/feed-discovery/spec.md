# feed-discovery Specification

## Purpose
TBD - created by archiving change feed-param-options. Update Purpose after archive.
## Requirements
### Requirement: Route Parameter Options Dictionary
系统 SHALL 维护「路由参数可选值字典」（`route_param_options` 表），按 `route_id` + `param_name` 组织一个参数的多个可选值（每行 `value` + `label`）。可选值 SHALL 只来自人工录入（`source=manual`）或文档抓取（`source=scraped`）的真实数据，MUST NOT 由 LLM 生成。除字典外，RSSHub 目录 `parameters` 自带的 `options` 数组（部分路由如 ifanr/jrj 已含枚举值）也是真实数据源——前端可直接消费，不违背 LLM 禁令；字典为人工/scraped 维护，与目录 options 互补（字典优先）。

#### Scenario: 字典命中时卡片提供点选
- **WHEN** 推荐路由的某参数在字典中存在可选值
- **THEN** 填参表单该参数渲染为下拉点选，选项来自字典（value+label）

#### Scenario: 字典未命中时退化为输入框
- **WHEN** 推荐路由的某参数在字典中无可选值
- **THEN** 填参表单该参数渲染为文本输入框（兜底任意值），不阻塞订阅

#### Scenario: 字典来源可追溯
- **WHEN** 字典记录被查询
- **THEN** 每条可选值携带 `source`（manual/scraped），无 `llm` 来源

### Requirement: Recommendation API Carries Param Options
`getRecommendations` 响应的 route 对象 SHALL 附带 `param_options`（按 `param_name` 分组的可选值数组），使前端一次拿全、无需二次请求。

#### Scenario: 响应包含 param_options
- **WHEN** 客户端请求推荐列表
- **THEN** 每条 route 对象含 `param_options` 字段（无字典数据时为空集合）

### Requirement: Recommendation Card Parameter Rendering
推荐卡片填参表单 SHALL 按参数规格分流渲染：有 `options` → 下拉点选；无 `options` → 文本输入框。`buildRouteParamSpecs` SHALL 输出 `{name, required, description, options?, docUrl?}`。`options` 来源优先级：字典（`param_options`，manual/scraped）> 目录自带 options（RSSHub 目录 `parameters` 里的枚举值）；两者都无时缺省（向后兼容，渲染输入框）。

#### Scenario: 有可选值参数渲染为下拉
- **WHEN** 参数规格含非空 `options`
- **THEN** 表单渲染下拉选择控件，选项为 options 列表

#### Scenario: 无可选值参数渲染为输入框
- **WHEN** 参数规格无 `options` 或为空
- **THEN** 表单渲染文本输入框

#### Scenario: 目录自带 options 时提供点选
- **WHEN** 某参数在 RSSHub 目录 `parameters` 自带 `options` 数组（如 ifanr `/category/:name`、jrj `/:channelNum`）且字典无该参数
- **THEN** `buildRouteParamSpecs` 用目录 options 作为 `spec.options`（下拉点选），无需字典录入

#### Scenario: 字典优先于目录 options
- **WHEN** 某参数同时有字典 options 和目录 options
- **THEN** 用字典 options（人工维护优先，目录兜底）

#### Scenario: 向后兼容无任何 options 源
- **WHEN** 调用 `buildRouteParamSpecs` 未传 `param_options` 且目录无该参数的 options
- **THEN** 输出退化为 `{name, required, description}`，不报错

### Requirement: Official Documentation Link
每条推荐路由的填参表单 SHALL 提供「官方文档」链接，URL = `{doc_base}/routes/{namespace}#{slug}`（slug 基于路由 path 推导）。`doc_base` SHALL 可经 `aisettings` 配置（默认 `https://docs.rsshub.app`），以应对官方文档站访问受限时切换镜像。

#### Scenario: 表单提供文档链接
- **WHEN** 用户打开任意推荐路由的填参表单
- **THEN** 表单底部显示「官方文档」按钮，链接指向该路由的文档页

#### Scenario: doc_base 可配置
- **WHEN** 管理员修改 `aisettings` 中的 `doc_base`
- **THEN** 后续文档链接使用新 base 生成

### Requirement: LLM Parameter Value Prohibition
系统 MUST NOT 使用 LLM 生成参数可选值。LLM 在订阅源发现链路中的职责 SHALL 限定为「推荐路由」（向量粗筛 + 精排）；参数可选值 SHALL 只来自字典真实数据。

#### Scenario: LLM 不产出参数值
- **WHEN** 推荐链路运行
- **THEN** 字典中所有可选值的 `source` 仅 `manual` 或 `scraped`，不存在 `llm` 来源记录

