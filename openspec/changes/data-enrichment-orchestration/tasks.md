# Tasks — data-enrichment-orchestration

> 参考实现：`tests/data_enrichment_poc/`（Python PoC，已验证三角色 + 三防御）。
> 架构总览见 `design.md` §0（两个独立循环 + 三表认知闭环）。

> **进度图例**：`[x]` 已实现且符合制品 ｜ `[ ]` 未实现 ｜ `[~]` ★ 已实现但**重定位待重做**（旧走向预测语义，需改为演进定位）｜ `[f]` 冻结（FinGenius，保留不发展）
> 2026-07-09 重定位：主线从「金融走向预测 + 涨跌兑现」拉回到「持久话题演进定位」。骨架保留；分析员输出 / review / 触发 / 前端报告 需重做（见 §11）。FinGenius 冻结。
> 原型方向已验证：`prototype/evolution-report.html`（报刊式演进分析报告 + 双类引用）。

## 1. 数据源与板块配置层

- [x] 1.1 迁移：新建 `board_data_sources` 表（semantic_board_id + source_type + config + enabled + 唯一约束）✅ GORM model 已建（`uniqueIndex:idx_board_src`）
- [x] 1.2 数据源工具注册表（`internal/dataenrichment/service/tool_registry.go`）：注册 list_etf_by_keyword / get_etf_quote / list_sectors（全量缓存用 mutex+loaded flag）
- [ ] 1.3 板块配置 API + handler：`enrichment_enabled` / `window_days` / `context_layers`
- [ ] 1.4 更新 `docs/reference/database/DATABASE_FIELDS.md` 补表（归入 §9 文档批次）

## 2. 循环 A：分层新闻汇总上下文（period 档案式）✅ 不变

- [x] 2.1 迁移：`topic_lifeline_context` 表加 `period` 字段（'2026-W27'/'2026-06'/'2026'/'all'），UNIQUE 改 `(topic_id, granularity, period)`（migration `20260706_0001` DROP 旧索引 + TRUNCATE 旧数据）
- [x] 2.2 汇总 service（`lifeline_context.go`）：每个 period 独立汇总一条（week/month/year 读该 period 的 sections；all 滚动单行）+ `period.go` 格式化函数
- [x] 2.3 定时任务：周/月/年各产独立 period 行 + 检查自愈（`HealMissing`）+ 归档清理（`ArchivePrune`，week>8周/month>12月）
- [x] 2.4 手动触发 API：指定 period 重生成
- [x] 2.5 context API：按 period 列表/查看 + 人工编辑
- [x] 2.6 orchestrator `readContextLayers`：按 granularity 取 MAX(period) 最新那条

## 3. 循环 B：三角色编排（分层上下文）— 骨架保留，分析员输出待重做

- [x] 3.1 `orchestrator.go`：`EnrichTopic(ctx, topicID)`，消费分层上下文
- [x] 3.2 14天详情渲染器（`lifeline_renderer.go`）
- [x] 3.3 解读员：全层读表1 + 14天详情 + 历史 applied review → 产业主题 JSON
- [x] 3.4 查询员 agent loop：去重拦截、命中0换宽泛词、max_loops=6、结果不截断
- [x] 3.5 三防御：`/no_think` 双保险 + 全量历史 + 去重（enable_thinking 走 DB provider 配置）
- [~] 3.6 分析员：★ **走向预测 sectors schema 待重做为演进定位**。旧实现输出 `sectors[]{direction/confidence/horizon/reasoning/evidence/trigger_up/trigger_down/symbols}`（涨跌语义）→ 需改为 `position`(强化/转折/扩散/衰减) + `signals`(跨泳道) + `evidence`(加 source_type) + 可选 `financial_view`（见 §11.2）

## 4. 循环 B：review judge — 待重做为定位变化对比

- [x] 4.1 `topic_enrichment_result` 表（快照不可变 + tool_calls + input_snapshot，无 report_id）+ `topic_enrichment_review` 表（含 jsonb）
- [~] 4.2 review judge service：★ **涨跌兑现 verdict 待重做为定位变化对比**。旧实现输出 `verdict[]{sector,predicted_dir,actual,mark=hit|part|miss}`（涨跌兑现）→ 需改为 `position_change{from,to,summary}` + `change_summary`（见 §11.3）
- [x] 4.3 applied 语义：不回写表1；下次增强读 applied review
- [x] 4.4 review API：后端已实现（`listReviews` / `updateReviewDeviation` / `applyReview`，不回写表1）；★ 前端 verdict 渲染待重做为定位迁移展示
- [x] 4.5 用户手动批注 API：手动创建 review（source=manual，applied 默认 true）✅ 后端已实现（`createReview`，prev_result_id 可空）

## 4b. 个股深度辩论（外部 FinGenius · [f] 冻结 · 2026-07-09 降级）

> ★ 降级：FinGenius 个股涨跌辩论与演进定位主线冲突。代码冻结、前端④默认折叠、标"金融可选模块·独立于演进主线"、不再作为主线发展。以下实现已完成，原样保留备查。

- [f] 4b.1 分析员 sectors schema 扩展 `symbols`（代表标的池，kind=etf/leader_stock，不带买卖建议）
- [f] 4b.2 `stock_debate_result` 表（提炼字段 verdict/consensus/agents/votes + 原始字段 fingenius_research/fingenius_battle/fingenius_task_id + distill_status + html_content），append-only，不回写表1/2/3
- [f] 4b.3 FinGenius HTTP 客户端（`fingenius_client.go`）：POST /analyze 提交 + GET /task/{id} 轮询 + GET /health，读 5 个 `FINGENIUS_*` env，GPL 进程隔离不引入其代码
- [f] 4b.4 debate_distill 提炼 service（`debate_distill.go`）：原始输出 → verdict/consensus/agents/votes（Operation=`data_enrichment.debate_distill`），失败降级 distill_status=failed 展示原文
- [f] 4b.5 独立端点 `POST/GET .../results/:id/debates`（前端按钮触发，失败 non-fatal 不阻塞主分析）
- [f] 4b.6 单元测试 36 个（distill 15 + client 12 + service 9）+ 修复 PollTask nil panic bug
- [ ] 4b.7 ★ 前端 `DebateSection.vue` 默认折叠 + 标注"金融可选模块·独立于演进主线"（降级处理）

## 5. 手动触发（仅手动，★ 话题级跨版块）

- [x] 5.1 手动触发增强 API 调用 EnrichTopic
- [~] 5.2 ★ **触发从单板块扩到话题级跨版块**：话题命中版块只要一个 enabled 就允许触发；工具按 enabled 版块动态注册（见 §11.1）
- [x] 5.3 增强失败只记日志，天然隔离
- [x] 5.4 result 表移除 report_id；加 `input_snapshot` jsonb

## 6. 可观测性接线 ✅

- [x] 6.1 所有 LLM 调用走 airouter，带 Operation（interpret / tool_use / analyze / review_judge / summarize_context / debate_distill 共 6 个）
- [x] 6.2 SessionID 通过 context 传递（循环B: `data_enrichment_{tid}_{uuid8}`；循环A: `lifeline_context_{tid}_{gran}_{uuid8}`；辩论: `data_enrichment_debate_{tid}_{rid}`）
- [x] 6.3 工具调用记录存 `topic_enrichment_result.tool_calls` jsonb
- [x] 6.4 编排元数据存 `topic_enrichment_result.input_snapshot` jsonb

## 7. 前端 — ★ 重定位重做

> 2026-07-09 重定位：前端从「认知工作台四区块（走向预测+兑现复盘）」改为「演进分析报告（报刊式）+ 新闻背景（独立）」。原型 `prototype/evolution-report.html` 已验证方向。

- [x] 7.1 TagsPage「数据增强」tab 已挂 BoardEnrichmentPanel ✅（保留为新闻背景入口）
- [x] 7.2 顶部 sticky 认知循环导航（保留旧工作台管理面板；演进报告用新入口，不强求 sticky 导航）
- [x] 7.3 ①新闻记忆（循环A独立产物）：周期筛选器翻历史 + inline 编辑 ✅（保留）
- [~] 7.4 ★ **走向预测卡片待重做为演进分析报告**：旧实现 result detail dialog 渲染新 sectors（涨红跌绿/置信度/触发）→ 需改为报刊式长文报告（见 §11.4）
- [~] 7.5 ★ **预测兑现复盘待重做为定位变化展示**：旧实现 review verdict（hit绿/part黄/miss红）→ 需改为定位迁移（prev→curr + change_summary）
- [x] 7.6 术语翻译用人话 ✅（演进定位：强化/转折/扩散/衰减；跨泳道信号；定位变化）
- [x] 7.7 完整交互态 + 双主题 ✅（hover/disabled/loading 态齐全；CSS 变量 var(--color-*) 自动适配明暗主题）
- [x] 7.8 数据契约为侦探墙可复用 ✅（boardEnrichment.ts 注释明示契约设计为侦探墙复用）
- [f] 7.9 ④个股深度辩论区块（DebateSection.vue）✅ 已实现；★ 待改默认折叠 + 降级标注（见 4b.7）

## 8. 测试 — ★ 重定位后需补

- `go test ./internal/dataenrichment → PASS`（4 子包全绿，含 36 个 FinGenius 测试 [f] 冻结保留）
- ★ 重定位后需补：演进定位 schema 测试（§11.2）、定位变化对比测试（§11.3）、跨版块触发测试（§11.1）
- `go test ./internal/topicgraph → PASS`（全量门禁无回归）

## 9. 文档

- [ ] `docs/reference/database/DATABASE_FIELDS.md`：board_data_sources + topic_lifeline_context + topic_enrichment_result + topic_enrichment_review + stock_debate_result 五表
- [ ] `docs/reference/flow/`：演进定位认知闭环流程图（两循环 + 三表 + 演进定位主线 + FinGenius 冻结），archive 后补 §12.2 变更溯源链接
- [ ] `docs/reference/standard/backend/ai-logging.md`：补齐 `data_enrichment.*` 跟踪表（含 review_judge / summarize_context / debate_distill）
- [x] `backend-go/AGENTS.md`：domain 白名单已含 `dataenrichment`

## 10. 验证（重定位前的旧验证，部分语义已变）

- `go vet ./... → 零警告` ✅
- `go build ./... → 成功` ✅
- `go test -short ./... → 全包 PASS` ✅（2026-07-06 全量门禁，重定位前）
- ★ 重定位后需重跑：`grep -rn "position\|signals\|position_change" backend-go/internal/dataenrichment/`（演进定位落地校验）
- ★ 重定位后需重跑：`grep -rn "evolution-report\|source_type"`（前端报告 + 双类引用落地校验）
- 旧校验（重定位后失效，仅存档）：
  - `grep -rn "verdict" → 命中`（review verdict + stock_debate_result）— 重定位后 review verdict 改 position_change
  - `grep -rn "sectors\|direction\|trigger" → 命中`（走向预测 schema）— 重定位后改 position/signals

## 11. ★ 演进定位重做批次（2026-07-09 新增）

> 主线重定位的代码改动。骨架（三角色编排 / 三表分离 / agent loop / 可观测性 / 循环A）保留，换的是"分析员和 review 里填的判断目标" + 触发跨版块化 + 前端报告重做。

### 11.1 触发话题级跨版块（部分完成；跨版块遍历暂缓）

> ★ 2026-07-12 核实：持久话题**不跨版块**（单归属 semantic_board_id），且当前**无跨版块话题关联机制**（`SectionRelation` 仅版块内部，`RebuildBoardRelations` 按单 board 重建）。故 11.1.2（工具动态注册）已落地、有实效；11.1.1（跨版块关联遍历）需先建跨版块话题关联数据模型，标记**暂缓**，留作后续独立 change。

- [⏸] 11.1.1 `EnrichTopic` 入口放开跨版块：查该话题命中的所有 SemanticBoard，过滤 enabled 的 —— **暂缓**：需先建跨版块话题关联机制（当前话题单归属、关系在版块内），属独立数据模型工程
- [x] 11.1.2 工具按版块数据源动态注册：`board_data_sources(enabled=true)` → `ToolsForSourceType` → `AllowedTools` 填进 `BoardEnrichmentConfig`，runAgentLoop 只描述/执行允许的工具（`service/board_config.go` + `board_config_impl.go` + `orchestrator.go` runAgentLoop 门禁）。非金融版块不再被塞行情工具
- [x] 11.1.3 工具动态注册测试：`TestToolsForSourceType` + `TestEnrichTopic_OnlyAllowedToolsAdvertised` + `TestEnrichTopic_EmptyAllowedToolsNoToolsAdvertised`

### 11.2 分析员演进定位 schema

- [x] 11.2.1 分析员 prompt 重写：角色改为「产业演进分析师」，输出 position 四档（强化/转折/扩散/衰减）
- [x] 11.2.2 输出 schema 重写：`position` + `signals`(跨泳道，lane 用持久话题泳道名) + `evidence`(加 `source_type: news|tool`) + 可选 `financial_view`（`analyzeOutput` struct）
- [x] 11.2.3 `topic_enrichment_result` 存储适配：复合对象 `{position,signals,evidence,financial_view}` 存进 `Sectors` jsonb 列（免 DDL，旧数据由 11.5 清）
- [x] 11.2.4 演进定位 schema 测试：`TestAnalyze_EvolutionPositionSchema`（含 financial_view 缺失子测）

### 11.3 review 定位变化对比

- [x] 11.3.1 review judge prompt 重写：从"涨跌兑现 hit/part/miss"→"定位变化对比（prev.position → curr.position）"
- [x] 11.3.2 输出 schema 重写：`position_change{from,to,summary}` + `change_summary`（`ReviewJudgeOutput` struct）
- [x] 11.3.3 `topic_enrichment_review` 存储适配：position_change 存进 `Verdict` jsonb 列、change_summary 存进 `DeviationSummary` text 列（免 DDL）；加 `extractPosition` 旧格式 prev 守卫（无 position 跳过 review）
- [x] 11.3.4 定位变化对比测试：`TestRunReviewJudge_PositionChangeSchema` + `TestEnrichTopic_SkipReviewOnOldFormatPrev`

### 11.4 前端演进分析报告

- [ ] 11.4.1 报刊式长文报告组件（基于 `prototype/evolution-report.html`）：单栏、衬线、drop-cap、双线 masthead
- [ ] 11.4.2 双类引用渲染：📰新闻 `[1]`红 / 🔧工具 `[T1]`蓝（吃 evidence.source_type）
- [ ] 11.4.3 跨泳道关联叙事：泳道名标签 + 传导机制自然语言写进正文
- [ ] 11.4.4 文末资料来源分两组（新闻报道 / 工具查证）
- [ ] 11.4.5 演进定位小结条（position 四档 + 迁移说明）
- [ ] 11.4.6 报告前端测试

### 11.5 数据清理

- [x] 11.5.1 重定位前的旧 result（sectors 涨跌语义）和旧 review（verdict 涨跌兑现）清空重跑（不可复用）—— 迁移 `20260712_0001` 在下次迁移运行（部署/服务启动）时执行 TRUNCATE，重跑随下次增强自动产生
- [x] 11.5.2 清理迁移脚本（幂等，TRUNCATE topic_enrichment_result / topic_enrichment_review / stock_debate_result，CASCADE + RESTART IDENTITY）—— `postgres_migrations.go` 版本 `20260712_0001`
