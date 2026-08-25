# article-content-crawling Delta Spec

## ADDED Requirements

### Requirement: firecrawl 队列并行消费（worker pool 并发 3）
firecrawl 定时任务 SHALL 在 claim 一批任务（50 个）后，以固定 3 个 worker 并行消费该批任务；每个 worker 处理完一个任务后 SHALL 保留对目标站点的礼貌限速间隔（500ms）。

#### Scenario: 一批任务由三个 worker 并行处理
- **WHEN** 队列中有 50 个 pending 任务被 claim
- **THEN** 3 个 worker 同时各处理一个任务，总耗时接近串行模式的 1/3（受目标站点响应时间影响）

#### Scenario: 限速间隔保留
- **WHEN** 单个 worker 连续处理两个任务
- **THEN** 两次抓取之间至少间隔 500ms，避免对同一目标站点高频请求

### Requirement: 并发下的计数与进度广播正确性
completed/failed 计数 SHALL 使用原子操作累加；WS 进度广播（`firecrawl_progress`）SHALL 广播原子读取的计数快照，批次结束时总数 SHALL 等于 completed + failed。

#### Scenario: 并发完成计数准确
- **WHEN** 3 个 worker 各自完成/失败若干任务后批次结束
- **THEN** 广播的 completed + failed 等于本批 total，无丢失或重复计数

#### Scenario: 失败降级路径在并发下不变
- **WHEN** 并发处理中某任务达到重试上限（terminal）
- **THEN** 仍按现有逻辑降级：置 `firecrawl_status=failed`、按 feed 配置置 `summary_status=incomplete` 并入 fallback retag 队列，与串行行为一致

### Requirement: 现有租约与退避机制不受并行化影响
并行化 SHALL 不改变队列的 claim 批大小（50）、租约时长（单页超时 + 5min）、失败退避（1min→30min 指数，上限 5 次）语义。

#### Scenario: worker 崩溃后任务可回收
- **WHEN** 某 worker 处理中进程崩溃，任务租约过期
- **THEN** 下个 tick 任务被重新 claim，行为与串行模式一致
