## Why

日报里的 section 顺序是"按质量排然后聚类"的产物，但用户完全不知道：

- 某条新闻凭什么排在前面（质量分从哪来、由什么决定）
- 某条新闻的 tag 为什么匹配到这个板块（匹配理由、相似度多少）
- `best_tier` / `avg_score` 这两个排序依据是什么含义、怎么算的

整套链路对用户是纯黑盒，无法诊断"为什么这条是头条""为什么这条被收进日报"。起源 issue：`docs/issues/01-quality-sort-blackbox.md`。

### 探索阶段的关键发现（决定本次 scope）

代码级复核暴露了三个 issue 未点明的事实，它们直接塑形了本 change 的设计：

1. **`downgraded` 标记在进日报管线的瞬间就丢了**（🔴 最深黑盒）。`collectBoardTags`（`daily_report_orchestrator.go:270`）的 SQL 不取 `downgraded` 列，`TagInput` 结构体（`daily_report_models.go:196`）压根没这个字段。后果：一个降级匹配的 `max_sim` tag（真实 tier=3）在**整条日报管线里被当成 tier=2**——系统自己都看不见降级，更别说用户。`filterTagsByQuality` 截断排序时还硬编码 `MatchTier(reason, false)`（`daily_report_orchestrator.go:407`），进一步固化了这个失真。

2. **section 级已持久化了聚合标量，但明细未持久化，且可半恢复地漂移**。`DailyReportSection` 有 `best_tier` / `avg_score`（生成时刻冻结✓）和 `cluster_tag_ids`（jsonb，tag ID 列表✓）。但每个 tag 的 `(match_reason, score, downgraded)` 明细在 section 组装时被压成两个标量丢弃。理论上能用 `cluster_tag_ids` rejoin 回 `topic_tag_board_labels`，但那张表会被 rematch 重写——**rejoin 出来的 score 会随时间漂移**（生成时的分 ≠ 现在的分）。

3. **`MatchTier` 语义未文档化，是人话翻译的地基缺口**。实际映射是 `direct_hit=0 > hit_rate=1 > max_sim(非降级)=2 > max_sim降级/weighted=3`（越小越好，`board_match_handler.go:403`）。前端"best_tier 升序"= 把 direct_hit 最多的 section 排最前。这个映射哪都没文档化，issue 里甚至把它误写成"tier=1 来自 direct_hit"（tier=1 其实是 hit_rate）。

### 与兄弟 change / 关联 issue 的边界

- **`topic-watchlist-observability`**（进行中，52 tasks）：刻意不碰质量排序黑盒，本 change 与之分离。两者都坚持"日报正文保持沉浸，理由进探究区"，但本 change 允许正文出一个**极轻的 tier 徽章（不露数字）**——因为质量排序直接决定了 section 先后顺序，给已成的排序事实一个色彩提示属于"解释既成事实"，不破坏阅读。
- **`embedding-content-mismatch`** issue（待 propose）：同源（匹配层误判），但一个是"治本（改 embedding/匹配区分度）"，本 change 是"装监控（只读暴露）"。保持分离，合并会让 change 失焦、且治本风险高一个量级。

## What Changes

四块，按数据血缘顺序，互不依赖可切片：

### A. 匹配血缘下沉（后端 · 治 🔴 事实1 + 缺口②）

把日报生成时刻每个 section 的全部来源 tag 的匹配明细**快照**到 section 级，使排序理由可追溯、且不随 rematch 漂移：

- `collectBoardTags` SQL 补取 `downgraded` 列；`TagInput` 结构体加 `Downgraded` 字段。
- `DailyReportSection` 新增 `quality_breakdown` JSON 列（结构 `[{tag_id, label, match_reason, score, downgraded}]`），在 section 组装时（`daily_report_orchestrator.go:164`）从当前作用域内的 `tags` 切片直接填充——**复刻 `BoardDailyReport.highlights` / `raw_clusters` 已有的 JSON 列模式，零新概念**。
- `MergeSimilarSections`（`daily_report_merge.go:203`）合并后按合并后的 tagIDSet 重算 `quality_breakdown`（与重算 `avg_score` 同处）。
- `filterTagsByQuality` 截断排序改用真实 `downgraded` 调 `MatchTier`（修硬编码失真）。

### B. MatchTier 语义文档化（后端 · 治 缺口③的地基）

把 tier 0/1/2/3 的映射与含义钉死，作为所有人话翻译的唯一依据。design.md 详述，daily-report-system spec 增一条 requirement 陈述该映射。

### C. 暴露（后端 API + 前端展示 · 治 缺口①③）

分两层 surface，复用 `match-score-visualization` spec 已建立的匹配质量语义（`matchReasonColor` / `matchInfoLabel` 工具函数）：

- **探究区（tag 级明细）**：section 详情/hover 展示该 section 全部来源 tag 的 `match_reason`（色彩）+ `score` + 降级标记。复用现有色系（direct_hit 绿 / hit_rate 蓝 / max_sim 橙 / weighted 灰）+ 降级 50% 不透明 + "↓" 后缀，与 `match-score-visualization` 在 TagsPage 的表现一致。
- **日报正文（tier 徽章，不破沉浸）**：每个 section 出一个极简 tier 徽章，**仅色彩、不露数字**（如最高质量=实心绿点、次之=蓝、可疑=橙、保底=灰）。用户一眼看出相对质量层级，不被数字打扰。这是本 change 唯一破兄弟 change"正文纯沉浸"原则的点，且经过克制处理（无分数文字）。

### D. 历史/兼容（§10）

`quality_breakdown` 列允许 NULL（历史 section 无明细），前端降级为"无质量明细"。不回刷历史 section。迁移幂等、可逆（DROP COLUMN）。

## Capabilities

### New Capabilities

无（本 change 不引入新业务能力，是对现有 daily-report-system 的可观测增强）。

### Modified Capabilities

- `daily-report-system`: 新增"质量血缘快照（quality_breakdown 列）"、"MatchTier 语义规格化"、"日报 section 质量明细 API 暴露"三条 requirement。

## Impact

- **后端（`internal/topicgraph/`）**
  - 修改：`daily_report_orchestrator.go`（collectBoardTags 取 downgraded + section 组装填 quality_breakdown + filterTagsByQuality 修硬编码）；`daily_report_merge.go`（合并重算 quality_breakdown）；`daily_report_models.go`（TagInput 加 Downgraded 字段、DailyReportSection 加 QualityBreakdown JSON 列）。
  - 迁移：版本化迁移 `daily_report_sections` 加 `quality_breakdown JSONB NULL`。
  - API：日报 detail / section 接口序列化 `quality_breakdown`。
- **前端（`front/app/features/tags/`）**
  - 探究区：section 详情/hover 展示 tag 级明细，复用（并上移到共享 utils）`matchReasonColor` / `matchInfoLabel`。
  - 正文：tier 徽章组件（纯色彩、无数字），颜色由主题语义 token 派生（editorial/dark 双主题）。
- **数据兼容**：历史 section `quality_breakdown=NULL`，前端降级显示。
- **不做**：不动匹配算法（归 `embedding-content-mismatch`）；不建新表（用 JSON 列）；不回刷历史；不改 `tag-to-board-matching` / `match-score-visualization` 的 spec（本 change 只读消费它们的语义，不改其行为）。
