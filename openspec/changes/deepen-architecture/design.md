## Context

Syntopica 是一个单用户 RSS 阅读器，后端 Go (Gin/GORM) + 前端 Nuxt 4 (SPA 模式)。当前架构的演进债务集中在三个地方：

1. **后端调度器**：`jobs/` 和 `app/` 之间的循环引用用 `runtimeinfo/schedulers.go` 里 8 个 `interface{}` 全局变量打破。`handler.go` 用类型断言发现方法。调度机制也不统一 (4 个 cron, 2 个 ticker)。
2. **前端事件流**：4 个独立组合函数各自连 WebSocket，各自过滤消息。项目已有 SSE 基础（部分功能用了 SSE），但尚未统一。
3. **跨层数据流**：后端大部分 domain handler 直连 `database.DB`；前端 Store 的 derived store 绕过 API 改本地状态。

## Goals / Non-Goals

**Goals:**
- **以 UI 功能为导向重组后端包架构**：`reader`（RSS阅读）、`topicgraph`（主题图谱）、`tagmanagement`（标签管理）、`admin`（系统管理），替代当前按实体(entity)分包的 `domain/` 结构
- **Spring 式分层**：Handler → Service → Repository，每个功能模块自包含
- 消除 `runtimeinfo/` 包，调度器通过注册中心而非全局 `interface{}` 变量暴露
- `router.go` 从 14 个 domain import 的单体函数变为委托调用
- 前端统一使用 SSE，单连接 + 类型化事件分发
- 前端禁止绕过 API 的 Store 变更，所有 mutation 必须持久化到后端
- 建立统一错误通知管道
- API 层归一化收口（序列化、查询构建）

**Non-Goals:**
- 不引入新的依赖框架（不用 Wire/dig 做 DI）
- 不改变 API 响应格式
- 不重写测试策略，只在新模块上应用新模式
- 不迁移 Pinia → useState（保持 Pinia 为主要状态管理）
- 不处理 narrative → daily_report 的废弃（仅标记，不删除代码）
- Phase 0 不拆分 `models/` 共享模型包（Phase 1 再做）

## Decisions

### D0: 功能导向包重组 (Feature-Oriented Package Organization)

当前 `internal/domain/` 按实体分包：article、feed、category、content、tagging 等。一个 UI 功能（如 RSS 阅读）的代码分散在 5+ 个包中，且 `models/` 是所有实体的大杂烩。

新结构按功能组织，与三个 UI 入口 + 系统管理对齐：

| UI 功能 | 新包名 | 包含的旧包 | Spring 对应 |
|---------|--------|-----------|-------------|
| RSS 阅读三栏 (index.vue) | `internal/reader` | feed + article + category + content | Reader Module |
| 主题图谱 (topics.vue) | `internal/topicgraph` | topicgraph + daily_report | TopicGraph Module |
| 标签管理 (tags.vue) | `internal/tagmanagement` | tagging (+ analysis, watched, extraction 子包) | TagManagement Module |
| 系统管理 (settings) | `internal/admin` | aiadmin + jobs + narrative + preferences | Admin Module |
| 共享模型 | `internal/models` | domain/models（保持共享，Phase 1 再拆） | Shared Entities |
| 基础设施 | `internal/platform` | 不变 | Infrastructure |

**依赖方向**（无循环）：
```
tagmanagement → models, platform (最底层，无其他 feature 依赖)
reader → tagmanagement, models, platform
  (feed/service.go 是核心编排器：refresh→tag→content completion)
topicgraph → tagmanagement, models, platform
  (重度消费 tagging 数据做图谱计算)
admin → reader, topicgraph, tagmanagement, models, platform
  (系统管理需要操作所有功能模块的调度器)
```

**迁移策略**：按依赖顺序从底向上逐包迁移，每步保证 `go build ./...` 通过。

1. `domain/models/` → `internal/models/` (叶子包，无 domain 依赖)
2. `domain/tagging/` → `internal/tagmanagement/` (最独立，不依赖其他 domain 包)
3. `domain/feed+article+category+content` → `internal/reader/` (4 个包合并为 1 个)
4. `domain/topicgraph+daily_report` → `internal/topicgraph/` (2 个包合并为 1 个)
5. `domain/aiadmin+jobs+narrative+preferences` → `internal/admin/` (4 个包合并为 1 个)
6. 删除空的 `internal/domain/` 目录

**合并包的命名约定**：合并多个包时，文件按来源加前缀避免歧义：
- `reader/feed_handler.go`（来自 feed/handler.go）
- `reader/article_handler.go`（来自 article/handler.go）
- `topicgraph/daily_report_handler.go`（来自 daily_report/handler.go）
- `admin/scheduler_handler.go`（来自 jobs/handler.go）

### D0.1: 子包独立 package 名 vs 统一 package 名

Go 的每个子目录是一个独立 package。当前选择每个子包用自己的短名（`package handler`、`package service`、`package repository`），引用时通过完整 import 路径区分：

```go
import (
    "syntopica-backend/internal/reader/handler"
    "syntopica-backend/internal/admin/handler"
)
// 使用：handler.GetCategories(c)   vs   adminhandler.GetSchedulerStatus(c)
```

**备选**：各子包使用功能前缀名（如 `package readerhandler`），避免同名字冲突。当前选择短名 + alias 是因为：
- 子包之间互相引用时别名更短（`service.NewXxx()` vs `readerservice.NewXxx()`）
- 外部消费者通过根包 facade 访问，不直接 import 子包
- alias 在少数冲突处局部解决即可

#### 子包结构全景

```
internal/
├── reader/                    # handler 在根包（公共 API），service/repository 在子包
│   ├── wire.go                # facade：重新导出子包符号，对外统一入口
│   ├── *_handler.go           # HTTP handler（具名导出，router.go 直接引用）
│   ├── service/               # FeedService, ContentCompletionService, FirecrawlService
│   └── repository/            # ReaderRepository, FirecrawlJobQueue
│
├── topicgraph/                # handler/service/repository 全部子包化
│   ├── wire.go                # facade
│   ├── handler/               # daily_report_handler, graph_handler
│   ├── service/               # daily_report 生成/聚类/去重/matching, graph_service
│   └── repository/            # TopicGraphRepository + daily_report 模型/仓库
│
├── tagmanagement/             # handler/service/repository 全部子包化
│   ├── wire.go                # facade（维持 tagging.Xxx 别名兼容）
│   ├── handler/               # tag_management, tag_queue, merge_preview, semantic_board
│   ├── service/               # tagger, hard_merge, merge_suggest, semantic_board_*, embedding...
│   ├── repository/            # TagManagementRepository, MergeReembeddingQueue
│   ├── analysis/              # 已有子包
│   ├── extraction/            # 已有子包
│   └── watched/               # 已有子包
│
└── admin/                     # handler/service/scheduler/repository 四层
    ├── wire.go                # facade
    ├── handler/               # ai/preferences/narrative/scheduler handler
    ├── service/               # narrative service/generator/collector, preferences
    ├── scheduler/             # auto_refresh/content_completion/firecrawl/daily_report...
    └── repository/            # AdminRepository
```

**关键约束：**
- **依赖方向单向**：handler → service → repository → models。禁止反向引用
- **测试文件跟随被测代码**：handler_test 在 handler/，service_test 在 service/
- **循环依赖通过共享 models/ 或 facade 打破**：如 topicgraph 的 daily_report_models 因 service ↔ repository 双需而放入 repository/
- **不直接 import 子包**：外部消费者（router.go、runtime.go）通过根包 facade 引用符号

### D1: 后端调度器接口 + 注册中心（在 admin 包内实现）

```go
// internal/admin/scheduler.go (在 admin 包内)
type Scheduler interface {
    Start() error
    Stop()
    GetStatus() SchedulerStatusResponse
    TriggerNow() map[string]interface{}
    UpdateInterval(seconds int) error
    ResetStats() error
}

type Registry struct {
    mu      sync.RWMutex
    items   map[string]Scheduler
}
```

- `runtimeinfo/` 包删除。`app/runtime.go` 创建 `Registry`，各调度器注册到它。
- `admin/scheduler_handler.go` 通过 `registry.Get(name)` 获取 `Scheduler` 接口值，不再类型断言。
- `TriggerNowWithDate` 作为 `DailyReportScheduler` 的特化方法，handler 通过接口类型判断 `scheduler.(*DailyReportScheduler)` 调用——接口不膨胀到容纳所有特殊需求。
- **备选考虑**：定义 `TriggerableWithDate interface { TriggerNowWithDate(string) ... }` 作为可选扩展接口。Go 的类型断言本身就是这种设计。选前者（具体类型断言）以保持 `Scheduler` 接口整洁。

### D2: 路由自注册模式（在功能包内实现）

```go
// 各功能包新增 routes.go
func RegisterRoutes(rg *gin.RouterGroup) {
    rg.GET("", GetXxx)
    rg.POST("", CreateXxx)
    // ...
}

// router.go 简化为
func SetupRoutes(r *gin.Engine) {
    api := r.Group("/api")
    reader.RegisterRoutes(api)      // /api/feeds, /api/articles, /api/categories, /api/content-completion, /api/firecrawl, /api/opml
    topicgraph.RegisterRoutes(api)  // /api/topic-graph, /api/daily-reports
    tagmanagement.RegisterRoutes(api) // /api/tag-queue, /api/embedding-*, /api/semantic-boards, ...
    admin.RegisterRoutes(api)       // /api/ai, /api/schedulers, /api/reading-behavior, /api/user-preferences
}
```

- 保持 `api.Group("/xxx")` 在各功能包的 `RegisterRoutes` 中创建——路径前缀是功能包的职责。
- 每个功能包负责挂载自己的子路由。不引入路由注册框架。

### D3: 前端 SSE 事件流

```
┌──────────────────────────────────────┐
│ useEventStream()                     │
│  - 单例 SSE 连接                     │
│  - on<T>(type, handler) → 取消订阅    │
│  - send(type, data)                  │
│  - 自动重连 + 心跳                   │
└────┬─────┬─────┬─────────────────────┘
     │     │     │
   TagWS  Report OrgWS  ...
```

- 消息类型字符串 (`'tag_completed'`, `'daily_report_progress'` 等) 集中为一个 `EventTypes` 常量对象。
- 现有 4 个 WebSocket composable 改为订阅特定事件类型。
- **为什么不继续用 WebSocket**：项目已有 SSE 通道，单连接场景 SSE 更简单（服务器推送，客户端只接收）。不需要双向实时通信的场景 SSE 够用。

### D4: Store 操作必须经 API 持久化

- `useArticlesStore` 的 `markAsRead`、`markAllAsRead`、`toggleFavorite` 当前只改本地，改为调用 `apiStore` 的同名方法（那些方法已经调 API）。
- `useArticlesStore` 的 computed 属性保留——它们只是过滤/排序/分组，不写数据。
- 乐观更新：先改 UI（乐观），调 API，失败时回滚。当前直接改 ref 不等 API 返回，加 `await` 和错误回滚。

### D5: 统一错误通知

```ts
// composables/useNotify.ts
export function useNotify() {
  const toasts = useState<Toast[]>('notify:toasts', () => [])
  return {
    success(msg: string): void
    error(msg: string): void
    warn(msg: string): void
    toasts: readonly(toasts)
  }
}
```

- 全局 toast 队列，组件只需 `const notify = useNotify(); notify.error('加载失败')`。
- 不引入第三方 toast 库，自己实现轻量通知组件放在 `app.vue` 层。
- API store 的错误自动推送通知，组件不再需要处理 `error.value`。

### D6: API 层归一化

```ts
// utils/api-helpers.ts
export function normalizePagination(params: Record<string, any>): string
export function snakeToCamel<T>(data: any): T

// tagQueue 和 embeddingQueue 共用泛型基类
export function createQueueApi<T>(endpoint: string): QueueApi<T>
```

- `buildQueryParams` 改为自动拼接 `?`——调用者不再手动 `?${query}`。
- 6 套独立 normalizer 收口为一个 `camelize<T>(data)` 通用函数 + 类型特定的 `mapXxx` 薄包装。

### D7: tagmanagement/service/ 按功能域拆子包

当前 `tagmanagement/service/` 有 40 个文件（24 业务 + 16 测试），涵盖 6 个独立功能域：

| 功能域 | 文件 | 行数 |
|--------|------|------|
| Tagging | article_tagger, tagger, tag_queue, workers, helpers, services, types, tag_cache, config_service | 1862 |
| Semantic Board | semantic_board_matching, semantic_board_upgrade, semantic_board_backfill, tag_clustering | 1978 |
| Embedding | embedding, embedding_queue, merge_reembedding_queue, description_backfill, person_metadata_backfill, cotag_expansion | 1678 |
| Extraction | extractor_enhanced, extractor_heuristic | 741 |
| Tag Merge | tag_merge_suggest, hard_merge | 623 |
| Auxiliary Label | auxiliary_label_service | 669 |

**推荐方案**：在 `service/` 下按域建子包：

```
service/
├── tagging/           # 打标核心流程
├── board/             # 语义看板
├── embedding/         # 向量化管线
├── merge/             # 标签合并
└── auxlabel/          # 辅助标签
```

`service/` 下的 `extractor_enhanced.go` 和 `extractor_heuristic.go` 移入已有的 `extraction/` 一级子包（与 `quality_score.go` 归一处）。已确认同意此方案。

### D8: tagmanagement 组织范式统一

当前混用两种模式：
- **垂直切片**：`analysis/`（有独立的 handler + service + queue）、`watched/`（有独立的 handler + service）
- **水平分层**：`handler/` + `service/` + `repository/` 按技术层分

**调查结论**：`analysis/` 包后端有路由注册但前端零调用——**确认是废弃代码**，直接删除整个 `analysis/` 目录。

`watched/` 拆解为水平分层：handler 移入 `handler/`，service 移入 `service/` 下对应子包。

已确认同意统一为水平分层。

### D9: narrative 废弃 + admin 包瘦身

**调查结论**：Narrative 后端有完整的 handler（12 个 endpoint）+ service（7 个文件 ~35K 行）+ models，但前端唯一消费者 `NarrativeGenerateDialog.vue` 实际调用的是 `useDailyReportsApi().generateDailyReport()`——UI 壳子用着 Narrative 名字，内核已经是 Daily Report。前端无任何 narrative API 调用。Daily Report 已完全替代了 Narrative 的功能。

**推荐方案**：
1. 后端删除 `admin/handler/narrative_handler.go` + `admin/service/narrative_*.go`（7 个文件）
2. `admin/repository/repository.go` 中 narrative 相关的 DB 方法清理
3. `models/narrative.go` + `models/narrative_board.go` 评估是否还被其他代码引用（如无则一并删除）
4. 前端 `NarrativeGenerateDialog.vue` 重命名为 `DailyReportGenerateDialog.vue`，消除命名混淆
5. `admin/wire.go` 中 `RegisterNarrativeRoutes` re-export 删除
6. `admin` 包瘦身后剩余：AI 管理 + 调度器 + 偏好设置，内聚性显著提升

这解决了原 D9（admin 包职责梳理）的核心问题——不再需要跨包迁移，直接删废弃代码即可。

### D10: platform/ai 合并到 platform/airouter

`platform/ai/service.go` (222行) 是 airouter 的 fallback，职责高度重叠。

**推荐方案**：将 `ai/service.go` 的内容合并到 `airouter/` 包内（如 `airouter/fallback.go`），删除 `platform/ai/` 目录。影响范围：检查所有 import `platform/ai` 的消费者并更新。

### D11: 巨型文件拆分

| 文件 | 行数 | 拆分建议 |
|------|------|----------|
| `tagmanagement/handler/semantic_board_handler.go` | 1923 | 按操作拆：`board_crud_handler.go` + `board_match_handler.go` + `board_upgrade_handler.go` |
| `topicgraph/repository/repository.go` | 1114 | 按实体拆：`graph_repository.go`（已有部分）+ 确认剩余职责归属 |
| `admin/service/narrative_service.go` | 1114 | 随 D9 整体删除 narrative 代码 |
| `topicgraph/service/daily_report_generator.go` | 1031 | 按阶段拆：初始化 / 收集 / 聚类 / 生成 / 后处理 |
| `tagmanagement/analysis/analysis_queue.go` | 871 | 按队列操作拆或保持（如果是单一队列的完整生命周期） |

## Risks / Trade-offs

- **功能包重组 → 大规模 import 路径变更**：约 85 个文件需要移动或修改 import。但按依赖顺序逐包迁移，每步可独立验证。`git mv` + `sed` 全局替换可自动化。
- **多包合并 → 命名冲突**：feed + article + category + content 合并为 reader 时，函数名无冲突（已按实体名前缀命名）。但 type 可能冲突，需逐一检查。
- **调度器接口变更 → 所有 scheduler 文件需改动**：每个 scheduler 已有大部分方法，只需显式声明实现了接口。改动量大但机械，可逐个进行。
- **SSE vs WebSocket**：SSE 只支持服务器→客户端，如果将来需要客户端→服务器实时消息则需回退。当前所有实时消息均为服务器推送，无此需求。
- **Store 操作加 await → 响应延迟**：从"瞬间改 UI"变成"调 API 再更新"。对标记已读/收藏这种高频操作，采用乐观更新策略——先改 UI，API 失败时回滚 + 通知。
- **大范围重构 → 功能回归风险**：分批实施，每批独立合并。先做后端包重组→分层→路由→调度器；再做前端。

## Migration Plan

### Phase 0: 后端功能包重组（新增，最高优先级）✅ 已完成
1. `domain/models/` → `internal/models/`（全局更新 import）
2. `domain/tagging/` → `internal/tagmanagement/`（最独立，无 domain 依赖）
3. `domain/feed+article+category+content` → `internal/reader/`（4 合 1）
4. `domain/topicgraph+daily_report` → `internal/topicgraph/`（2 合 1）
5. `domain/aiadmin+jobs+narrative+preferences` → `internal/admin/`（4 合 1）
6. 删除 `internal/domain/` 目录，全量验证

### Phase 1: Spring 式分层（在新功能包内补齐）✅ 已完成
1. 每个功能包添加 `repository.go`，handler 内 DB 调用迁移到 repository
2. handler 只做 HTTP 参数解析 + 响应序列化
3. service 封装业务逻辑 + 调 repository
4. 子包拆分：handler/service/repository 子目录化（见 D0 子包组织结构）

### Phase 1.7: 后端包内组织优化（代码审查发现）
1. tagmanagement/service/ 按功能域拆子包（tagging/board/embedding/merge/auxlabel）
2. tagmanagement 组织范式统一（垂直切片 → 水平分层）
3. admin/service/narrative/ 子包化
4. platform/ai 合并到 platform/airouter
5. 巨型文件拆分

### Phase 2: 路由自注册 + 调度器接口化
1. 各功能包添加 `routes.go`（RegisterRoutes）
2. `router.go` 简化为委托调用
3. 定义 Scheduler 接口 + Registry
4. 删除 `runtimeinfo/` 包

### Phase 3: 前端 SSE 统一（需后端配合）
1. 后端确认/补充 SSE 端点覆盖所有当前 WS 消息类型
2. 前端实现 `useEventStream()`
3. 逐个迁移 4 个 WS composable
4. 删除旧的独立 WS 连接

### Phase 4: 前端 Store + Error + API 收口（可并行）
1. 修 Store bypass → 加 API 调用 + 乐观更新
2. 加 `useNotify` → 替换组件内 error refs
3. 收口 normalization → 删重复工具
4. 拆 `stores/api.ts` 巨型 store 为功能 store

### Phase 5: 代码卫生 + 清理

## Open Questions

1. SSE 端点当前有哪些？是否需要新增端点覆盖所有 WS 消息类型？
2. 后端 `platform/tracing/handler.go` 是否移到独立包（保持现状也行，它是基础设施）？
3. `models/` 共享包拆分粒度：按功能包拆分 vs 保留共享？TopicTag 等类型被多个功能包使用。

## 已决策

- **D7 extraction 归属** → 同意方案 A：service 下的 extractor 移入已有 extraction/ 子包
- **D8 垂直切片 → 水平分层** → 同意；analysis 包已确认为废弃代码，直接删除
- **D9 Narrative 废弃** → Daily Report 已替代 Narrative 功能；后端 narrative 代码删除，前端组件重命名
