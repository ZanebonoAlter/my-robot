# Tasks — Feed Icon 状态机

> 本 change 引入 `feeds.icon_source` 字段，统一 icon 来源状态机。按 §10 数据库变更需附兼容性验证。

## 1. 后端：Feed model 新增 icon_source 字段

- [x] 1.1 `backend-go/internal/models/feed.go`：在 `Icon` 字段后新增 `IconSource string \`gorm:"size:20;default:fallback" json:"icon_source"\``
- [x] 1.2 确认 GORM AutoMigrate 会自动加列（项目用 AutoMigrate，加带 default 的列对存量行安全填 fallback）
- [x] 1.3 `feed.go` 的 `ToDict()` 方法新增 `"icon_source": f.IconSource` 输出

## 2. 后端：FetchFaviconURL 重写

- [x] 2.1 `backend-go/internal/reader/service/rss_parser.go:209`：`FetchFaviconURL` 参数从 `feedURL` 改为 `siteURL`，逻辑改为解析 siteURL 拼 `{scheme}://{host}/favicon.ico`，解析失败返回空串（不再返回 `mdi:rss`，也不再依赖 Google s2）
- [x] 2.2 新增单元测试 `rss_parser_test.go`：验证从 siteURL 拼 favicon（正常 host / 空 siteURL / 解析失败 三种情况）

## 3. 后端：RefreshFeed 选图逻辑改造

- [x] 3.1 `backend-go/internal/reader/service/feed_service.go:67-76`：重写 icon 重算条件为 `if feed.IconSource == "auto" || feed.IconSource == "fallback"`（抽为纯函数 `resolveFeedIcon` 便于单测）
- [x] 3.2 移除 `firstArticleImage` 相关代码（L59-65 的循环 + L71-72 的 case 分支），文章图不再用作 feed icon
- [x] 3.3 选图优先级：`parsed.Image` → `FetchFaviconURL(parsed.Link)`；取到则 `IconSource="auto"`，均失败则 `Icon="mdi:rss", IconSource="fallback"`
- [x] 3.4 新增/更新 `feed_service_unit_test.go`：验证 custom 不被覆盖、fallback 重算、auto 可重复刷新、文章图不被使用

## 4. 后端：CreateFeed / UpdateFeed / OPML 写入 icon_source

- [x] 4.1 `backend-go/internal/reader/handler/feed_handler.go:237-239`：CreateFeed 兜底时同时设 `Icon="mdi:rss", IconSource="fallback"`；若 req.Icon 非空则 `IconSource="custom"`
- [x] 4.2 `feed_handler.go:333-335`：UpdateFeed 接受非空 icon 时，updates 同时设 `icon_source="custom"`
- [x] 4.3 `backend-go/internal/reader/handler/opml.go:138`：OPML 导入置 `Icon="mdi:rss", IconSource="fallback"`

## 5. 后端：存量数据迁移

- [x] 5.1 版本化迁移 `postgres_migrations.go` `20260617_0001`（按项目惯例写进 `postgresMigrations()`，启动时自动执行）：
  ```sql
  -- 用 IS DISTINCT FROM 守卫保证幂等（default fallback 填充后首次分类）
  UPDATE feeds SET icon_source = CASE ... END
  WHERE icon_source IS DISTINCT FROM (CASE ... END);
  UPDATE feeds SET icon = 'mdi:rss' WHERE icon_source = 'fallback' AND icon IN ('', 'rss');
  ```
- [x] 5.2 在本地 docker postgres 执行迁移，验证存量行 icon_source 正确分布（23 auto / 6 fallback）

## 6. 前端：FeedIcon 降级修复

- [x] 6.1 `front/app/components/feed/FeedIcon.vue`：`@error` 改为切换响应式状态 `imgFailed`，失败时渲染 `mdi:rss` Icon 组件（不再 display:none 留白）
- [x] 6.2 移除 `FeedIcon.vue:47-57` 的 `getFaviconFromUrl` 及 `fallbackIcon` 中基于 articleLink/feedId 拼 favicon 的分支（favicon 获取归后端状态机）
- [x] 6.3 `front/app/types/feed.ts`：Feed 类型新增 `icon_source?: 'auto' | 'custom' | 'fallback'`
- [x] 6.4 新增/更新 `FeedIcon` 组件单测：验证 URL 加载失败降级到 mdi:rss

## 7. 前端：侦探墙补渲染 feedIcon

- [x] 7.1 `front/app/features/topic-graph/components/TimelineItem.vue:94`：文章项补渲染 `<FeedIcon :icon="article.feedIcon" :size="12" />`（feedName 同行紧凑布局）
- [x] 7.2 `front/app/features/topic-graph/components/TopicGraphArticleCard.vue:37`：同理补渲染 feedIcon（ArticleCard interface 加 feedIcon?）
- [x] 7.3 `front/app/features/topic-graph/composables/useTopicTimeline.ts:204`：修正 `feedIcon: article.image_url`（文章图错位）→ 改用后端返回的 `feed_icon` 字段
- [x] 7.4 后端补字段：`types.go` TopicArticleCard 加 FeedIcon + `repository.go:559` 赋值 `card.FeedIcon = article.Feed.Icon`（getTopicArticles 接口原未返回 feed_icon，已补）+ `topicGraph.ts` TopicArticlesResponse 加 feed_icon

## 8. 数据兼容性验证（§10）

- [x] 8.1 确认 GORM AutoMigrate 加 `icon_source` 列对存量行无破坏（default fallback 自动填充，本地 docker 验证 29 行全部填充）
- [x] 8.2 确认 Feed API JSON 响应新增 `icon_source` 字段不破坏前端现有解析（前端 `icon_source?` 可选字段，types 已更新）
- [x] 8.3 确认迁移 SQL 幂等（IS DISTINCT FROM 守卫，本地重复执行 UPDATE 0 验证）
- [x] 8.4 确认回滚路径：`ALTER TABLE feeds DROP COLUMN icon_source` 可安全回滚

## 9. 测试

> 归档前重跑本节全部命令，确认零失败（§11.4）。

### 9.1 后端单测
- [x] 9.1.1 `cd backend-go && go test ./internal/reader/service/...`（FetchFaviconURL + resolveFeedIcon 状态机 + buildArticleFromEntry）→ PASS
- [x] 9.1.2 `cd backend-go && go test ./internal/reader/handler/...`（CreateFeed/UpdateFeed icon_source 写入）→ PASS

### 9.2 前端单测
- [x] 9.2.1 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`（FeedIcon 降级逻辑）→ 18 files / 95 tests passed

## 10. 文档

- [x] 10.1 更新 `docs/reference/architecture/backend.md`：新增 Feed Icon 状态机说明（三态流转图 + favicon 获取策略 + resolveFeedIcon）
- [x] 10.2 更新 `docs/reference/database/DATABASE_FIELDS.md`：feeds 表新增 `icon_source` 列说明（类型、default、取值范围）
- [x] 10.3 更新 `docs/reference/api/feeds.md`：POST/PUT 的 icon 字段说明 + icon_source 语义

## 11. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。前端编译类命令必须通过 Windows cmd 执行。

- [x] 11.1 `cd backend-go && go vet ./internal/reader/... ./internal/models/... ./internal/platform/database/... ./internal/topicgraph/...` → exit 0
- [x] 11.2 `cd backend-go && go build ./...` → exit 0
- [x] 11.3 `cd backend-go && golangci-lint run ./internal/reader/... ./internal/models/...` → 3 issues（均为既存：opml.go:165 errcheck / opml.go:161 + semantic_label.go:6 gofmt，非本次引入行；本次改动行 0 issue）
- [x] 11.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（0 errors，23 warnings 均为既存 no-explicit-any / unused-vars）
- [x] 11.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0
- [x] 11.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 18 files / 95 tests passed
- [x] 11.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → ✨ Build complete!
- [x] 11.8 `docker exec syntopica-postgres psql ... "SELECT icon_source, COUNT(*) FROM feeds GROUP BY icon_source"` → auto: 23 / fallback: 6（无 NULL/空串）
- [x] 11.9 `openspec instructions apply --change feed-icon-state-machine --json` → 见下方执行结果
