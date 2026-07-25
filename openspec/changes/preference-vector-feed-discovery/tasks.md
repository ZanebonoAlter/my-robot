# Tasks: preference-vector-feed-discovery

## 1. 数据层（后端）

- [ ] 1.1 新增模型与迁移：`preference_vectors`（board_id 可空 + source + vector + tag_weights jsonb + UNIQUE(board_id, source)）、`rsshub_routes`（UNIQUE(namespace, path) + requires_parameters/usable_directly + content_hash + status）、`route_embeddings`（UNIQUE(route_id) + text_hash）、`feed_recommendations`（recommendation_hash UNIQUE + 状态字段），pgvector 列写法沿用 `topic_tag_embeddings`
- [ ] 1.2 迁移测试：四表建表/约束/索引（走既有 migrator 测试模式）

## 2. preference-profile（后端）

- [ ] 2.1 偏好聚合 service：行为权重（favorite=1.0/深读=0.6/open=0.3）× 30 天时间衰减 × 标签向量质心，按 `topic_tag_board_labels` 分桶 + 全局桶；最小标签数阈值退全局；全量重建幂等；不覆盖 `source=seed` 行
- [ ] 2.2 `preference_profile_update` scheduler job（默认 3600s，registry 模式，失败仅记日志）+ `POST /api/preference-profile/recompute` 手动触发（同一路径）
- [ ] 2.3 `GET /api/preference-profile` 画像读取（版块分组 top 标签/权重/来源/最后计算时间，空数据返回空列表）
- [ ] 2.4 种子写入 service：兴趣文本 embedding → 板块向量匹配（阈值可配，默认 0.5）→ `source=seed` **加权合并 upsert**（α 默认 0.4 可配：`normalize(α×incoming+(1−α)×existing)`，tag_weights 同步合并，保 `UNIQUE(board_id,source)` 单行）
- [ ] 2.5 单测：权重公式、分桶归属、幂等、种子不被重算覆盖、种子加权合并（多次写入累积非覆盖）、维度/模型不一致拒绝
- [ ] 2.6 【apply step1 前置】grep 确认 favorite 上报链路：文章收藏是否写 `reading_behaviors.event_type='favorite'`（`useReadingTracker` 仅 open/close/scroll）。若无上报 → 决定补收藏上报 vs 下掉 D1 favorite=1.0 档（见 design Open Questions）

## 3. rsshub-route-catalog（后端）

- [ ] 3.1 目录同步 service：`GET {rsshub_base_url}/api/namespace` 拉取 → 解析入 `rsshub_routes` → content_hash diff（新增/变更入库、消失标 gone）；`rsshub_base_url` 配置项（默认自建实例，可缺省）
- [ ] 3.2 参数标记：path 参数段解析（`:param` 必填 / `:param?` 可选）→ requires_parameters / usable_directly 入库字段
- [ ] 3.3 可用性校验 worker：example 路径异步 GET（默认 2 req/s 可配）→ ok/broken/unknown；不阻塞同步主流程
- [ ] 3.4 路由 embedding：新路由/text_hash 变更入队生成（复用 embedding 队列模式），存 `route_embeddings`
- [ ] 3.5 `rsshub_catalog_sync` scheduler job（默认每日）+ `POST /api/discovery/catalog/sync`、`GET /api/discovery/catalog/status`
- [ ] 3.6 单测：namespace 响应解析、hash diff 幂等、参数标记规则、gone 标记

## 4. feed-discovery（后端）

- [ ] 4.1 粗筛 service：pgvector `<=>` 每版块 top-N（默认 8 可配），排除 broken/**已接受/已 dismiss 的 route_id（状态机维度去重，param 路由无需猜 url）**/usable_directly 额外按 feeds.url 去重/dismiss 冷却期
- [ ] 4.2 精排 service：候选元数据 + 版块画像摘要 → LLM（走 airouter capability route，apply 时定路由）→ 保留子集 + 理由；`recommendation_hash`（route_id+board_id，**不含 source，qa/manual 共享池**）幂等落库
- [ ] 4.3 推荐 API：`GET /api/discovery/recommendations`、`POST .../refresh`、`POST .../:id/accept`（usable_directly 直订 / requires_parameters 填参 → fetch 验证 → CreateFeed）、`POST .../:id/dismiss`（冷却默认 30 天可配）
- [ ] 4.4 问答 API：`POST /api/discovery/ask` → 即时粗筛+精排返回 + `source=qa` 落库 + 调 2.4 种子写入
- [ ] 4.5 单测：粗筛排除规则（route_id 状态机去重 + usable_directly feeds.url）、hash 幂等（qa/manual 共享池）、accept 双路径（直订/填参）、dismiss 跨 source 冷却

## 5. 前端

- [ ] 5.1 feeds 区域「发现订阅源」入口 + 发现页：推荐卡片流（按版块分组、usable_directly/requires_parameters/未验证标注、接受/拒绝）、手动刷新、问答输入框
- [ ] 5.2 接受流程 UI：直订一键完成；填参表单（展示目录中文参数说明）→ 验证反馈
- [ ] 5.3 设置 `preferences` section 替换为「兴趣画像」视图（版块分组标签/权重/来源/最后计算时间 + 手动重算 + 空态引导）
- [ ] 5.4 新增 api/store/composable（discovery、preferenceProfile），类型定义

## 6. 旧偏好功能废弃

- [ ] 6.1 后端删除：`models/user_preference.go`、`admin/service/preferences_service.go`、`admin/handler/preferences_handler.go` user-preferences 部分、`admin/scheduler/job_preference_update.go`、runtime/router/routes/wire 相应注册；DROP `user_preferences` 表（迁移）
- [ ] 6.2 前端删除：`ReadingPreferencesPanel.vue`、`useReadingPreferences.ts`、`stores/preferences.ts`、`types/reading_behavior.ts` 的 `UserPreference`、`SettingsSectionPreferences.vue` 旧引用（由 5.3 接替）
- [ ] 6.3 确认 `reading_behaviors` 采集与 `/api/reading-behavior/*` 不受影响（grep 验证）

## 7. 配置 UI 扩展（apply 验收后补充，2026-07-25）

- [ ] 7.1 feed_discovery 独立 capability route：airouter 加 `CapabilityFeedDiscovery` 常量；recommendation_service 精排(rerank)改用 `CapabilityFeedDiscovery`（替代 `CapabilitySummary`，不与总结共用，便于分清用量与单独配模型）；问答 Ask 的 embedding 仍用 `CapabilityEmbedding`（通用）；前端 `AIRouterCapabilityRoutes` 能配 feed_discovery route（若前端硬编码 capability 列表则补一项）
- [ ] 7.2 RSSHub 实例配置 UI：后端 `rsshub_base_url` 从 `ai_settings` 表读（key=`rsshub_config`，仿 summary_config/firecrawl_config 模式），`catalog_sync_service`/`recommendation_service` 改读配置（缺省回落 DefaultRSSHubBaseURL）；加 `GET/POST /api/settings/rsshub` 读写 API；前端新增 `SettingsSectionRsshub.vue`（填实例地址 + 连通测试可选）+ 注册 SettingsWorkspace sidebar
- [ ] 7.3 门禁：后端 capability/config 改动单测；前端 SettingsSectionRsshub + capability route 配置 typecheck/build/test:unit（cmd.exe）

## 8. 推荐卡片 UX 打磨（2026-07-25 验收补充）

- [ ] 8.1 直订卡片分类选择：usable_directly 一键订阅也提供「订阅到分类」下拉（统一填参/直订流程，复用 DiscoveryCard 现有 categoryId + categories 数据）
- [ ] 8.2 推荐依据可视化：卡片显示相似度 score（%）+ 匹配版块 board_label（数据已有 card.score/card.boardLabel，补展示），让推荐不再是黑盒
- [ ] 8.3（待定）匹配标签依据：后端粗筛记录贡献匹配的偏好 tag，卡片展示「匹配标签：AI/芯片」——中等改动，待用户确认是否要做

## 测试

- [ ] T1 后端：`go test ./internal/<改动包>` 按 §AI Behavior Rules 只跑影响包（新增 service/handler 包 + admin + reader）
- [ ] T2 前端：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"`（新增 store/composable 单测）
- [ ] T3 集成：目录同步 → 偏好重算 → 刷新推荐 → accept/dismiss → 问答 全链路手动验证（本地 docker PG + 自建 RSSHub 实例）

## 文档

<!-- doc-impact: flow api database architecture standard configuration -->

- [ ] D1 `docs/reference/flow/reading.md`：偏好段落重写（旧分数制 → 向量画像）；scheduler.md 增两个新 job；新增/更新 discovery 相关 flow（含业务约束与不变量节）
- [ ] D2 `docs/reference/api/`：新增 preference-profile / discovery 端点，移除 user-preferences
- [ ] D3 `docs/reference/database/`：四张新表 + user_preferences 删除
- [ ] D4 `docs/reference/architecture/map.md`：新增 discovery/preference-profile 域索引；runtime.md 清理 preference_update
- [ ] D5 `docs/reference/configuration.md`：`rsshub_base_url`、同步间隔、推荐条数、dismiss 冷却、种子阈值等配置项
- [ ] D6 standard：新增 LLM 调用点（精排/问答）遵循 ai-logging 规范，确认无需改规范本身（如需新增 route 约定则补）

## 验证

- [ ] V1 `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` → 全绿
- [ ] V2 `cd backend-go && go test ./internal/<影响包>` → 全绿（按 T1 实际影响包列表）
- [ ] V3 `cd front && pnpm lint` → 零错误；`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 零错误
- [ ] V4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 成功
- [ ] V5 `grep -rn "user-preferences\|UserPreference\|preference_update" backend-go/internal front/app --include="*.go" --include="*.ts" --include="*.vue" | grep -v preference_profile | grep -v preference-profile` → 零残留（reading-behavior 不受影响）
- [ ] V6 `curl -s http://localhost:5000/api/preference-profile` → 200 且空数据返回空列表；`curl -s -X POST http://localhost:5000/api/discovery/catalog/sync` → 目录入库存量 >3000 行（`SELECT count(*) FROM rsshub_routes`）
- [ ] V7 `bash scripts/doc-impact.sh verify` + `bash scripts/check-standards.sh` → 对账零失败
