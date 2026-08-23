# Design — fix-quality-audit-p0

## Context

质量审计（docs/research/quality-audit/）三项 P0 修复。三项互相独立、均为小改动，合一个 change 因为主题一致（审计 P0 落地）且各自单独开档开销不成比例。代码细节已在审计阶段核实（file:line 级），无歧义需要再决策的架构问题，本 design 只记录关键取舍。

## Goals / Non-Goals

**Goals:**
- 前端 AI 摘要开关读值与后端真实值一致（修读键 + 清死字段 + 默认值对齐 false）
- feed_count 有了周期自愈机制，标签排序不再持续失真
- 死组件清零，unified-dialog spec 迁移图与实态一致

**Non-Goals:**
- 不动后端 `article_summary_enabled` 的读写实现（本就正确）
- 不做 narrative 遗留清算、BoardThreadBrowser 拆分等 P1 项
- 不为死组件删除引入「先归档再删」流程——三重 grep 已验证，直接删
- 不重构 useGlobalSettings 的 legacy AI 设置区（只移除与 `ai_summary_enabled` 死 key 直接相关的部分，其余 legacy ref 保持原样）

## Decisions

**D1. 读键修复直接对齐后端字段，不做兼容期映射**
- `stores/api.ts:148` 改为 `feed.article_summary_enabled ?? false`。后端 `ToDict`（models/feed.go:60）稳定返回该键，无历史版本兼容负担（单用户系统，前后端同仓同部署），两层间接（先映射后消费）只会再加漂移点。
- 默认值取 `false`：对齐 gorm `default:false`（models/feed.go:24）。旧代码写死 `true` 是 bug 的一部分。

**D2. feed_count 对账放进 TagQualityScoreJob，照抄 ref_count 既有模式**
- 该 job（admin/scheduler/job_tag_quality_score.go）已有 auxiliary `ref_count` 对账 SQL 先例，运行频率已覆盖「周期修正」需求；单独开新 job 反而增加调度面。
- 用 `COUNT(DISTINCT a.feed_id)`（模型注释明示该语义，topic_graph.go:57），一条 UPDATE 全量重算，不搞增量维护（打标路径加维护属过度设计，审计确认量级可控）。
- 失败处理照抄 ref_count：`logging.Warnf` 不中断 job。

**D3. union type 里的 `'ai_summary_enabled'` 死 key 一并清理**
- `useGlobalSettings.ts:54` 与 `FeedDetailEditor.vue:16` 的 setting union type 含该 key 但全库无 emit 调用（审计 + 复核确认），后端 update handler 也只认 `article_summary_enabled`（feed_handler.go:349）。保留即暗示「这条路径可用」，误导后来者。删 key 属于本 change 主题内（消灭 `ai_summary_enabled` 这个名字），非越界清理。

**D4. 死组件删除连带 unified-dialog spec 迁移图更新**
- Migration Map Pattern A（unified-dialog/spec.md:206）列有其中 5 个 dialog。该段是历史迁移记录而非行为契约，删除组件后同步把涉及清单改为标注已移除，避免 spec 与代码再次漂移。

## Risks / Trade-offs

- [feed 卡片开关从「恒显示开」变为真实值，存量 feed 未开过摘要的会显示关闭] → 这正是修复目标；部署影响汇报中向用户明示此可见变化，无数据操作。
- [对账 SQL 全表重算，标签量大时 job 变慢] → 单用户量级（千级 tag）下毫秒级；照抄 ref_count 模式已验证可行。若未来量级增长，job 有 tracing 可观测。
- [删除组件漏检动态引用] → 已做 Pascal/kebab/import 三重 grep（含测试与 plugins）复核；apply 阶段删除后跑 `pnpm lint` + `nuxi typecheck` 双验证兜底。
- [useGlobalSettings legacy ref 清理牵出隐藏消费方] → apply 时 grep `aiSummaryEnabled` 全部消费点确认仅内部使用后再动；有外部消费则只清 union key、ref 保留并在 tasks 标注。

## Migration Plan

纯代码修复，无数据迁移。后端对账 SQL 随下个 TagQualityScoreJob 调度周期自动生效（无需手工触发；如需立即验证可手动跑一次 job）。回滚 = git revert 三处改动，无状态残留。
