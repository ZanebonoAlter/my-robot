## Why

日报 thread（事件）粒度的跑题**完全没有质量信号**：LLM 在 `ClusterTags`(Step3) 偶尔把语义不相关的 tag（如"华为腾讯百度争夺机器人大脑生态"）误归进某叙事框架，`GenerateClusterThreads` 忠实生成对应 thread，它随所在 section 锚定成功而**搭便车**进入话题——用户看到"机器人事件挂在 OpenAI 债务危机话题下"却无任何信号提示它离群。这是**粒度错位**：现有 observability 量到 section↔topic（`topic-anchor-match-observability`）和 tag↔板块（`quality-scoring-observability`），独缺 **thread↔section** 这一层。

为什么不上"归簇时 embedding 紧凑性剔除"治本：伞形/开放性话题（如「XR 硬件爆发：从消费级新品到全球军援」）的子事件在 embedding 空间天然分散（distance spread 达 0.29，几乎贴着锚定阈值上限），按紧凑性误杀会割掉合理叙事。**事后校验 thread 是否忠于它所在 section 的标题（叙事宣言）**，对紧凑型/伞形型 section 自适应——本 change 走这条更安全的治理点。

## What Changes

- **后端贴合度计算**：thread 生成后，计算 thread 标题 embedding 与其所属 section 标题 embedding 的**余弦距离**，作为 thread 贴合度信号落库（不跑题则近、跑题则远）。复用 orchestrator 现有 embedding 链路。
- **后端落库**：`daily_report_threads` 表新增 embedding 列 + 贴合距离列；detail/timeline 等接口 SELECT 新字段，前端类型契约扩展。
- **前端标记 + 软降级**：离群 thread（距离超阈值）**灰显/折叠**，保留信息不删除，附离群标记；hover/展开可查看贴合度数值与中文标签。正文极轻（形态/降级，无数字），分数文字只进探究区——沿用 observability 系列展示分层哲学。
- **历史兼容**：历史 thread 无贴合度字段统一降级为正常 thread（不降级、不报错）。无回刷、无迁移。

## Capabilities

### New Capabilities
<!-- 无。本 change 给已有 capability 加 requirement，不新建。 -->

### Modified Capabilities
- `thread-storage`: 新增「Thread 贴合度信号落库」requirement——`daily_report_threads` 存储 thread 标题 embedding 及其与所属 section 标题的余弦贴合距离，detail/timeline 接口暴露该字段。
- `daily-report-system`: 新增「Thread 贴合度可视化与软降级」requirement——离群 thread 灰显/折叠 + 离群标记 + 探究区贴合度行；信号随 section 叙事广度自适应，不预设"组须紧凑"。

## Impact

- **后端（`backend-go/`）**
  - 修改：`internal/topicgraph/service/daily_report_orchestrator.go`（thread 装配后批量算 thread↔section 标题向量距离，复用 `Embed` 调用）；`internal/topicgraph/repository/daily_report_models.go`（`DailyReportThread` 加 `Embedding`/`FitDistance` 字段）；`repository/daily_report_repository.go`（thread 写入 SELECT 新列 + 四接口 detail/timeline/lifeline/topic-lifeline SELECT 贴合距离）。
  - 数据：`daily_report_threads` 加 `embedding vector(2560)` + `fit_distance numeric` 两列（AutoMigrate），历史行留空。
- **前端（`front/app/`）**
  - 修改：`api/dailyReports.ts`（`DailyReportThread` 类型加 `fit_distance`/`embedding`）；thread 渲染组件（`features/tags/components/daily-report/` 内，挂载于 `DailyReportTopicSection` 的 threads 列表）加贴合度信号 + 软降级样式 + 探究行。
  - 新增：`utils/threadFit.ts`（贴合度分档 + 中文标签）+ 单测。
- **数据兼容**：历史 thread 的 `fit_distance` 为空，前端按"正常 thread"渲染（不降级、不报错）。无迁移、无回刷。
- **不做**：不检测/重组 section 内 tag（源头治理留作后续，见 `embedding-content-mismatch` 待办 issue）；不做归簇后 embedding 紧凑性剔除（伞形话题误杀风险）；不做反哺回路（事后校验不回流到事前 ClusterTags 约束）；不改 System 1/2 可视化与锚定算法。
