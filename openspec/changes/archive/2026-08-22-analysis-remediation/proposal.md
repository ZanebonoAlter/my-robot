# Proposal: analysis-remediation

## Why

2026-08-20 凌晨三个定时分析任务产出了报告（`analysis-reports/` 三份），发现了四类高价值问题，用户确认全部处理：

1. **数据库存储浪费严重（db-analysis 报告）**：全库 9.3 GB 中约 5.3 GB 可回收。最大两项：`topic_tag_embeddings` 93.8% 孤儿行（~3.07 GB，GORM 声明了 `OnDelete:CASCADE` 但 DB 实际无此 FK，删 tag 向量全部残留）；`semantic_labels` 89% disabled 行仍各带 2 个 2560 维全精度向量（~1.7 GB）。另有零使用索引（articles GIN 全文索引 185 MB、otel_spans 4 个索引 352 MB）与 `embedding_queues` 无保留策略慢涨。
2. **开发流程耗时可压缩（pi-sessions-analysis 报告）**：openspec change 编排中"派发子线程→阻塞干等→验收"的串行模式占 ~40h 纯等待，是活跃时间的最大可压缩块（预期省 12-20h/季度）。
3. **openspec 工作流指令 4 份副本冗余（harness-docs-analysis 报告 R1）**：`.pi/prompts/` + `.agents/skills/` + `.claude/skills/` + `.claude/commands/` 各 9 份近乎相同，`.claude/` 版已实际漂移。
4. **规范遵守"不可观测"（harness-docs-analysis 报告）**：openspec 走 change ≈95%、测试分层 ≈100%，但归档门禁（doc-impact verify + check-standards）纯靠自觉无强制点；quota-gate fail-open 无痕；§12.2 溯源表存在断链（scheduler-cron 归档不存在）。

## What Changes

四个工作流（workstream）：

### W1: 数据库存储清理（预期 9.3 GB → ~4.0 GB，-57%）

- 清 `topic_tag_embeddings` 孤儿行（分批 DELETE + VACUUM），并补 `FK ... ON DELETE CASCADE` 防复发（存量清干净后加）
- `semantic_labels` disabled 行向量置 NULL（保留行本体与 aliases）；禁用流程代码补置 NULL 逻辑
- DROP `idx_articles_search_vector`（业务零引用双证据：代码 grep + idx_scan=0）
- DROP otel_spans 零使用索引 ×4（trace_id/kind/status/name，共 ~352 MB），保留 pkey + start_time
- `embedding_queues` completed > 30 天清理，并入现有 job_log_cleanup 调度
- 同步更新 docs/reference/database/ 文档

不包含：向量降精度（halfvec/MRL，db 报告建议 #5，风险中-高需单独评估召回率，另开 change）。

### W2: 子代理后台并行编排指引

- 修订 `docs/reference/开发执行规范.md` §0.6 编排六步：独立任务组用 background 派发，主线程期间做验收准备/文档，`get_subagent_result` 收口；明确哪些步骤可并行、哪些必须串行（依赖前序产物的任务）

### W3: openspec 指令去重

- 删除 `.claude/skills/openspec-*`（9 份）与 `.claude/commands/opsx/`（9 份）——已漂移的重复副本
- 保留 `.pi/prompts/opsx-*`（pi 原生命令入口）与 `.agents/skills/openspec-*`（skill 注册表自动触发层），两者机制不同不属重复
- 根 AGENTS.md 补一句防再生说明（防止 `openspec update --tools claude` 再生成）

### W4: 合规观测与归档硬门禁

- 新增 `.pi/extensions/spec-gate.ts`：拦截 `openspec archive` 命令，归档前强制跑 doc-impact.sh verify + check-standards.sh + tasks.md 尾三节检查，失败 block（豁免：`SPEC_GATE_BYPASS=1`，记 warning 不静默）
- 新增 `.pi/extensions/test-scope-guard.ts`（soft 模式）：检测全量 `go test ./...` 非归档语境时 notify 软提醒（不 block）
- 改 `quota-gate.ts`：fail-open 放行时补落 custom_message（消除"没触发/静默失败"不可区分）
- 修 §12.2 溯源表断链（scheduler-cron 归档链接指向不存在的条目）

## Capabilities

### New Capabilities

（无——全部挂已有 capability）

### Modified Capabilities

- `tag-embedding-management`: 新增"tag 删除时 embedding 级联清理（DB FK 兜底）+ 存量孤儿一次性清理"需求（当前 spec 只有"保存新 embedding 清理旧记录"，无删除侧约束）
- `semantic-label-model`: 新增"disabled 标签不保留向量（禁用时置 NULL，重新启用由 llm_extract 重算）"需求
- `log-cleanup`: 新增 `embedding_queues` completed 保留策略（30 天），并入现有 scheduled cleanup
- `doc-impact-gate`: 新增"归档命令硬门禁"需求（spec-gate.ts 拦截 + 三项检查 + 豁免通道）
- `agent-guide-slim`: 新增"openspec 指令单一权威源、禁止 .claude 副本再生"需求
- `development-docs`: 新增"开发执行规范 §0.6 编排含后台并行派发指引"需求

## Impact

- **backend-go/**：semantic_labels 禁用流程加置 NULL；topic_tag 删除路径核对（FK 建立后 GORM 声明与 DB 一致）；job_log_cleanup 加 embedding_queues 清理；可能涉及迁移文件（FK）
- **数据库（需用户部署后操作/确认）**：
  - 一次性清理 SQL 在迁移或维护脚本中执行（分批 + 备份先行）
  - **数据影响**：孤儿 embedding 删除不可逆（但其指向的 tag 已不存在，无业务损失）；disabled 标签向量置 NULL 后重新启用需 llm_extract 重算（可再生）
  - DROP 索引后若将来需要全文搜索/trace 查询需重建（分钟级）
- **.pi/extensions/**：新增 spec-gate.ts、test-scope-guard.ts；修改 quota-gate.ts
- **文档**：AGENTS.md、开发执行规范.md（§0.6 并行指引、§12.2 断链修复）、docs/reference/database/、flow/ 变更溯源
- **删除**：`.claude/skills/openspec-*`、`.claude/commands/opsx/`（共 18 个文件）
