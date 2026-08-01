## ADDED Requirements

### Requirement: 全局分析暂停开关
系统 SHALL 提供全局"分析暂停"开关，控制所有分析类任务的执行。开关状态 SHALL 持久化到数据库，服务重启后 SHALL 保持暂停前的状态。

#### Scenario: 默认不暂停
- **WHEN** 系统首次部署，未设置过暂停标志
- **THEN** analysis_paused 为 false，所有分析类任务正常运行

#### Scenario: 暂停状态跨重启保持
- **WHEN** 用户触发暂停后服务重启
- **THEN** 重启后 analysis_paused 仍为 true，分析类任务仍处于暂停

### Requirement: 暂停生效范围——分析类
当 analysis_paused 为 true 时，分析类调度任务的 JobFunc SHALL 在每次 tick 自检并直接返回、不 lease 新任务，覆盖：content_completion、firecrawl、daily_report、board_upgrade_suggest、lifeline_weekly、lifeline_monthly、lifeline_yearly、tag_quality_score。tag worker 池（TagQueue、EmbeddingQueueWorker、MergeReembeddingQueueWorker）SHALL 不消费各自队列。

#### Scenario: 暂停时调度任务不 lease
- **WHEN** analysis_paused 为 true 且 content_completion 调度器触发 tick
- **THEN** 该 tick 不从队列 lease 任何任务，直接返回 skipped 结果

#### Scenario: 暂停时 tag worker 不消费
- **WHEN** analysis_paused 为 true
- **THEN** TagQueue 不 lease tag_jobs，EmbeddingQueueWorker 不 lease embedding_queues

### Requirement: 入库与维护类不受暂停影响
当 analysis_paused 为 true 时，auto_refresh（RSS 入库）及维护类调度（log_cleanup、aux_label_cleanup、blocked_article_recovery、rsshub_catalog_sync、preference_profile_update）SHALL 继续正常运行。

#### Scenario: 暂停时 RSS 继续入库
- **WHEN** analysis_paused 为 true 且 auto_refresh 触发
- **THEN** RSS feed 照常刷新，新文章照常写入 articles 表

#### Scenario: 暂停时日志清理继续
- **WHEN** analysis_paused 为 true 且 log_cleanup 触发
- **THEN** log_cleanup 照常清理过期日志

### Requirement: 优雅停——不强杀在跑任务
暂停 SHALL 只阻断新的 tick/lease，当前正在执行（已 lease）的任务 SHALL 自然执行完成，系统 SHALL NOT 强制中断在跑任务。

#### Scenario: 暂停时在跑批处理跑完
- **WHEN** firecrawl 调度器已 lease 一批任务正在抓取，此时用户触发暂停
- **THEN** 当前这批抓取任务执行完成，后续 tick 不再 lease 新任务

### Requirement: 恢复后自动续跑
当 analysis_paused 从 true 切回 false 时，暂停期间堆积的 pending 队列任务 SHALL 在后续 tick/lease 周期按既有 created_at 顺序自动消化，无需手动干预。

#### Scenario: 恢复后消化堆积任务
- **WHEN** 暂停期间 tag_jobs 堆积了 50 条 pending，用户触发恢复
- **THEN** TagQueue 在后续周期按 created_at 顺序逐步处理这 50 条，无需手动操作

### Requirement: 总闸与分闸共存
全局分析暂停（总闸）与 feed.tagging_enabled（分闸）SHALL 共存。当全局暂停生效时，即使 feed.tagging_enabled 为 true，该 feed 的文章 SHALL NOT 进入 tag 处理；全局恢复后，分闸重新生效。

#### Scenario: 总闸关时分闸无效
- **WHEN** analysis_paused 为 true 且某 feed 的 tagging_enabled 为 true
- **THEN** 该 feed 的新文章入库但不进入 tag 队列处理

#### Scenario: 总闸恢复时分闸重新生效
- **WHEN** analysis_paused 从 true 切回 false 且某 feed 的 tagging_enabled 为 true
- **THEN** 该 feed 的文章按 feed-tagging-control 既有规则进入 tag 处理

### Requirement: 暂停控制 API
系统 SHALL 提供 GET /api/analysis/pause 返回当前暂停状态（含 paused 布尔与 paused_at 时间），SHALL 提供 POST /api/analysis/pause 接受 { paused: bool } 切换状态并返回新状态。

#### Scenario: 读取暂停状态
- **WHEN** 调用 GET /api/analysis/pause
- **THEN** 返回 { paused: <bool>, paused_at: <time|null> }

#### Scenario: 切换为暂停
- **WHEN** 调用 POST /api/analysis/pause 且 body 为 { paused: true }
- **THEN** 系统写入暂停标志，返回 { paused: true, paused_at: <now> }，后续分析类任务停止 lease

### Requirement: 前端顶部栏暂停开关
首页顶部栏 SHALL 提供二态开关按钮：运行态显示"暂停"动作图标并可点击触发暂停；暂停态以醒目高亮显示"恢复"动作图标并可点击触发恢复。切换时 SHALL 通过全局通知提示用户。暂停态 SHALL 经轮询维持。

#### Scenario: 运行态点击暂停
- **WHEN** 当前 analysis_paused 为 false，用户点击顶部栏暂停按钮
- **THEN** 按钮切换为暂停态高亮，弹出"分析已暂停"通知

#### Scenario: 暂停态点击恢复
- **WHEN** 当前 analysis_paused 为 true，用户点击顶部栏恢复按钮
- **THEN** 按钮切换回运行态，弹出"分析已恢复"通知

### Requirement: 浏览器 favicon 暂停态标识
当 analysis_paused 为 true 时，浏览器标签页 favicon SHALL 切换为带暂停标识（⏸ 角标）的图标；恢复时 SHALL 切回默认 favicon。

#### Scenario: 暂停时 favicon 变化
- **WHEN** analysis_paused 切为 true
- **THEN** 浏览器 tab favicon 显示带 ⏸ 角标的图标

#### Scenario: 恢复时 favicon 复原
- **WHEN** analysis_paused 切回 false
- **THEN** 浏览器 tab favicon 恢复为默认 favicon.png
