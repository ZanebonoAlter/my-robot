## MODIFIED Requirements

### Requirement: 日报数据模型
**DailyReportSection 字段变更**：移除 `prev_section_id` 和 `status` 字段。Section 之间的跨天关系通过 `daily_report_section_relations` 关系表表达，status 通过关系拓扑动态推导（见 section-relations spec）。

**DailyReportThread 字段变更**：移除 `status` 和 `prev_thread_id` 字段。Thread 简化为叙事下的具体事件条目，不再跨天追踪（见 thread-storage spec）。

#### Scenario: Section 无 status 和 prev_section_id 字段
- **WHEN** 日报生成器保存 section 到数据库
- **THEN** section 记录 SHALL 不包含 status 和 prev_section_id 列，跨天关系通过关系表存储

#### Scenario: Thread 无 status 和 prev_thread_id 字段
- **WHEN** 日报生成器保存 thread 到数据库
- **THEN** thread 记录 SHALL 不包含 status 和 prev_thread_id 列

### Requirement: 日报分段并行生成
系统 SHALL 并行执行两类 LLM 生成调用：
- **Call A（今日重点）**：输入全部标签(label+desc+article_count) + 昨日日报，输出 2-3 个重点项（含标题、选择理由、关联标签 ID）
- **Call C×K（聚类叙事线索）**：每个聚类一次调用，输入该聚类标签，输出 0-N 条线索（仅 title + summary + tag_ids + confidence）

Call C 生成完成后，系统 SHALL 将线程作为 `daily_report_threads` 表的行持久化。

`GenerateDailyReport` 函数 SHALL 返回 `(*BoardDailyReport, []DailyReportSection, [][]DailyReportThread, error)`，其中第三项为每个 cluster 对应的 `[]DailyReportThread` 列表。

Thread 生成 SHALL 移除 status、prev_thread_id 相关的 prompt 要求和 JSON schema 字段。Thread 的 system prompt SHALL 简化为仅要求输出 title、summary、tag_ids、confidence。

prompt version 升级为 "3.0"。

#### Scenario: 并行生成成功
- **WHEN** 有 5 个聚类
- **THEN** 系统 SHALL 同时发起 Call A + Call C×5，共 6 个并行 LLM 调用

#### Scenario: Thread 输出不含 status
- **WHEN** Call C 为某聚类生成 3 条 thread
- **THEN** 每条 thread SHALL 只包含 title、summary、tag_ids、confidence，不包含 status 和 prev_thread_id

#### Scenario: 昨日日报不存在
- **WHEN** 某板某日为首次生成日报
- **THEN** Call A 的"昨日日报"输入 SHALL 为空，Call C 不传入任何历史线索上下文（已移除 getPrevThreadSummaries 调用）

### Requirement: 日报生成编排流水线
系统 SHALL 提供 `GenerateDailyReport(ctx, boardID, date)` 编排函数，按顺序执行：收集板内事件标签 → 质量筛选 → 去重 → LLM 分组(带组数限制) → 查询昨日日报 → 并行生成(Call A + C×K) → section embedding 生成 → **同日 section 两阶段合并** → section embedding 匹配写入关系表 → 组装 BoardDailyReport + DailyReportSection(含 best_tier/avg_score) → 存储。

流水线 SHALL NOT 执行 thread 级别的 tag 交集匹配或 prev_thread_id 赋值。

#### Scenario: 完整流水线执行
- **WHEN** 触发 SemanticBoard #5 在 2026-05-25 的日报生成
- **THEN** 系统 SHALL 按序执行：收集标签 → 质量筛选 → 去重 → LLM分组 → 查询昨日日报 → 并行生成 → embedding 生成 → 同日合并 → section embedding 匹配写入关系表 → 组装存储 → status="done"

#### Scenario: 生成失败
- **WHEN** 流水线中任一步骤失败（如 LLM 调用超时）
- **THEN** 系统 SHALL 设置 status="failed"，保留已完成的中间结果，WS 广播失败状态

### Requirement: 日报查询 API
系统 SHALL 提供以下查询端点：
- `GET /api/semantic-boards/:id/daily-reports?days=7`：查询该 board 最近 N 天的日报列表
- `GET /api/daily-reports/:id`：查询单篇日报详情（含关联 sections，每个 section 通过 GORM Preload 包含 threads 列表）
- `GET /api/semantic-boards/:id/section-timeline?days=14`：查询板块 section 时间线（含关系）
- `GET /api/daily-reports/:id`：查询单篇日报详情（section 不含 status/prev_section_id，thread 不含 status/prev_thread_id）

移除端点：
- ~~`GET /api/daily-reports/threads/:id/lineage`~~
- ~~`GET /api/semantic-boards/:id/thread-timeline`~~

#### Scenario: 查询板块日报列表
- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=7`
- **THEN** 系统 SHALL 返回 board #5 最近 7 天的日报列表

#### Scenario: 查询日报详情含线程
- **WHEN** 请求 `GET /api/daily-reports/42`
- **THEN** 系统 SHALL 返回日报 #42 完整内容，每个 section 包含 threads 列表（每条含 id、title、summary、tag_ids、confidence、related_article_ids），section 和 thread 均不含 status/prev_*_id 字段

## REMOVED Requirements

### Requirement: Thread 级别连续性匹配（基于 tag 交集）
**Reason**: Thread 不再跨天追踪，lineage 追踪上移到 Section 级关系表
**Migration**: 关系表 `daily_report_section_relations` 替代 thread 级 tag 交集匹配

### Requirement: Section embedding 语义延续匹配（单链 prev_section_id）
**Reason**: 替换为多对多关系表，匹配结果写入 `daily_report_section_relations`
**Migration**: `prev_section_id` 列删除，关系通过 `daily_report_section_relations` 表表达
