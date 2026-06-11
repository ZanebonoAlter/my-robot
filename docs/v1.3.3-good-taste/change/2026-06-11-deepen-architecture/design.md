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
2. **拆 `stores/api.ts` 巨型 store 为功能 store**（D13）
3. 加 `useNotify` → 替换组件内 error refs
4. 收口 normalization → 删重复工具

### Phase 5: 代码卫生 + 清理

## Open Questions

1. SSE 端点当前有哪些？是否需要新增端点覆盖所有 WS 消息类型？
2. 后端 `platform/tracing/handler.go` 是否移到独立包（保持现状也行，它是基础设施）？
3. `models/` 共享包拆分粒度：按功能包拆分 vs 保留共享？TopicTag 等类型被多个功能包使用。

### D12: 调度器工厂模式（BaseScheduler 消除重复脚手架）

**问题**：9 个 scheduler 文件共 ~3320 行，其中每个文件都重复实现了 Start/Stop、TriggerNow（互斥锁+返回 map）、UpdateInterval（stop→改 interval→restart）、ResetStats、GetStatus、状态字段（`mu`、`running`、`isExecuting`、`nextRun`、`lastRun`、`lastError`...）等脚手架代码。实际业务逻辑仅占 ~30%。

此外，调度机制也不统一：A 类（auto_refresh、content_completion、daily_report）用 `robfig/cron`；B 类（firecrawl、log_cleanup、blocked_article_recovery 等）用 `time.Ticker` + goroutine。但对外接口完全一样——底层差异不应暴露。

**方案**：引入 `BaseScheduler`，将脚手架统一到一处，业务逻辑收敛为 `JobFunc func(ctx context.Context) (*JobResult, error)`。

```go
// scheduler/base.go

type JobFunc func(ctx context.Context) (*JobResult, error)

type JobResult struct {
    Data    map[string]interface{} // 业务指标（写入 GetStatus）
    Summary string                  // 日志摘要
}

type Config struct {
    Name         string
    Interval     time.Duration
    StartupDelay time.Duration
    Job          JobFunc
}

type BaseScheduler struct {
    // 封装所有公共状态：mutex, running, isExecuting, nextRun, lastRun,
    // lastError, totalRuns, successRuns, failedRuns, cron/ticker...
    // 实现 Scheduler 接口的全部方法
    ...
}

func New(cfg Config) *BaseScheduler { ... }
```

注册新 job 从 250 行缩减为 ~30 行：

```go
// jobs/log_cleanup.go — 只写业务逻辑
func logCleanupJob(ctx context.Context) (*scheduler.JobResult, error) {
    cutoff := time.Now().AddDate(0, 0, -7)
    aiDeleted := repository.Repo.DB().Exec("DELETE FROM ai_call_logs WHERE created_at < ?", cutoff).RowsAffected
    otelDeleted := repository.Repo.DB().Exec("DELETE FROM otel_spans WHERE start_time_unix_nano < ?", cutoff.UnixNano()).RowsAffected
    return &scheduler.JobResult{
        Data: map[string]interface{}{
            "ai_call_logs_deleted": aiDeleted,
            "otel_spans_deleted":   otelDeleted,
        },
        Summary: fmt.Sprintf("ai_call_logs=%d, otel_spans=%d", aiDeleted, otelDeleted),
    }, nil
}
```

**迁移策略**：分三批，由简到难——
1. 简单 scheduler（log_cleanup、aux_label_cleanup、blocked_article_recovery）：无状态持久化、无 TriggerNow 特殊逻辑
2. 中等 scheduler（auto_refresh、preference_update、tag_quality_score）：有 SchedulerTask DB 状态持久化
3. 复杂 scheduler（content_completion、daily_report、firecrawl）：有额外的特化方法（如 `TriggerNowWithDate`）和复杂业务依赖

**收益估算**：

| 指标 | 现在 | 工厂后 |
|------|------|--------|
| 代码量 | ~3320 行 | base.go ~400 行 + 9×50 行 ≈ 850 行 |
| 新增 scheduler | 复制 250 行 | 写一个函数 ~30 行 |
| GetStatus 不在接口中 | handler type assertion | BaseScheduler 统一实现 |
| A/B 类并发差异 | 每个文件自己选 | BaseScheduler 内部统一 |

## 已决策

- **D7 extraction 归属** → 同意方案 A：service 下的 extractor 移入已有 extraction/ 子包
- **D8 垂直切片 → 水平分层** → 同意；analysis 包已确认为废弃代码，直接删除
- **D9 Narrative 废弃** → Daily Report 已替代 Narrative 功能；后端 narrative 代码删除，前端组件重命名
- **D12 调度器工厂模式** → 同意 BaseScheduler 方案；分三批迁移（简单→中等→复杂）
- **D13 stores/api.ts 拆分** → 保留 api.ts 仅做基础能力（HTTP 封装、全局 loading/error），各功能域的 API 调用下沉到对应 feature composables 或独立 store
- **D14 前端巨型组件拆分阈值** → 单文件 >500 行（~15K）应拆分，优先处理 >40K 的 4 个组件

### D13: stores/api.ts 拆分策略

`stores/api.ts` (13K) 是前端最大的技术债，承载了 feeds / articles / categories / tags / scheduler / content-completion / firecrawl 等所有实体的 CRUD 状态和方法。

**问题**：
- 所有实体状态（feeds、articles、categories 等）和方法（fetchXxx、createXxx、updateXxx）堆积在一个 store 中
- 其他 store（`articles.ts`、`feeds.ts`）与 api.ts 职责边界模糊
- Feature composables 需要直接 import api store 获取数据，耦合严重

**拆分策略**：

```
stores/
├── api.ts              # 瘦身后：HTTP client 封装、全局 loading/error 状态
├── articles.ts         # 文章相关状态 + API 调用（从 api.ts 迁入）
├── feeds.ts            # Feed 相关状态 + API 调用（从 api.ts 迁入，合并已有 feeds.ts）
└── preferences.ts      # 保持不变
```

各 feature composables 中的 API 调用逻辑：
- `features/feeds/composables/useAutoRefresh.ts` → 直接调用 `api/` 层，不经过 store
- `features/topic-graph/composables/` → 通过 `api/topicGraph.ts` 直接调用
- `features/tags/components/` → 通过 `api/semanticBoards.ts` 等直接调用

**原则**：
- Store 只管理需要跨组件共享的状态
- 一次性的 API 调用（如对话框中的 CRUD）直接走 `api/` 层，不经过 store
- Store 内的 mutation 必须调用 API 持久化（Phase 4 的 store-integrity 约束）

**迁移步骤**：
1. 盘点 api.ts 中所有状态和方法，按功能域分组
2. feeds 相关 → `stores/feeds.ts`（已有 `feeds.ts`，内容合并）
3. articles 相关 → `stores/articles.ts`（已有 `articles.ts`，补充 API 方法）
4. scheduler/content-completion/firecrawl → 各自 feature composable 内部管理
5. api.ts 瘦身至仅保留基础能力

### D14: 前端巨型组件拆分阈值

当前 >15K 的组件清单（按大小排序）：

| 组件 | 大小 | 位置 | 拆分优先级 |
|------|------|------|------------|
| `TopicGraphPage.vue` | 74K | features/topic-graph | P0 |
| `GlobalSettingsDialog.vue` | 67K | components/dialog | P0 |
| `TagsPage.vue` | 54K | features/tags | P0 |
| `AIRouterSettingsPanel.vue` | 42K | features/ai | P1 |
| `ArticleContentView.vue` | 39K | features/articles | P1 |
| `TagMergePreview.vue` | 32K | features/topic-graph | P1 |
| `TopicGraphSidebar.vue` | 29K | features/topic-graph | P1 |
| `BoardDailyReportTimeline.vue` | 30K | features/tags | P1 |
| `BoardThreadBrowser.vue` | 24K | features/tags | P1 |
| `SectionLifecyclePanel.vue` | 22K | features/tags | P2 |
| `FeedLayoutShell.vue` | 18K | features/shell | P2 |
| `UpgradeSuggestionPanel.vue` | 16K | features/tags | P2 |

**拆分规则**：
- **>40K (P0)**：本轮必须拆，优先处理
- **20K–40K (P1)**：本轮尽量拆，视带宽推进
- **15K–20K (P2)**：标记但本轮不强制，后续迭代处理

**拆分模式**（统一采用）：
1. **抽取 composable**：状态管理 + 业务逻辑 → `useXxxPage()` 或 `useXxxPanel()`
2. **拆子组件**：模板中独立区块 → 各自 `<XxxSection>` / `<XxxPanel>` 组件
3. **消除 props drilling**：通过 composable 共享状态，子组件从 composable 读取
4. **目标行数**：页面级组件 <300 行，面板级组件 <200 行

### D15: 事件流 Seam 必须真实统一生命周期

当前复盘发现 `useEventStream()` 的外部 Interface 已经统一，但内部仍基于 `/ws` WebSocket，且存在两个新的耦合/生命周期问题：

- 后端全局事件目前只有 `/ws` WebSocket；只有 tag merge preview 的 scan/evaluate 是专用 SSE endpoint。
- 单例连接 `destroy()` 后仍保留全局实例，下一次订阅可能复用已 destroyed 的连接对象，导致后续页面无法重连。
- `useDailyReportProgress()` 订阅事件但未保存 unsubscribe，组件卸载后 handler 仍留在全局事件流中。
- `TagQueuePanel.vue` 仍自建 WebSocket，绕过统一事件流 Seam。

**决策**：事件流 Module 必须作为唯一全局实时事件 Seam。由于后端尚无覆盖 `/ws` 所有消息类型的全局 SSE 端点，本轮保留 WebSocket Adapter，但文档/spec 统一改为“实时事件连接”而非宣称已完成 SSE 切换。

**落地约束**：
1. `useEventStream()` 的连接 Adapter 与文档/spec 保持一致：当前是 `/ws` WebSocket Adapter；未来新增全局 SSE 端点后可在不改变外部 Interface 的情况下切换为 `EventSource`。
2. `on()` 返回的 unsubscribe 是唯一退订 Interface，所有调用方必须在 `onUnmounted` 或 composable 清理阶段执行。
3. 最后一个订阅者退订时只关闭连接并释放全局实例，不留下 destroyed singleton。
4. 禁止 feature/component 直接 `new WebSocket` / `new EventSource` 订阅全局事件；特例（如长任务专用 stream）必须在对应 API module 中作为命名 Adapter 暴露。

### D16: Store 数据一致性的真正 Seam 是功能 Store，不是 `apiStore`

当前 `useArticlesStore` 通过 `apiStore.markAsRead/markAllAsRead/toggleFavorite` 持久化，而 `apiStore` 又动态 import `useArticlesStore` 刷新/读取文章状态。这形成了 Store 间循环知识：两个 Store 都知道对方内部状态和更新顺序。

**决策**：文章写入的唯一外部 Interface 是 `useArticlesStore()`；`apiStore` 不再承载文章 mutation。`apiStore` 只保留 categories/feeds 的过渡能力，最终按 D13 继续瘦身。

**落地约束**：
1. `useArticlesStore.markAsRead/markAllAsRead/toggleFavorite` 直接调用 `useArticlesApi()`，在本 Store 内完成乐观更新、回滚和错误通知。
2. Feed unread count 的同步通过 `useFeedsStore` 暴露小 Interface（如 `clearUnreadCounts` / `adjustUnreadCount`），不允许 articles store 直接遍历 `apiStore.feeds`。
3. `useApiStore` 不再动态 import `~/stores/articles`；跨 Store 初始化由页面/应用启动层编排。
4. `useFeedsStore` 不应只是 `apiStore.feeds/categories` 的 shallow wrapper；要么合并真实 Feed 状态与 API 调用，要么删除 wrapper 让调用方直接使用清晰的 Reader/Feed Module。

### D17: 大组件拆分不能只把复杂度搬进单个 composable

当前 `TopicGraphPage.vue` 已显著缩小，但 `useTopicGraph.ts` 变成 >1100 行，暴露大量 refs、computed、handlers、loaders。Deletion test 显示：删除该 composable 后复杂度不会消失，而是完整回到页面；说明它仍是 shallow Module，只是换了文件位置。

**决策**：页面拆分的目标不是文件行数下降，而是形成多个深 Module，每个 Module 以小 Interface 隐藏一组高内聚行为。

**建议拆分形态**：

| Module | 职责 | 外部 Interface |
|--------|------|----------------|
| `useTopicGraphQuery` | 图谱筛选、加载、view model | `filters`, `viewModel`, `loadGraph`, `loading/error` |
| `useTopicGraphSelection` | 节点选择、高亮、图谱可见性 | `selectedTopic`, `highlightedNodeIds`, `selectNode`, `toggleVisible` |
| `useTopicTimeline` | digest、pending articles、聚合分组 | `items`, `groups`, `selectDigest`, `loadForTopic` |
| `useFloatingPanelDrag` | 浮层拖拽通用逻辑 | `panelRef`, `position`, `startDrag`, `reset` |
| `useArticlePreview` | 文章预览、收藏、局部更新 | `selectedArticle`, `open`, `close`, `toggleFavorite` |

同样规则适用于 `useTagsPage()`、`useGlobalSettings()` 和后续 AIRouter/ArticleContent 拆分。

### D18: Feature 之间不得深 import 对方内部实现

当前复盘发现 `tags` / `topic-graph` feature 直接 import `features/articles/utils/normalizeArticle`、`features/articles/components/ArticleContentView.vue`，`TagsPage.vue` 直接 import `features/topic-graph/components/TagMergePreview.vue`。这些都是跨 feature 的内部实现耦合。

**决策**：跨 feature 共享能力必须上移到共享 Module 或通过 feature 的显式 Facade 暴露，禁止深 import 对方内部组件/工具。

**落地约束**：
1. `normalizeArticle` 移到 `api/normalizers/article.ts` 或 `types/article/normalize.ts`，成为底层数据 Adapter。
2. 可复用 UI（如文章预览、合并预览）若确实跨 feature 使用，应放入 `components/` 或建立 feature facade（如 `features/articles/public.ts`），只暴露稳定 Interface。
3. `features/*/components` 内部组件默认视为 feature 私有；其他 feature 不直接引用。

### D19: API 归一化只能有一个查询/解包 Seam

当前新增了 `utils/api-helpers.ts`，但 `api/client.ts` 仍有 `buildQueryParams`，`createQueueApi.ts` 仍手写 `URLSearchParams`。这会让 query building 规则继续分叉。

**决策**：查询构建、响应解包、snake_case 转换必须分别只有一个权威 Module。

**落地约束**：
1. `apiClient.buildQueryParams` 内部委托 `buildQueryString`，或删除其中一个，保留唯一公共 Interface。
2. `createQueueApi` 必须复用统一 query builder。
3. API module 中的 `as unknown as ApiResponse<T>` 逐步替换为 `unwrapResponse<T>` 或类型化 normalizer。
4. 后端 snake_case DTO 可以保留在 API 层类型中，但进入 Store/Feature 前必须经 normalizer 转为前端领域类型。

### D20: 通知 Module 按 Nuxt 状态模型实现，避免重复通知

当前 `useNotify()` 使用模块级 `ref`，与设计中的 `useState('notify:toasts')` 不一致。如果未来启用 SSR，模块级状态有跨请求污染风险。另有 `apiStore` 和 feature store 同时通知的重复风险。

**决策**：`useNotify()` 使用 Nuxt `useState` 管理 toast 队列，并规定错误通知的唯一责任层。

**落地约束**：
1. `useNotify()` 内部用 `useState<Toast[]>('notify:toasts', () => [])`。
2. 写操作失败由执行该写操作的 Store/Composable 通知；底层 API module 不直接弹 toast。
3. 同一次失败不得由 `apiStore` 与 feature store 同时通知。
4. View 组件可保留必要的局部 `error` 展示状态，但全局错误反馈统一走 `useNotify()`。
