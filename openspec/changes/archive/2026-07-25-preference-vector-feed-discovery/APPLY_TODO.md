# Apply 进度跟踪 — preference-vector-feed-discovery

> 主线程（本 agent）单会话执行。pi 无 subagent 工具，全在本会话完成。

## ✅ 已完成（本会话）

### step1 上下文 + step2 计划
- 读全部 change 制品 + 4 spec + 执行规范；favorite 决策：**链路存在，保留 D1 favorite=1.0 档**
- 落点：新代码全放 `internal/admin/` + `internal/models/`（零 domain 白名单风险）

### §1 数据层 ✅
- `models/discovery.go`：PreferenceVector / RSSHubRoute / RouteEmbedding / FeedRecommendation
- migrator RunAutoMigrate 注册 4 新表
- `20260725_0001` DROP user_preferences destructive migration
- database 迁移测试 green

### §2 preference-profile ✅（TDD 全 green）
- `preference_profile_service.go`：RecomputeAll（D1 权重/衰减/分桶/全局桶/幂等/不覆盖 seed）+ WriteSeed（D7/A 加权合并）+ GetProfile
- `discovery_helpers.go`：articleBehaviorLevel/timeDecay/normalizeVector/mergeSeedVectors/ComputeRecommendationHash/ParseRouteParameters + pgvector 辅助
- scheduler `job_preference_profile_update.go` + handler + routes `/preference-profile` + runtime 注册（3600s）
- 集成测试：产出向量/幂等/不覆盖 seed/种子合并 全 green

### §3 rsshub-route-catalog ✅（TDD 全 green）
- `catalog_sync_service.go`：SyncAll（/api/namespace + content_hash diff + 参数标记 + gone）+ GetStatus
- `catalog_extras.go`：CheckAvailability（D4 异步限流）+ EmbedPendingRoutes（路由向量）
- scheduler `job_rsshub_catalog_sync.go` + handler + routes `/discovery/catalog/*` + runtime 注册（每日）
- 集成测试：入库/参数标记/幂等/gone/不可达保留 全 green
- RSSHub 实例 rsshub.app 实测可达，API 结构已适配（{ns:{routes:{path:detail}}}）

### §4 feed-discovery ✅（TDD 全 green）
- `recommendation_service.go`：粗筛（pgvector <=> + route_id 状态机去重 + feeds.url/冷却）+ 精排（LLM/直出）+ 状态机（accept/dismiss）+ Ask（问答 + 种子写入）
- D5/B/C：recommendation_hash=route_id+board_id（不含 source），qa/manual 共享池
- handler + routes `/discovery/recommendations/*` + `/discovery/ask`
- 集成测试：排除 broken/accepted + accept 落地 + dismiss 冷却 全 green

### §6 后端删除 ✅
- 删 models/user_preference.go、preferences_service.go、job_preference_update.go
- preferences_handler.go 仅留 reading-behavior（采集链路保留）
- 清 wire/routes/runtime/migrator 引用 + DROP migration
- grep 验证零残留（reading-behavior 不受影响）

### 后端门禁 ✅
- golangci-lint / go vet / go build 全绿
- 影响包 test 全 green（admin/service、models、database 迁移）
- 注：admin/scheduler **全量** test FAIL 是**预存问题**（develop 基线 git stash 验证同样 FAIL，tracing panic），非本次引入；-short 单元 green

## ⏳ 剩余工作（前端 §5/§6.2 已由子线程完成并移出；以下为待办）

### §5 前端 ✅（子线程完成，门禁全绿）
- `types/discovery.ts` + `api/discovery.ts` + `api/preferenceProfile.ts`（snake→camel + id 字符串化 normalizer）
- `stores/discovery.ts`（推荐卡片/目录状态/写操作通知）+ `utils/routeParams.ts`（填参解析纯函数）
- `features/discovery/`（useDiscovery + DiscoveryPanel + DiscoveryCard + public.ts）+ `pages/discovery.vue`
- feeds 侧边栏「发现订阅源」入口（AppSidebarView）
- 设置 preferences section 换「兴趣画像」视图：`SettingsSectionPreferences.vue` 重写 + `features/settings/composables/usePreferenceProfile.ts`，SettingsWorkspace 元信息更新
- 前端门禁：lint 0 error / typecheck OK / test:unit 384 绿（新增 routeParams 14 + discovery store 10）/ build 成功
- 实测闭环（WSL 5055 后端 + Windows dev）：发现页双空态、目录同步 3097 条、画像重算 8 版块 528 标签真实渲染、问答失败优雅 toast

### §6 前端删除 ✅
- 删 ReadingPreferencesPanel.vue / useReadingPreferences.ts / stores/preferences.ts
- types/reading_behavior.ts 删 UserPreference；api/reading_behavior.ts 删 user-preferences 两方法（track/stats 保留）
- GlobalSettingsDialog.vue 删 preferences tab；useGlobalSettings activeTab 类型同步
- schedulerMeta.ts 删 preference_update 映射，补 preference_profile_update / rsshub_catalog_sync
- grep 零残留确认

### 文档 D1-D6
- flow/reading.md（偏好段重写）、scheduler.md（两新 job）、新增 discovery flow
- api/（preference-profile / discovery 端点，移除 user-preferences）
- database/（4 新表 + user_preferences 删除）
- architecture/map.md（discovery/preference-profile 域索引）、runtime.md 清理
- configuration.md（rsshub_base_url、同步间隔、推荐条数、dismiss 冷却、种子阈值）
- standard（LLM 调用点 ai-logging）

### §12 flow 变更溯源 + 归档门禁
- archive 后在受影响 flow 补变更溯源链接
- `doc-impact.sh verify` + `check-standards.sh` 对账
- 全量 `go test ./...` + `go build ./...` + `pnpm build`

### 其他
- git 提交（zanebonoalter <380207345@qq.com>）
- LLM 精排/问答依赖 airouter embedding+summary route 配置（design Open Question：复用 summary route 兜底）
