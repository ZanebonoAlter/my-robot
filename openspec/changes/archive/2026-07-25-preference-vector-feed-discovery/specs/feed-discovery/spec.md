# Spec: feed-discovery

## ADDED Requirements

### Requirement: 两段式推荐生成

系统 SHALL 按「向量粗筛 → LLM 精排」生成订阅源推荐：粗筛以各版块偏好向量对 `route_embeddings` 做余弦相似度 top-N（默认每版块 8 条，可配），排除 `status=broken`、**已接受或已 dismiss 的 route（按 route_id 维度状态机去重，`requires_parameters` 路由无需预判最终 url）**、`usable_directly` 路由额外按 `feeds.url` 去重、dismiss 冷却期内的路由；精排将候选元数据与版块画像摘要交 LLM 产出保留子集和每条一句推荐理由。推荐 MUST 以 `recommendation_hash`（route_id+board_id）幂等落 `feed_recommendations` 表，同 hash 已有 pending 行不重复入库。**`recommendation_hash` 不含 source：`qa` 与 `manual_refresh` 共享同一幂等池与 dismiss 冷却池**——同一条源（route+board）无论由问答还是刷新首次产出，都只占一个 pending 坑；dismiss 一次即对该 route+board 跨所有 source 冷却。此为预期去重语义。推荐生成 MUST 由手动刷新触发（`POST /api/discovery/recommendations/refresh`），不做自动定时推送。

#### Scenario: 手动刷新产出推荐卡片

- **GIVEN** 存在偏好向量与已向量化路由目录
- **WHEN** 用户调用 `POST /api/discovery/recommendations/refresh`
- **THEN** 系统按版块粗筛并经 LLM 精排后落库推荐
- **AND** 每条推荐含路由信息、所属版块、相似度与 LLM 推荐理由

#### Scenario: 刷新幂等

- **GIVEN** 某路由已在 pending 推荐中
- **WHEN** 再次刷新且该路由仍被选中
- **THEN** 不产生重复 pending 行

#### Scenario: qa 与手动刷新共享幂等池

- **GIVEN** 某路由+版块组合已被问答推荐（`source=qa`，pending）
- **WHEN** 用户手动刷新推荐且该组合仍被粗筛选中
- **THEN** 不产生新的 `source=manual_refresh` 行（同 hash 已占坑）

#### Scenario: dismiss 跨 source 冷却

- **GIVEN** 用户 dismiss 了某 `source=qa` 推荐的 route+board
- **WHEN** 冷却期内手动刷新再次选中该 route+board
- **THEN** 该组合不再入库（跨 source 冷却生效）

#### Scenario: 已订阅路由不再推荐

- **GIVEN** 某路由对应 URL 已存在于 `feeds`
- **WHEN** 刷新推荐
- **THEN** 该路由不出现在新推荐中

### Requirement: 推荐卡片状态机

推荐卡片 SHALL 具备状态机 `pending → accepted | dismissed`。接受（`POST /api/discovery/recommendations/:id/accept`）MUST 完成订阅落地后标记 `accepted` 并记录 `accepted_feed_id`；拒绝（`POST .../dismiss`）标记 `dismissed` 并进入冷却期（默认 30 天，可配），冷却期内同 hash 不得再入库。

#### Scenario: 接受零参数推荐一键订阅

- **GIVEN** 一条 `usable_directly` 的 pending 推荐
- **WHEN** 用户接受（可选指定分类）
- **THEN** 系统用实例地址拼接路由路径创建 feed
- **AND** 推荐标记 `accepted` 并关联新 feed id

#### Scenario: 接受带参数推荐需用户填参

- **GIVEN** 一条 `requires_parameters` 的 pending 推荐
- **WHEN** 用户接受
- **THEN** 系统提示填写必填参数（展示目录自带的中文参数说明）
- **AND** 用户提交参数后先经 feed fetch 验证，验证通过才创建 feed

#### Scenario: dismiss 冷却防重现

- **GIVEN** 用户 dismiss 了某推荐
- **WHEN** 冷却期内再次刷新推荐
- **THEN** 同 hash 推荐不再出现

### Requirement: 问答式即时推荐

系统 SHALL 提供 `POST /api/discovery/ask`：将问题文本 embedding 后对 `route_embeddings` 粗筛、LLM 精排，即时返回推荐列表；同时以 `source=qa` 将推荐落 `feed_recommendations` 表（接受/拒绝走同一状态机），并按 preference-profile 的问答种子要求写入种子偏好。

#### Scenario: 问答即时返回推荐

- **WHEN** 用户提问「我想看 AI 芯片相关的中文资讯」
- **THEN** 响应包含 LLM 精选的路由推荐及理由
- **AND** 这些推荐以 `source=qa` 落库，可在推荐列表中接受或拒绝

### Requirement: 发现页入口与展示

系统 SHALL 在订阅源（feeds）区域提供「发现订阅源」入口，发现页 SHALL 包含：推荐卡片流（按版块分组，标注 `usable_directly` 一键订阅 / `requires_parameters` 需填参数 / 未验证状态）、手动刷新按钮、问答输入框。卡片 MUST 提供接受与拒绝操作。

#### Scenario: 从订阅源区域进入发现页

- **WHEN** 用户在 feeds 区域点击「发现订阅源」
- **THEN** 打开发现页并加载 pending 推荐列表

#### Scenario: 无目录或无偏好时的空态引导

- **GIVEN** 目录未同步且无偏好向量
- **WHEN** 用户打开发现页
- **THEN** 页面展示空态引导（同步目录 / 通过问答建立兴趣）
- **AND** 不出现报错
