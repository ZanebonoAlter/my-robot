## Context

分析任务在 Syntopica 后端由两类组件承担：

1. **BaseScheduler 注册的定时 JobFunc**（`internal/app/runtime.go` 注册）：`content_completion` / `firecrawl` / `daily_report` / `board_upgrade_suggest` / `lifeline_weekly` / `lifeline_monthly` / `lifeline_yearly` / `tag_quality_score`。
2. **独立常驻 worker 池**（`tagging.StartAllWorkers`）：`TagQueue`（消费 `tag_jobs`，pollInterval 1s，batchSize 20，concurrency 3）、`EmbeddingQueueWorker`（消费 `embedding_queues`）、`MergeReembeddingQueueWorker`。三者均为 `once.Do` 单例 + `stopChan`，**不经过** BaseScheduler / Registry。

两类组件都消费 DB 持久化队列（`firecrawl_jobs` / `tag_jobs` / `embedding_queues`）：任务以 `status=pending` 静卧，被 lease 后处理，完成后置 completed。停掉消费方，pending 任务安全滞留，恢复后接着消化——这是本变更能简单成立的根基。

现状无全局"暂停"开关：`Registry.StopAll()` 会连入库（`auto_refresh`）一起停，不可用；`feed-tagging-control` 提供 per-feed 分闸（`feed.tagging_enabled`），但无全局总闸。

## Goals / Non-Goals

**Goals:**
- 一键暂停/恢复所有"分析类"处理（scheduler + worker 池两套）。
- 入库（`auto_refresh`）与维护类调度不受影响。
- 暂停状态持久化，服务重启保持。
- 优雅停：不强杀在跑任务。
- 前端顶部栏二态开关 + favicon 暂停态可见 + 切换通知。

**Non-Goals:**
- 不做"队列堆积数显示"（后续增强）。
- 不做定时自动暂停（如每日 20:00–23:00）。
- 不做 per-scheduler 分组暂停（本期是一刀切总闸）。
- 不强杀在跑任务（见 D3）。

## Decisions

### D1: 统一 gate 模式——"lease 前自检暂停标志"
两套组件采用同一模式（暂停标志检查放在 lease 新任务之前）：

- **Scheduler 侧**：声明式 wrapper（如 `analysispause.Guard(jobFunc)` 或 `scheduler.WithPauseGate`）。在 `runtime.go` 注册分析类 job 时包一层；被包的 job 开头自检：暂停则返回 `skipped: paused` 的 JobResult，不 lease。
- **Worker 侧**：`TagQueue` / `EmbeddingQueueWorker` / `MergeReembeddingQueueWorker` 三个 worker 的 lease 循环各加一处 `analysispause.IsPaused()` 自检，暂停时 sleep 一个 poll 周期、continue。

**为何不选"遍历 scheduler 调 Stop()/Start()"**：
- (a) Stop/Start 不持久化状态，服务重启破功（与"重启保持暂停"目标冲突）；
- (b) worker 池不在 Registry，Stop/Start 管不到，仍需单独 gate，不如统一模式；
- (c) 声明式 wrapper 让"哪些 job 受暂停影响"集中可见，新增分析 job 包一层即可，不易漏。

### D2: 标志存储复用 ai_settings
- 新增 config key：`analysis_paused`（bool）+ `analysis_paused_at`（time）。
- 复用 `internal/platform/aisettings/config_store.go` 既有 Load/Save 模式（`daily_report_time` / `board_upgrade_suggest_time` 同款）。
- 无 DB migration、无 schema 变更。

### D3: 优雅停边界
- 自检点放在"lease 前"。一个 tick/lease 周期开始时自检通过 → 本次处理一批（如 firecrawl 一次 N 个、TagQueue batchSize 20）；处理中途用户按暂停 → **本批跑完**，下一周期自检 skip。
- 延迟 = 一个批处理周期（秒~分钟级），对"打游戏"场景可接受。
- 明确不引入强杀/中断逻辑（复杂且易产生半截数据）。

### D4: tag worker 池独立 gate
- 三个 worker（`TagQueue` / `EmbeddingQueueWorker` / `MergeReembeddingQueueWorker`）的 lease 循环各加自检。
- 加 gate 不改变其 `once.Do` 单例 + `stopChan` 生命周期管理，仅在循环内多一个分支。

### D5: favicon 暂停态——SVG 注入
- 暂停时经 `useHead` 注入 `<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,...">`（带 ⏸ 角标的内联 SVG）；运行态恢复 `favicon.png`。
- 参考 `composables/useTheme.ts` 既有 `useHead` 动态改 head 先例。
- 不新增位图文件。

### D6: API + 轮询复用
- `GET /api/analysis/pause` → `{ paused, paused_at }`。
- `POST /api/analysis/pause { paused }` → 写标志、返回新状态。
- 前端暂停态承载于 `composables/useSchedulerStatus.ts`，复用其既有轮询周期，不新增定时器。

## Risks / Trade-offs

- **[暂停及时性]** 当前批跑完才真停（秒~分钟延迟）→ 设计如此（优雅停），可接受；"立即停"属 future work。
- **[新增分析 job 漏包 wrapper]** → wrapper 声明式 + tasks 显式清单 + 归档前 grep 校验"分析类 job 均被 Guard 包裹"。
- **[worker 池 3 处改造]** → tasks 显式列出三个 worker，逐个加自检。
- **[SVG favicon 旧浏览器兼容]** → 单用户现代浏览器场景，可接受；即便不渲染，降级原 `favicon.png` 也无害。
- **[总闸/分闸交互]** → 总闸关时分闸无效须在 specs 明确并以 scenario 覆盖。

## Migration Plan

- 纯增量，无 schema 变更、无数据迁移。
- 部署后默认 `analysis_paused=false`，不影响任何现有行为。
- 回滚：移除 wrapper + worker 自检即可，`ai_settings` 残留标志无害。

## Open Questions

- 暂停态按钮是否加脉冲动画提醒"该恢复了"？（前端增强，apply 阶段定，不阻塞主线。）
