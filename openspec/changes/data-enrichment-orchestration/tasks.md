# Tasks — data-enrichment-orchestration

> 参考实现：`tests/data_enrichment_poc/`（Python PoC，已验证三角色 + 三防御）。
> 架构总览见 `design.md` §0（两个独立循环 + 三表认知闭环）。

## 1. 数据源与板块配置层

- [ ] 1.1 迁移：新建 `board_data_sources` 表（semantic_board_id + source_type + config + enabled + 唯一约束 + CHECK）
- [ ] 1.2 数据源工具注册表 `internal/dataenrichment/registry.go`：Tool 结构 + 注册 list_etf_by_keyword / get_etf_quote / list_sectors（Go net/http 对接东方财富/新浪，全量缓存）
- [ ] 1.3 板块配置 API + handler：`enrichment_enabled` / `window_days` / `context_layers`
- [ ] 1.4 更新 `docs/reference/database/DATABASE_FIELDS.md` 补表

## 2. 循环 A：分层新闻汇总上下文（period 档案式）

- [ ] 2.1 迁移：新建 `topic_lifeline_context` 表（persistent_topic_id + granularity + **period** + content + as_of_date + source，UNIQUE 改 `(topic_id, granularity, period)`）
- [ ] 2.2 汇总 service（`internal/dataenrichment/service/lifeline_context.go`）：**每个 period 独立汇总一条**（week/month/year 读该 period 的 sections 一次汇总；all 滚动单行增量合并）（Operation=`data_enrichment.summarize_context`）
- [ ] 2.3 定时任务：周/月/年各产独立 period 行（不覆盖旧）+ 检查自愈补缺失历史 period + 归档清理超期行（week>8周 / month>12月）
- [ ] 2.4 手动触发 API：重生成任意 period（不只是 granularity）
- [ ] 2.5 context API：按 period 列表/查看 + 人工编辑 content（前端周期筛选器翻历史）

## 3. 循环 B：三角色编排（分层上下文）

- [ ] 3.1 `internal/dataenrichment/service/orchestrator.go`：编排入口 `EnrichTopic(ctx, topicID)`，消费分层上下文
- [ ] 3.2 14天详情渲染器（对标 PoC `render_lifeline_for_agent`）：GetTopicLifeline + join daily_report_threads 取 top-2 title
- [ ] 3.3 解读员（对标 `interpret_lifeline`）：全层读表1（按 context_layers，未生成跳过）+ 14天详情 + 历史 applied review → 产业主题 JSON
- [ ] 3.4 查询员 agent loop（对标 `research_topic_evolved`）：去重拦截、命中0换宽泛词、max_loops=6、结果不截断
- [ ] 3.5 三防御落地：enable_thinking=false（airouter 请求层）、历史不截断、去重拦截
- [ ] 3.6 分析员（对标 `analyze_evolved_impact`）：分层上下文 + 数据 → evolution_assessment + **sectors（含 direction/confidence/horizon/reasoning/evidence/trigger_up/down）** + causal_chain（见 design §3.3）

## 4. 循环 B：review judge（预测兑现度复盘）

- [ ] 4.1 迁移：新建 `topic_enrichment_result` 表（快照不可变 + tool_calls jsonb，sectors 含 outlook/reasoning/evidence）+ `topic_enrichment_review` 表（prev/curr result + **verdict 兑现度结算** + deviation_summary + applied）
- [ ] 4.2 review judge service（`internal/dataenrichment/service/review_judge.go`）：增强后 LLM 对照上次预测 vs 实际走势（JSON should_review+reason+**verdict[]{sector,predicted_dir,actual,mark}**+deviation_summary），值得才写行（Operation=`data_enrichment.review_judge`）
- [ ] 4.3 applied 语义：标记采纳，**不回写表1**；下次增强解读员读 applied review
- [ ] 4.4 review API：兑现度复盘展示（hit/part/miss）+ 人工调整 deviation_summary + 采纳
- [ ] 4.5 用户手动批注 API：手动创建 review（source=manual，prev_result_id 可空，applied 默认 true）

## 5. 手动触发（仅手动，不挂日报管线）

- [ ] 5.1 手动触发增强 API：CRUD 界面"重新分析某话题"调用 EnrichTopic
- [ ] 5.2 只对 `enrichment_enabled=true` 的板块允许触发
- [ ] 5.3 增强失败只记日志告警，天然隔离（无日报阻断问题）
- [ ] 5.4 result 表移除 report_id 关联（不挂日报）；加 `input_snapshot` jsonb 字段（编排元数据可追溯）

## 6. 可观测性接线

- [ ] 6.1 所有 LLM 调用走 airouter，带 Operation（interpret / tool_use / analyze / review_judge / summarize_context）
- [ ] 6.2 SessionID 通过 context 传递（循环B: `data_enrichment_{tid}_{uuid8}`；循环A: `lifeline_context_{tid}_{gran}_{uuid8}`）
- [ ] 6.3 工具调用记录（名/参数/返回摘要/耗时）存 `topic_enrichment_result.tool_calls` jsonb
- [ ] 6.4 编排元数据（读的context层/as_of/section范围/引用review）存 `topic_enrichment_result.input_snapshot` jsonb，可追溯

## 7. 前端：「数据增强」tab 认知工作台（第一版）

> 原型见 `prototype/enrichment-workbench.html`（双主题 HTML，已迭代验证交互方向）。

- [ ] 7.1 板块详情页新增「数据增强」tab（与板块内容/日报/文章并列），按四步认知循环组织（最近怎么了→会往哪走→猜得准吗→数据源）
- [ ] 7.2 顶部 sticky 认知循环导航 + 步骤间承上启下引导条（滚动跟随高亮当前步）
- [ ] 7.3 ①新闻记忆：**周期筛选器**（粒度下拉 + period 翻页 ‹2026-06›）翻历史 + 结构化叙事段落 inline 编辑
- [ ] 7.4 ②走向预测：**板块可展开卡片**（凭什么判断[信号→机制] + 支撑数据 + 板块专属触发条件）+ **证据链 tooltip**（悬停显示 evidence.quote 原话，不跳转）
- [ ] 7.5 ③预测兑现复盘：时间轴 + 逐条兑现结算（hit/part/miss）+ 采纳 + 「已喂给下一轮解读」标记
- [ ] 7.6 术语翻译：禁用 granularity/session_id/evolution_assessment 等后端术语，用人话
- [ ] 7.7 完整交互态（loading/empty/error）+ 双主题（editorial/dark）
- [ ] 7.8 数据契约设计为侦探墙可复用（为后续重构铺路，避免返工）

## 8. 测试

后端受影响包：`internal/dataenrichment`、`internal/topicgraph`（管线挂载点）。

- `go test ./internal/dataenrichment → PASS`
  - tool registry：list_etf 返回完整命中、未知工具拒绝
  - agent loop：去重拦截、换词重查、max_loops 兜底
  - 循环A汇总：每 period 独立一条、all 滚动合并、检查自愈补历史 period
  - review judge：兑现度结算（hit/part/miss）、不回写表1
  - 三角色串通（用 mock lifeline + mock tool，不依赖外部接口）
- `go test ./internal/topicgraph → PASS`（确保数据增强只读 GetTopicLifeline，不破坏持久话题主数据）

## 9. 文档

- [ ] `docs/reference/database/DATABASE_FIELDS.md`：board_data_sources + topic_lifeline_context + topic_enrichment_result + topic_enrichment_review 四表
- [ ] `docs/reference/flow/`：数据增强认知闭环流程图（两循环 + 三表），archive 后补 §12.2 变更溯源链接
- [ ] `docs/reference/standard/backend/ai-logging.md`：补齐跟踪表新增 `data_enrichment.*` 行（含 review_judge / summarize_context）
- [ ] `backend-go/AGENTS.md`：domain 白名单加 `dataenrichment`

## 10. 验证

- `cd backend-go && go vet ./... → 零警告`
- `cd backend-go && golangci-lint run ./... → 零失败`
- `go test ./internal/dataenrichment → PASS`
- `go test ./internal/topicgraph → PASS`
- `cd backend-go && go build ./... → 成功`
- `grep -rn "data_enrichment\." backend-go/internal/dataenrichment/ → 命中 5 个 Operation`
- `grep -rn "data_enrichment_news\|data_enrichment_analysis" backend-go/internal/dataenrichment/ → 命中 2 capability`
- `grep -rn "as_of_date" backend-go/internal/dataenrichment/ → 命中（循环A时效控制落地）`
- `grep -rn "applied" backend-go/internal/dataenrichment/service/ → 命中（review 不回写表1语义落地）`
- `grep -rn "input_snapshot" backend-go/internal/dataenrichment/ → 命中（可追溯落地）`
- `grep -rn "period" backend-go/internal/dataenrichment/ → 命中（period 档案式落地）`
- `grep -rn "verdict" backend-go/internal/dataenrichment/ → 命中（兑现度结算落地）`
- `grep -rn "NextWeeklyLifelineTime\|NextMonthlyLifelineTime\|NextYearlyLifelineTime" backend-go/ → 命中 3 墙钟调度函数`
