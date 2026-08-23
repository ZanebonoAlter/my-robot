
## A 级四件套已拍板决策与迁移前提

【范围】迁移事实库基础 + A 级四件套，用户已拍板全部三个决策点（2026-08-21）。

【本仓库现状（区别于另一项目）】
- `.pi/extensions/` 下无 `lib/` 目录、无 harness-log.ts、无 harness-telemetry.ts——事实库基础设施完全不存在，需从 docs/research/ 三份参考副本迁移
- constraint-injection.ts（29627B）无任何 logEvent 插桩（grep 确认仅注释提及 harness 一词）
- quality-gate.ts（6812B）无插桩
- `.pi/extensions/tests/` 只有 constraint-injection.smoke.cjs + run-smoke.sh，无 run-harness-smoke.sh
- .gitignore 无 .pi/harness/ 条目

【已拍板决策】
- A1 pin 身份：change 语境 (change, title) 复合键；research 语境（docs/research/<topic>/）锚点 id（pin_finding 写入时生成 `<!-- pin:xxx -->`）。pin.read 由 constraint-injection 注入 explore-findings.md 时按 `###` 标题自报；pin.read 保留期 30 天（对齐 constraint.inject），pin.write 永久
- A2 安全开库：PRAGMA application_id（拟 ASCII "SYNT"）+ user_version 单调版本；失败策略=拒绝（append-only 审计账本不做 dsh 派生库的重置重建）；检查必须在任何写 PRAGMA（含 journal_mode=WAL）之前；新库判定在创建后做（node:sqlite 打开即建空库，application_id=0 且无用户表）
- A3 粒度：每命令一条 gate.check {cmd, phase, ok, ms, diag}，非每回合一条 gate.run；修复回合不单独记（下轮 gate.check 通过自然闭环）；diag 复用 A4 截断规范
- A4 白名单六类：quota-block|timeout|gate-fail|model-error|tool-error|unknown，映射不进一律 unknown 不透传原文；failure 对象 {stage: dispatch|run|result, category, exitLike, diag≤512B}；dsh 纪律"诊断是展示文本不是协议，程序不得按其分支"
- A4 效果待观察（用户明确要求后续特别关注：失败分类是否真能提升诊断效率）

【依赖关系】A2 并入迁移任务（一次改开库逻辑）；A4 的 gate-fail 类别判定依赖 A3 已记账；A1 建议最后（需先拍板锚点格式，已在决策内）

【源实现参考】docs/research/harness事实库.md（设计文档）、docs/research/harness-telemetry.ts（钩子采集）、docs/research/lib/harness-log.ts（node:sqlite 引入含实验 warning 外科抑制、dbCache 按路径缓存、RETENTION_DAYS、SIZE_FUSE 100MB）。注意源实现的 fileRefs 指向另一项目路径，迁移时只搬逻辑。

<!-- pinned 2026-08-23T03:47:13Z -->
