# Spec: rsshub-route-catalog

## ADDED Requirements

### Requirement: 路由目录同步

系统 SHALL 从配置的 RSSHub 实例（`rsshub_base_url`）的 `/api/namespace` 端点同步全量路由元数据（namespace、path、name、url、description、parameters、example）入 `rsshub_routes` 表。同步 MUST 按 `content_hash` diff：新增/变更入库，目录中消失的路由标记 `gone`（不物理删除）。同步由 scheduler job（默认每日）和手动端点 `POST /api/discovery/catalog/sync` 触发，两条路径等效；同步失败 MUST 保留既有目录且仅记日志。

#### Scenario: 首次全量同步

- **WHEN** 对空 `rsshub_routes` 表执行同步
- **THEN** 实例返回的全部路由入库（实测约 3245 条）
- **AND** 每条记录保存 name/description/parameters/example 元数据

#### Scenario: 增量同步幂等

- **GIVEN** 目录内容无变化
- **WHEN** 再次执行同步
- **THEN** 不产生新行、不更新 `content_hash` 未变的行

#### Scenario: 实例不可达

- **WHEN** 同步时 RSSHub 实例不可达
- **THEN** 同步 job 记日志并正常结束
- **AND** 既有目录数据保持不变

### Requirement: 参数需求标记

系统 SHALL 在路由入库时解析 path 中的参数段：存在不带 `?` 的必填参数标记 `requires_parameters=true`，无参数或全部参数可选标记 `usable_directly=true`。两个标记 MUST 持久化为数据库字段，供推荐与前端分流使用。

#### Scenario: 零参数路由可直接订阅

- **WHEN** 入库路由 path 为 `/36kr/newsflashes`（无参数段）
- **THEN** 该行 `usable_directly=true` 且 `requires_parameters=false`

#### Scenario: 必填参数路由需用户填写

- **WHEN** 入库路由 path 为 `/bilibili/user/dynamic/:uid`
- **THEN** 该行 `requires_parameters=true` 且 `usable_directly=false`

### Requirement: 可用性校验

系统 SHALL 对带 example 的路由以可配速率（默认 2 req/s）异步发起 GET 校验：成功返回且含条目标记 `status=ok`，超时/非 200/空条目标记 `status=broken`，无 example 保持 `unknown`。校验 MUST 为后台异步，不阻塞同步主流程。

#### Scenario: example 可达标记 ok

- **GIVEN** 路由带 example 且该路径在实例上可正常返回条目
- **WHEN** 可用性校验执行到该路由
- **THEN** `status` 更新为 `ok` 并记录 `last_checked_at`

#### Scenario: broken 路由被推荐排除

- **GIVEN** 路由 `status=broken`
- **WHEN** 推荐粗筛执行
- **THEN** 该路由不出现在候选集中

### Requirement: 路由向量生成

系统 SHALL 为目录路由生成 embedding（文本取 namespace+name+description 摘要），存 `route_embeddings` 表（含 `dimension`/`model`/`text_hash`），使用与 `embedding_config` 配置一致的模型。同步发现新路由或 `text_hash` 变化时 MUST 入队生成/重算；向量维度或模型与当前配置不一致时 MUST 拒绝参与粗筛并提示重建。

#### Scenario: 新路由自动向量化

- **WHEN** 同步入库一批新路由
- **THEN** 这些路由进入 embedding 生成队列
- **AND** 完成后 `route_embeddings` 存在对应行且维度与当前配置一致
