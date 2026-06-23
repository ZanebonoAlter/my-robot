## Why

后端和前端都存在模块边界模糊、重复代码分散、接口不规范的问题。后端用 `interface{}` 全局变量打破循环引用，前端四个独立 WebSocket 连接各自为政。这些不是"功能 bug"，但会让新增功能越来越慢、测试越来越难写、新人越来越懵。趁代码量还在可控范围，把架构根基踩实。

Phase 0–1 已完成（包重组 + Spring 式分层）。本轮聚焦 Phase 2+ 以及代码审查中发现的新问题。

## What Changes

### 后端

- **调度器接口化**：定义正式的 Go `Scheduler` 接口，用注册中心替代 `runtimeinfo/schedulers.go` 里 8 个 `interface{}` 全局变量。消除 `handler.go` 中 574 行类型断言分发代码。
- **运行时统一管理**：将 `Runtime` 从 9 个具名字段改为调度器列表，启动/关闭遍历列表而非逐个硬编码。加一个调度器只需创建 + 塞列表。
- **路由自注册**：每个业务包提供 `RegisterRoutes(rg)`，`router.go` 从 14 个 domain import 缩成薄薄的调用链。
- **数据访问规范化**：只 `daily_report` 有 `repository.go`，其他包 handler 直连 `database.DB`。逐步统一到至少 service 层间接访问。
- **调度器工厂模式**：引入 `BaseScheduler` 封装全部公共脚手架（生命周期、互斥执行、状态追踪、统计计数），9 个 scheduler 的重复代码从 ~3320 行缩减至 ~850 行。业务逻辑收敛为 `JobFunc` 函数，新增 scheduler 从复制 250 行变成写 ~30 行函数。分三批迁移（简单→中等→复杂）。

### 前端

- **统一实时事件流**：用一个 `useEventStream()` 组合函数管连接，其他模块订阅类型化事件。当前后端全局事件只提供 `/ws`，本轮保留 WebSocket Adapter；专用长任务继续通过命名 SSE Adapter 暴露。
- **修复 Store 绕过 API 的静默数据丢失**：`useArticlesStore` 的 `markAsRead` / `toggleFavorite` 只改本地不改后端，刷新即丢。统一所有变更必须经过 API。
- **统一错误处理**：25+ 组件各自 `error.value` / `notice.value` / `console.error`，需要一个全局 toast / 错误总线。
- **API 层归一化收口**：5 种 query-building 写法、6 套 snake_case→camelCase 映射、`tagQueue` 与 `embeddingQueue` 结构一致可共用泛型基类。
- **Store 巨型文件拆分**：`stores/api.ts` (13K) 承载所有实体 CRUD，是前端最大技术债。按功能域拆分，各 feature 的 API 调用下沉到对应模块。
- **组件健康度**：多个巨型组件远超合理阈值（>500 行），需拆解为 composable + 子组件。涉及 `TopicGraphPage.vue` (74K)、`GlobalSettingsDialog.vue` (67K)、`TagsPage.vue` (54K)、`AIRouterSettingsPanel.vue` (42K)、`ArticleContentView.vue` (39K) 等。`FeedLayoutShell.vue` 消除 7 处不必要的 `any`。

### 前端架构复盘追加范围

当前已完成一批抽取后，复盘发现仍有几类“复杂度搬家”问题，需要纳入本 change 的后续收口：

- **事件流 Seam 深化**：`useEventStream()` 必须成为唯一全局实时事件入口，修复 destroyed singleton、未退订 handler、`TagQueuePanel` 自建 WebSocket 等问题；当前以后端 `/ws` 作为 Adapter，避免再宣称已切换 SSE。
- **Store 循环知识消除**：文章写入由 `useArticlesStore` 直接持久化，不再绕 `apiStore`；`apiStore` 不再动态 import `articlesStore`。Feed unread count 通过小 Interface 同步。
- **Composable 深化**：`TopicGraphPage.vue` / `TagsPage.vue` 拆分后，不能只形成巨型 `useTopicGraph()` / `useTagsPage()`；继续按 query、selection、timeline、preview、drag 等内聚行为拆成深 Module。
- **跨 feature import 收口**：`features/tags`、`features/topic-graph` 不直接 import `features/articles` 或彼此内部组件/工具；共享 normalizer 和可复用 UI 上移到稳定 Module 或 feature facade。
- **API/通知单一入口**：查询构建、响应解包、snake_case normalizer 各只保留一个权威 Module；`useNotify()` 按 Nuxt `useState` 实现并避免重复通知。

### 后端包内组织优化（Phase 0–1 遗留问题）

- **`tagmanagement/service/` 拆分**：24 个业务文件 + 16 个测试文件 = 40 个文件堆在一个目录下，涵盖 Tagging / Embedding / Semantic Board / Tag Merge / Auxiliary Label / Extraction 六个功能域。按域拆为子包（`service/tagging/`、`service/embedding/`、`service/board/`、`service/merge/`），每个子包独立内聚。
- **`tagmanagement` 组织范式统一**：当前 `analysis/` 和 `watched/` 是垂直切片（各自有 handler + service），而 `handler/` + `service/` + `repository/` 是水平分层。两种模式混用。统一为水平分层（`analysis/` 和 `watched/` 的 handler 移入 `handler/`，service 移入 `service/`），垂直切片不再保留独立的 handler。
- **清理废弃代码**：`tagmanagement/analysis/` 后端有路由但前端零调用，确认废弃直接删除。`admin/handler/narrative_handler.go` + `admin/service/narrative_*.go` 共 8 个文件，前端唯一消费者（`NarrativeGenerateDialog.vue`）实际调用的是 Daily Report API，只是借了 Narrative 名字——一并删除，前端组件重命名。
- **`platform/ai` 与 `platform/airouter` 合并**：`platform/ai` 只有一个文件 `service.go`（222行），是 airouter 的 fallback + prompt 工具。合并到 `airouter/` 消除歧义。
- **巨型文件拆分**：`semantic_board_handler.go` (1923行)、`topicgraph/repository.go` (1114行) 需要按操作域拆分。

### 前端大组件拆分

超过 500 行（~15K）的 `.vue` 文件都应拆出 composable 管理状态 + 子组件管理 UI：

| 组件 | 大小 | 优先级 |
|------|------|--------|
| `TopicGraphPage.vue` | 74K | P0 |
| `GlobalSettingsDialog.vue` | 67K | P0 |
| `TagsPage.vue` | 54K | P0 |
| `AIRouterSettingsPanel.vue` | 42K | P1 |
| `ArticleContentView.vue` | 39K | P1 |
| `TagMergePreview.vue` | 32K | P1 |
| `TopicGraphSidebar.vue` | 29K | P1 |

### 代码卫生

- 删除未使用的依赖 (`d3-dag`, `p5`, `katex`) 和死代码 (`useWebSocketRebuild.ts` 无消费者)。
- WebSocket 消息类型、分类标识符、路由路径等魔法字符串集中为常量。
- `narrative` 包如确认被 `daily_report` 替代则标记废弃路线。

## Capabilities

### New Capabilities

- `scheduler-seam`: 后端的调度器接口、注册中心、统一生命周期管理、BaseScheduler 工厂模式
- `route-self-registration`: 业务包自注册路由，消除 router.go 上帝函数
- `event-stream-client`: 前端统一 SSE 客户端，替代多连接各自为政
- `error-notification`: 前端全局错误通知系统
- `api-normalization`: 前端 API 层共享的序列化/反序列化/查询工具
- `store-integrity`: 前端 Store 数据一致性，杜绝绕过 API 的本地变更
- `data-access-pattern`: 后端统一的 DB 访问模式（repository 或 service 层间接访问）

### Modified Capabilities

无现有 spec，本次均为新建。

## Impact

- **后端破坏性变更**: `runtimeinfo/` 包删除；调度器构造函数签名可能变化；`router.go` 重构为委托调用。
- **前端破坏性变更**: 多处直连 WebSocket 收口为 `useEventStream()`；Store mutation API 变更（调用者需 await API）；跨 feature 深 import 将迁移到共享 Module 或 feature facade。
- **依赖变更**: 前端删除 `d3-dag`、`p5`、`katex`。
- **向后兼容**: API 响应格式不变，UI 行为不变。
