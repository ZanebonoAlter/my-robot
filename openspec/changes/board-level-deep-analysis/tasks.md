# Tasks: board-level-deep-analysis

## 1. 数据模型与迁移

- [ ] 1.1 `topic_enrichment_result` 加列：`semantic_board_id *uint`（NULL=单泳道）、`analysis_scope string`（默认 `topic`）、`persistent_topic_id` 放开非空约束；新表 `reference_roles`。写迁移并验证：`go test ./internal/platform/database`（迁移测试）通过 + 旧数据 `analysis_scope` 全为 `topic`（抽查 SQL）
- [ ] 1.2 repository 层适配：版块档 result 的写入/列表/详情查询（按 board_id 过滤）+ 参考角色 CRUD repository。验证：repository 单测覆盖 scope 过滤与参考角色增删改查

## 2. 参考角色库（后端 + 注入）

- [ ] 2.1 参考角色 handler + 路由（`/reference-roles` CRUD + 启停），enforced：停用即时生效（每次注入现查 DB）。验证：handler 单测（创建/启停/删除）通过
- [ ] 2.2 orchestrator prompt 装配处统一注入函数：enabled 文档按 `updated_at DESC` 注入三角色 system prompt，总量上限 ~4k 字符截断 + 截断留痕日志；零文档零影响。验证：单测（注入格式/超限截断/空库行为）
- [ ] 2.3 手工录入首份《内部看美国·方法论画像》（内容取 `docs/research/board-analysis-reference-role/` 探索产出）。验证：库中可查询到 enabled 文档

## 3. 版块级分析编排

- [ ] 3.1 态势卡装配器：每活跃泳道一卡（lifeline week 降级事实指纹 + 命中统计 + 质量信号：活跃度/密度/sparse 历史），质量信号查询期计算不落库，按质量排序控制详略。验证：单测（多泳道排序/稀疏降级/无 lifeline 兜底）
- [ ] 3.2 新 Operation `data_enrichment.board_interpret`：输入态势卡集合 + 参考角色 + 板块级历史 applied review，输出 candidates（钩子×切角）+ chosen + reason；素材稀薄诚实降级。验证：orchestrator 单测（候选 JSON 解析/稀薄板块 sparse 路径）
- [ ] 3.3 版块形态的 tool_use/analyze prompt 分支：论证骨架=层级递进机制层、lane 论据织入主路径（内部工具引导优先级 ≥ 外部检索）、跨泳道素材非并列罗列。SessionID `data_enrichment_board_{board_id}_{uuid8}`。验证：单测（prompt 装配含版块分支）+ max_loops 防御复用断言
- [ ] 3.4 `EnrichBoard(boardID)` 编排入口：enrichment_enabled 门槛、result 写入（scope=board，sectors 载 `{thesis, candidates, argument, depth, lane_refs}`）、完成后自动 review judge（对比上一份板块报告）。验证：集成测试（DB）：两连触发产出两份独立快照 + 第二份自动 review，review 不回写 lifeline
- [ ] 3.5 evidence `source_type` 扩 `lane`：解析/校验（lane_id 属本板块活跃集合）+ 旧枚举零影响。验证：单测（lane 证据解析/幽灵引用拒绝/news|web|page 回归）

## 4. API 与前端

- [ ] 4.1 版块分析 API：`POST /semantic-boards/:id/enrichment/analysis/trigger`、`GET .../analysis/results`（列表/详情）；单泳道 trigger 加可选 `{prefill_lens}`。验证：handler 单测（未开启板块拒绝/触发落 result/prefill_lens 透传）
- [ ] 4.2 前端 `BoardEnrichmentPanel` 重排：版块分析主视图（最新报告+触发+历史）、单泳道下拉收拢「聚焦分析」折叠区。验证：`pnpm lint` + `pnpm exec nuxi typecheck`（Windows cmd）通过
- [ ] 4.3 版块报告渲染组件：thesis/candidates/argument（机制层+证据）/depth/lane_refs；lane 引用点击 → 聚焦分析预填 lens 触发；旧单泳道报告渲染回归不变。验证：unit test（scope 分支渲染/lane 点击 payload）+ 手动 UI 走查（opencli：板块分析 tab → 触发 → 报告渲染 → 泳道引用点击）
- [ ] 4.4 设置页参考角色管理 section（对齐博查配置模式）。验证：`pnpm lint` + 手动走查（录入/启停即时生效）

## 5. 收尾

- [ ] 5.1 影响包测试全绿：`go test ./internal/dataenrichment/...`（DB 集成）+ `golangci-lint run ./...` + `go vet ./...`；前端 `pnpm test:unit`（Windows cmd）
- [ ] 5.2 文档：`docs/reference/flow/data-enrichment.md` 增补版块级分析链路与参考角色库、业务约束节补条（参考角色=方法非事实、lane 证据、单泳道降级定位）；`architecture/map.md` 索引更新。验证：`doc-impact.sh verify` 通过
