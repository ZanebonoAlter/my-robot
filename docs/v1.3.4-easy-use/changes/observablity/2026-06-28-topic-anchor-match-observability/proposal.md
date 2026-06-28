## Why

日报 section 的持久话题锚定（section↔persistent topic）数据**已经落库并已通过 API 传到前端**（`topic_match_distance` 余弦距离、`topic_match_confidence` 三态 `anchor_hit`/`auto_new`/`unmatched`），但前端**零展示**——只取 `persistent_topic_id`/`status` 做分区分组，把锚定紧实度的分数全丢了。

这导致用户看到的质量信号只有 `quality-scoring-observability` 装的那一套（**标签↔板块**匹配的 `best_tier`/`quality_breakdown`，即"版块综合评分"），而**真正想要的是"这条 section 锚到那个持久话题有多紧"**（**section↔话题**匹配分）——后者当前完全黑盒。`design.md` D6 刚放宽了持久话题锚定的双重确认（锚定本就更脆弱），这块恰恰最该被观测却最看不到，本 change 就是给它装监控。

## What Changes

纯前端 change，后端零改动（detail/timeline/lifeline/topic-lifeline 四接口都已 SELECT 了 `topic_match_distance`/`topic_match_confidence`，数据链路就绪）。按展示分层，互不依赖可切片：

- **A. 探究区话题锚定行（hover 探针）**：在现有 `SectionQualityExplore` 探针顶部加一行——「🔗 话题锚定 · {话题名} · 距离 0.13 · 强锚定」，复用 System 1 的"分数文字只进探究区"哲学。历史 section（无 `topic_match_distance`）降级文案"未锚定话题"。
- **B. 正文锚定紧实度点（tier 徽章旁）**：在现有 `SectionTierBadge`（System 1 色点）旁加**第二个更小的点**，表示 section↔话题锚定紧实度，**仅形态、无数字**：按 distance 双阈值三档递减（实心→半透明→淡半透明）表达极紧/稳/松锚，空心表达未锚/新候选。与 tier 徽章并列，让正文一眼可见两套信号。
- **C. 工具函数**：新增 `topicAnchorTier(distance, confidence)`（紧实度分档）与 `topicAnchorLabel(distance, confidence)`（中文标签），与 `matchReasonColor`/`matchInfoLabel` 同放共享 utils，供探针 + 徽章复用。
- **D. 测试**：工具函数单测 + 徽章/探针组件单测，覆盖三态、紧/松分档、历史降级。

## Capabilities

### New Capabilities
<!-- 无。本 change 给已有 capability 加 requirement，不新建。 -->

### Modified Capabilities
- `daily-report-system`: 新增「日报 Section 话题锚定可视化暴露」requirement——正文锚定紧实度点（无数字）+ 探究区话题锚定行（含距离数值）。与 `quality-scoring-observability` 新增的「质量明细暴露」并列，覆盖日报 section 的第二套匹配血缘（section↔话题，而非标签↔板块）。

## Impact

- **前端（`front/app/`）**
  - 修改：`features/tags/components/daily-report/SectionQualityExplore.vue`（探针顶部加话题锚定行）；`features/tags/components/daily-report/SectionTierBadge.vue` 或同目录新增 `SectionAnchorBadge.vue`（锚定紧实度点）；`features/tags/components/daily-report/DailyReportTopicSection.vue`（挂载锚定点）。
  - 新增：`utils/topicAnchor.ts`（紧实度分档 + 中文标签）+ 单测。
  - 数据契约 `api/dailyReports.ts`：`topic_match_distance`/`topic_match_confidence`/`persistent_topic` 字段已存在，无需改。
- **后端（`backend-go/`）**：**零改动**。detail（GORM Preload）/ timeline / lifeline / topic-lifeline 四接口都已 SELECT 这两列（`daily_report_repository.go:395-396/528-529/700-701`）。
- **数据兼容**：历史 section 的 `topic_match_distance`/`topic_match_confidence` 可能为空（`topic_match_distance` 用 `omitempty`，未锚定时为零值/缺省），前端统一降级为"未锚定"空心点，不报错。无迁移、无回刷。
- **不做**：不动持久话题锚定算法（归 `daily_report_assignment.go` 的领域逻辑）；不暴露距离原始数值到正文（只进探究区）；话题生命线/话题清单的锚定分布统计不在本期（留作后续 change，避免 scope 膨胀）；不改 `quality-scoring-observability` / `topic-watchlist-observability` 的 spec（本 change 只读消费已落地的字段）。
