# Tasks — data-enrichment-orchestration

> 参考实现：`tests/data_enrichment_poc/`（Python PoC，已验证三角色 + 三防御）。
> 架构总览见 `design.md` §0（两个独立循环 + 三表认知闭环）。

## 1. 数据源与板块配置层

- [ ] 1.1 迁移：新建 `board_data_sources` 表（semantic_board_id + source_type + config + enabled + 唯一约束 + CHECK）
- [ ] 1.2 数据源工具注册表 `internal/dataenrichment/registry.go`：Tool 结构 + 注册 list_etf_by_keyword / get_etf_quote / list_sectors（Go net/http 对接东方财富/新浪，全量缓存）
- [ ] 1.3 板块配置 API + handler：`enrichment_enabled` / `window_days` / `context_layers`
- [ ] 1.4 更新 `docs/reference/database/DATABASE_FIELDS.md` 补表

## 2. 循环 A：分层新闻汇总上下文

- [ ] 2.1 迁移：新建 `topic_lifeline_context` 表（persistent_topic_id + granularity + content + as_of_date + source + 唯一约束）
- [ ] 2.2 汇总 service（`internal/dataenrichment/service/lifeline_context.go`）：week 直接读最近7天重算；month/year/all 用「增量 sections + 旧汇总」LLM 合并（Operation=`data_enrichment.summarize_context`）
- [ ] 2.3 定时任务：周/月/年刷新 + 检查自愈（从 as_of_date 次周期起逐块补遗漏，非覆盖当前）
- [ ] 2.4 手动触发 API：重生成某 granularity
- [ ] 2.5 context CRUD API + 人工编辑 content

## 3. 循环 B：三角色编排（分层上下文）

- [ ] 3.1 `internal/dataenrichment/service/orchestrator.go`：编排入口 `EnrichTopic(ctx, topicID)`，消费分层上下文
- [ ] 3.2 14天详情渲染器（对标 PoC `render_lifeline_for_agent`）：GetTopicLifeline + join daily_report_threads 取 top-2 title
- [ ] 3.3 解读员（对标 `interpret_lifeline`）：全层读表1（按 context_layers，未生成跳过）+ 14天详情 + 历史 applied review → 产业主题 JSON
- [ ] 3.4 查询员 agent loop（对标 `research_topic_evolved`）：去重拦截、命中0换宽泛词、max_loops=6、结果不截断
- [ ] 3.5 三防御落地：enable_thinking=false（airouter 请求层）、历史不截断、去重拦截
- [ ] 3.6 分析员（对标 `analyze_evolved_impact`）：分层上下文 + 数据 → evolution_assessment/sectors/causal_chain

## 4. 循环 B：review judge

- [ ] 4.1 迁移：新建 `topic_enrichment_result` 表（快照不可变 + tool_calls jsonb）+ `topic_enrichment_review` 表（prev/curr result + deviation_summary + applied）
- [ ] 4.2 review judge service（`internal/dataenrichment/service/review_judge.go`）：增强后 LLM 半自动对比（JSON should_review+reason+deviation_summary+affected_context），值得才写行（Operation=`data_enrichment.review_judge`）
- [ ] 4.3 applied 语义：标记采纳，**不回写表1**；下次增强解读员读 applied review
- [ ] 4.4 review CRUD API + 人工调整 deviation_summary
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

## 7. 前端：板块 tab CRUD 界面（第一版）

- [ ] 7.1 板块详情页新增「数据增强」tab（与板块内容/日报/文章并列）
- [ ] 7.2 表1 context 面板：查看 + 手动重生成（单 granularity）+ 编辑 content
- [ ] 7.3 表2 result 面板：查看（含 LLM 调用 trace，点 session_id 回放 ai_call_logs）+ 手动触发增强
- [ ] 7.4 表3 review 面板：查看认知演进史 + 编辑 deviation_summary + 采纳(applied)
- [ ] 7.5 数据契约设计为侦探墙可复用（为后续重构铺路，避免返工）

## 8. 测试

后端受影响包：`internal/dataenrichment`、`internal/topicgraph`（管线挂载点）。

- `go test ./internal/dataenrichment → PASS`
  - tool registry：list_etf 返回完整命中、未知工具拒绝
  - agent loop：去重拦截、换词重查、max_loops 兜底
  - 循环A汇总：week 直接重算、month 增量合并
  - review judge：should_review=true/false 判断、不回写表1
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
- `grep -rn "enable_thinking" backend-go/internal/dataenrichment/ → 命中`
- `grep -rn "Operation:" backend-go/internal/dataenrichment/ → 命中 interpret/tool_use/analyze/review_judge/summarize_context`
- `grep -rn "as_of_date" backend-go/internal/dataenrichment/ → 命中（循环A时效控制落地）`
- `grep -rn "applied" backend-go/internal/dataenrichment/service/review_judge.go → 命中（不回写表1语义落地）`
- `grep -rn "input_snapshot" backend-go/internal/dataenrichment/ → 命中（可追溯落地）`
