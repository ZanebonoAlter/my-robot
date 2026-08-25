## MODIFIED Requirements

### Requirement: 全局分析暂停开关

系统 SHALL 提供全局"分析暂停"**用户开关**（`analysis_paused`，true=用户主动暂停），其值 SHALL 持久化到数据库，服务重启后 SHALL 保持用户上次设置不变。开关的读取（`GET /api/analysis/pause`）与切换（`POST /api/analysis/pause`）API、前端按钮行为 SHALL NOT 因 AI 模型健康状态而改变。

分析类任务是否**实际运行** SHALL 由「有效暂停态」决定：

> 有效暂停 = 用户暂停（analysis_paused） **或** NOT 健康（由 ai-model-health capability 定义）。

当 `analysis_paused=false`（用户未暂停）但模型未就绪（NOT 健康）时，分析类任务 SHALL 同样不 lease、不消费队列；但用户开关状态 SHALL 保持 false 不被改动（前端以健康提示告知，而非翻转开关）。当健康就绪后（且用户未暂停），分析 SHALL 自动恢复运行，无需用户再次手动启动。

暂停生效范围（分析类）、入库与维护类不受影响、优雅停、恢复后自动续跑、总闸与分闸共存等既有语义 SHALL 保持不变，仅"是否实际运行"增加健康维度。

#### Scenario: 用户开关默认不暂停

- **WHEN** 系统首次部署，未设置过暂停标志
- **THEN** analysis_paused 为 false（用户意图=运行）

#### Scenario: 用户开关跨重启保持

- **WHEN** 用户触发暂停后服务重启
- **THEN** 重启后 analysis_paused 仍为 true（用户意图不变）

#### Scenario: 健康未就绪时开关不变但任务不跑

- **GIVEN** analysis_paused=false（用户未暂停），模型未就绪（NOT 健康）
- **WHEN** content_completion 调度器触发 tick
- **THEN** 该 tick SHALL 不 lease 任何任务（有效暂停），且 GET /api/analysis/pause SHALL 仍返回 paused=false

#### Scenario: 健康就绪后自动恢复运行

- **GIVEN** analysis_paused=false，此前因 NOT 健康而暂停运行
- **WHEN** 启动健康检测完成且判定健康
- **THEN** 分析类任务 SHALL 恢复 lease（无需用户点启动）

#### Scenario: 用户手动暂停优先级不变

- **WHEN** analysis_paused=true（用户主动暂停），即便模型健康
- **THEN** 有效暂停 SHALL 为 true，分析类任务 SHALL 不运行

## ADDED Requirements

### Requirement: 健康门硬执行

当 AI 模型未就绪（NOT 健康，见 ai-model-health）时，所有分析类调度任务的 JobFunc SHALL 在每次 tick 自检 `analysispause.IsPaused()`（= 用户暂停 || NOT 健康）并直接返回、不 lease 新任务；tag worker 池（TagQueue、EmbeddingQueueWorker、MergeReembeddingQueueWorker）SHALL 不消费各自队列。`IsPaused()` 在健康快照未就绪（启动竞态）时 SHALL 视 NOT 健康返回 true（保守，分析不跑）。手动切换暂停/恢复（POST /api/analysis/pause）SHALL NOT 因健康状态被拒绝——开关仅表达用户意图，实际是否运行由健康门在 worker 侧裁定。

#### Scenario: 模型未就绪时调度任务不 lease

- **GIVEN** analysis_paused=false（用户未暂停），模型 NOT 健康
- **WHEN** content_completion / firecrawl / daily_report 等分析类调度器触发 tick
- **THEN** 各 tick SHALL 不 lease 任务，直接返回（与用户主动暂停表现一致）

#### Scenario: 模型未就绪时 tag worker 不消费

- **WHEN** 模型 NOT 健康
- **THEN** TagQueue SHALL 不 lease tag_jobs，EmbeddingQueueWorker SHALL 不 lease embedding_queues

#### Scenario: 启动竞态期视为不健康

- **WHEN** 后端刚启动、健康快照未就绪（首次检测未完成）
- **THEN** IsPaused() SHALL 返回 true，分析类任务 SHALL 不 lease

#### Scenario: 手动启动不被健康状态拒绝

- **GIVEN** 模型 NOT 健康，analysis_paused=true
- **WHEN** 用户点击启动（POST /api/analysis/pause { paused:false }）
- **THEN** 系统 SHALL 接受请求、写入 analysis_paused=false（用户意图），SHALL NOT 返回错误；分析类任务是否实际运行仍由健康门在 worker 侧裁定

#### Scenario: 恢复时重新探活

- **WHEN** 用户点击恢复（POST /api/analysis/pause { paused:false }）
- **THEN** 系统 SHALL 在更新用户开关后异步触发一次 RunStartupProbe，使健康门能自愈（启动探活曾失败、或模型后来才就绪时，点恢复即重新评估）；pause=true 时 SHALL NOT 触发。响应仍只反映用户意图

### Requirement: 前端健康未就绪提示

前端 SHALL 在「用户意图为运行（analysis_paused=false）但 AI 模型未就绪（NOT 健康）」时，于显著位置（顶部 banner）展示提示，告知用户 LLM/Embedding 未连通、分析暂停运行，并提供跳转至模型配置页的入口。该提示 SHALL NOT 修改或禁用既有的暂停/启动按钮。设置页 SHALL 提供「AI 健康状态」面板，展示各路由主 provider 的可达性明细、是否被后端拉起、上次检测时间，以及 `auto_start_models` 总开关。

#### Scenario: 意图运行但不健康时展示 banner

- **GIVEN** analysis_paused=false，ai_healthy=false
- **WHEN** 用户打开任意页面
- **THEN** 顶部 SHALL 显示「AI 模型未就绪（LLM/Embedding 未连通），分析暂停运行」提示，含跳转设置页入口，且暂停/启动按钮 SHALL 保持可用不被禁用

#### Scenario: 用户主动暂停时不展示健康 banner

- **GIVEN** analysis_paused=true（用户主动暂停）
- **WHEN** 模型亦 NOT 健康
- **THEN** 顶部 SHALL NOT 显示该健康未就绪 banner（用户已主动暂停，无需再提示健康）

#### Scenario: 设置页展示健康面板与总开关

- **WHEN** 用户打开设置页
- **THEN** SHALL 见「AI 健康状态」面板（各路由主 provider 通断 + 是否后端拉起 + 上次检测时间）与 auto_start_models 开关

#### Scenario: 顶部栏常驻健康指示

- **WHEN** 顶部栏渲染
- **THEN** SHALL 常驻显示当前 AI 健康状态（健康/不健康二态，如 mdi:heart-pulse 绿/红），点击跳 AI 健康设置 section；与仅在不健康时出现的 banner 并存，二者 SHALL NOT 互斥
