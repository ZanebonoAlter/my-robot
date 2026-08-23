# Design: analysis-remediation

## Context

三个定时分析任务（2026-08-20，`analysis-reports/`）发现：数据库 9.3 GB 中约 5.3 GB 为垃圾/冗余（孤儿 embedding、disabled 向量、零使用索引）；openspec 编排串行等待 ~40h；openspec 指令 4 份副本；归档门禁无强制点、quota fail-open 无痕。用户确认四项全做。

现状关键事实（来自 db 报告实测）：
- `topic_tag_embeddings` 271,639 行中 254,954 行为孤儿（指向已删除 tag），~3.07 GB；GORM 声明 `OnDelete:CASCADE` 但 DB 无此 FK（全库 21 个 FK 漏了它）
- `semantic_labels` 89% 行 status=disabled，每行带 embedding + merge_embedding 两个 2560 维向量，~1.7 GB
- `idx_articles_search_vector`（GIN，185 MB）idx_scan=0 且代码零引用；otel_spans 6 索引中 5 个 idx_scan=0（~352 MB）
- `embedding_queues` completed 行不清理（当前 26 MB/2 月慢涨）
- job_log_cleanup 调度框架已存在（log-cleanup spec），可挂新清理目标

## Goals / Non-Goals

**Goals:**
- W1: 回收 ~5.3 GB（全库 -57%），并建立防复发机制（FK / 流置 NULL / 保留策略）
- W2: 开发执行规范 §0.6 编排指引支持后台并行派发，砍串行等待
- W3: openspec 指令收敛到 .pi/prompts + .agents/skills 两个机制层，删除 .claude 两份漂移副本
- W4: 归档门禁从"自觉"变"硬拦截"；quota fail-open 可观测；§12.2 断链修复

**Non-Goals:**
- 向量降精度（halfvec/MRL 截断）——需离线评估召回率，另开 change
- 文档瘦身其余项（R2 Windows cmd 规则 5 遍、R5 英文尾注等）——agent-guide-slim 后续 change
- otel 采样降频（db 报告 #7）——有 7 天保留兜底，属稳态优化另议
- 会话报告其余优化建议（检查增量化、handoff 标准化等）——仅落地最大项并行化，其余留报告备查
- test-scope-guard 硬 block 模式——soft 起步，出现实锤违规再升级

## Decisions

### D1 (W1): 清理走"SQL 维护脚本 + FK 走正式迁移"，不用单一大 migration

- 存量清理（分批 DELETE、UPDATE 置 NULL、DROP INDEX）：`scripts/db-cleanup-2026-08.sql` + 分批执行的 shell 包装（每批 ≤5 万行，批间 sleep），**先 pg_dump 备份相关表再执行**
- FK 补齐：走项目既有迁移机制（先清干净存量孤儿，否则加 FK 失败）
- 为什么不用一条大 migration：30 万行 DELETE 单事务锁表久、失败全回滚；分批可中断可观察。FK 是结构性变更必须进版本化迁移，与数据清理解耦。

### D2 (W1): 普通VACUUM 而非 VACUUM FULL

DELETE 后普通 VACUUM 将空间归还给 DB 内部复用（后续 embedding 写入不再涨盘）；VACUUM FULL 会真正还盘给宿主但需排它锁。决策：默认普通 VACUUM；如用户想立即还盘给 Docker 卷，低峰手动跑 VACUUM FULL（写入 tasks 作为可选步骤）。

### D3 (W1): GIN 只删索引 + 禁用 trigger，保留 tsvector 列

`idx_articles_search_vector` DROP；search_vector 维护 trigger 一并禁用（消除写放大）；tsvector 列保留（回滚只需重建索引+启用 trigger，分钟级）。列不删，避免表重写。

### D4 (W1): disabled 置 NULL 采用"存量 UPDATE + 流程补置 NULL"双管

存量：分批 `UPDATE semantic_labels SET embedding=NULL, merge_embedding=NULL WHERE status='disabled' AND embedding IS NOT NULL`；流程：Go 侧禁用 label 的代码路径补同样置 NULL。重新启用时 llm_extract 可再生向量（报告确认）。

### D5 (W1): embedding_queues 清理并入 job_log_cleanup

复用 log-cleanup 既有调度（Scheduled cleanup + 手动触发 + API 可观测），加一条 `DELETE ... WHERE status='completed' AND created_at < now()-30d`（带 created_at 索引确认）。不新建调度器。

### D6 (W2): 编排指引修订而非新文档

改 `docs/reference/开发执行规范.md` §0.6：六步中"派发子线程"一节明确——无依赖的任务组用 `Agent(run_in_background: true)` 并行派发，主线程期间做验收准备/文档起草，用 `get_subagent_result` 收口；保留"有依赖必须串行"的红线（如 design 未完成不能派实现）。理由：并行化是执行方式变化，不是流程结构变化，六步本身不动。

### D7 (W3): 删 .claude 两份，保留 .pi/prompts + .agents/skills

- 删：`.claude/skills/openspec-*`（9）+ `.claude/commands/opsx/`（9）——已漂移，git 历史即备份，若将来用 claude code 可 `openspec update --tools claude` 再生（ AGENTS.md 补防再生说明：更新时只带 `--tools pi`）
- 留：`.pi/prompts/opsx-*` 是 `/opsx:` 命令入口（手动调用层）；`.agents/skills/openspec-*` 是 skill 注册表（description 自动触发层）——机制不同，非重复

### D8 (W4): spec-gate.ts 复刻 quota-gate 的 tool_call 拦截模式

- 拦 `bash` 工具中 `/openspec\s+archive/` 命令 → 跑三项检查：① `scripts/doc-impact.sh verify <change-dir>` ② `scripts/check-standards.sh` ③ tasks.md 尾三节（测试/文档/验证 + doc-impact 标记）正则存在性
- 失败 → `block: true` + 中文 reason（列失败项 + 修复指引，风格同 quota-gate）；全绿放行
- 豁免：命令带 `--force` 或环境变量 `SPEC_GATE_BYPASS=1`（记 warning 不静默）
- 不跑测试（§11.1 条件 1 仍归 agent 手动 + 归档门禁兜底）——与 quality-gate 不跑 go test 同理
- 环境变量：`SPEC_GATE_ENABLE`（默认开）、`SPEC_GATE_TIMEOUT_MS`（默认 60000）

### D9 (W4): test-scope-guard.ts soft 起步

正则命中 `go test ./...`（全量）→ 查近 N 条消息语境，非归档语境 → `notify` 软提醒（不 block）。`TEST_SCOPE_GUARD=soft|hard|off` 默认 soft。理由：语境判定有假阴性，当前遵守率 100%，硬 block 的摩擦不划算。

### D10 (W4): quota-gate fail-open 补 custom_message

查询失败放行分支加一条 custom_message 落盘记录（"quota 查询失败已放行"+原因），复用 quality-gate 失败落盘的既有模式，把"没触发/静默失败"变成可审计。

## Risks / Trade-offs

- [30 万行 DELETE 锁表/长事务] → 分批 ≤5 万 + 批间 sleep + 低峰执行 + 先备份（pg_dump 三张相关表）
- [FK 加不上（遗漏孤儿）] → 顺序保证：清孤儿 → 复核 count=0 → 再加 FK；FK 迁移失败自动回滚无副作用
- [disabled 置 NULL 后用户重新启用标签需重算] → 可接受（llm_extract 再生）；在完工汇报中明确告知
- [DROP 索引后偶发按 trace_id 排障退化为顺序扫（231 万行约秒级）] → 保留 pkey+start_time；报告确认当前零扫描
- [spec-gate 误 block（check-standards 算进其他 active change 脏文件）] → verify 已支持 excuse 豁免；reason 中提示；BYPASS 通道兜底
- [删 .claude 后若再用 claude code 缺指令] → `openspec update --tools claude` 一键再生；git 可恢复
- [禁用 trigger 后将来要做全文搜索] → 列保留，重建索引分钟级

## Migration Plan (W1 数据部分)

1. 停相关写入不现实（单用户系统，选低峰即可）；`pg_dump` 备份 topic_tag_embeddings / semantic_labels / articles 元数据
2. 复核孤儿 count（重跑报告 SQL，确认数量级一致）
3. 分批清孤儿 embedding → 复核 → FK 迁移（AutoMigrate/显式迁移）→ 验证 FK 生效（删一个测试 tag 看级联）
4. 分批 disabled 置 NULL → Go 禁用流程补置 NULL 代码 + 测试
5. DROP GIN 索引 + 禁 trigger；DROP otel 4 索引
6. 普通VACUUM（可选 VACUUM FULL 由用户决定）
7. embedding_queues 挂 job_log_cleanup + 测试
- 回滚：数据类操作靠备份恢复；FK/DROP INDEX 均可反向迁移脚本恢复；代码回滚走 git

## Open Questions

（无——四项方案均已获报告证据支持，用户已确认全做；向量降精度明确排除待后续 change）
