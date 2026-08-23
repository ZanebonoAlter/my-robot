# fix-quality-audit-p0 — 质量审计 P0 修复

<!-- constraint-domains: reading, content-enrichment, semantic-board -->

## Why

2026-08-23 全链路质量审计（docs/research/quality-audit/）发现三个直接影响正确性的 P0 问题：① 前端读取后端从不返回的字段，导致 AI 摘要开关展示值与真实值恒相反；② `topic_tags.feed_count` 反规范化计数无对账机制，标签列表/聚类排序随时间持续失真；③ 前端 6 个零引用死组件（含 671 行大件）滞留误导维护。三项均已三重验证属实，改动小、收益大。

## What Changes

1. **修复 AI 摘要开关读键错位**：`front/app/stores/api.ts:148` 从 `feed.ai_summary_enabled`（后端从不返回）改为 `feed.article_summary_enabled`，默认值从写死 `true` 对齐后端默认 `false`；清理 `types/feed.ts` 双字段（`aiSummaryEnabled`/`ai_summary_enabled` 死字段）、`stores/api.ts` 响应接口死键、`useGlobalSettings.ts` 与 `FeedDetailEditor.vue` union type 中无调用方的 `'ai_summary_enabled'` 死 key（审计确认面板无 emit 调用，注释自认 legacy unused）。
2. **feed_count 周期对账**：在 `TagQualityScoreJob`（backend-go/internal/admin/scheduler/job_tag_quality_score.go）中新增对账 SQL，照抄同文件 auxiliary `ref_count` 对账模式：`UPDATE topic_tags SET feed_count = (SELECT COUNT(DISTINCT a.feed_id) FROM article_topic_tags att JOIN articles a ON a.id = att.article_id WHERE att.topic_tag_id = topic_tags.id)`。
3. ~~**删除 6 个零引用死组件**~~【实施中推翻，不执行】：审计的「零引用」判断是误判（grep 过滤 bug + Nuxt 自动导入裸名使用），运行时验证 6 个 dialog 全部活跃（详见 tasks.md 组 3 结论反转说明）。此调查方法缺陷已回馈修正审计报告。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `feed-settings-ui`: 新增 Scenario 锁定「feed 卡片 AI 摘要 toggle 状态读取自后端 `article_summary_enabled` 字段（缺省 false）」的字段名契约，防止字段名再次漂移。
- `tagging-domain`: 新增 Requirement「topic_tags.feed_count 周期对账」——TagQualityScoreJob 周期将 feed_count 重算为 distinct feed 引用数，修正打标不维护导致的排序失真。

> `unified-dialog` spec 的 Migration Map（历史迁移记录段落，非 Requirement）中 Pattern A 涉及清单同步移除已删 5 个死 dialog——纯文档同步，不出 delta spec，落 tasks.md 文档节处理。

## Impact

- **前端**：`front/app/stores/api.ts`（读键+响应接口）、`front/app/types/feed.ts`（双字段清理）、`front/app/composables/useGlobalSettings.ts`（union type 死 key + legacy ref 清理）、`front/app/features/settings/components/FeedDetailEditor.vue`（union type 死 key）、删除 `components/dialog/` 6 文件。用户可见行为变化：feed 卡片/文章页 AI 摘要开关终于反映真实后端值（旧行为恒显示开启）。
- **后端**：`backend-go/internal/admin/scheduler/job_tag_quality_score.go` 增对账 SQL（下个调度周期起 feed_count 自动修正，无需手工操作）。
- **无 API 契约变化、无表结构变化、无数据迁移**：`ai_summary_enabled` 相关后端读写两侧本就统一为 `article_summary_enabled`，纯前端侧修正。
