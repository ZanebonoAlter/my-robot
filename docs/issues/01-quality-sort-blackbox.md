# Issue 01: 日报质量排序与匹配黑盒 — MatchReason/Score/tier 链路不可见

> **Status:** needs-triage
> **Priority:** medium
> **Component:** backend-go/internal/tagmanagement（tag→board 匹配）, backend-go/internal/topicgraph/service（聚类/聚合/排序）, front/app/features/tags（展示）

## 症状

日报里的 section 顺序是"按质量排然后聚类"的产物，但用户完全不知道：

- 某条新闻凭什么排在前面（质量分从哪来、由什么决定）
- 某条新闻的 tag 为什么会匹配到这个板块（匹配理由是什么、相似度多少）
- best_tier / avg_score 这两个排序依据是什么含义、怎么算的

整套链路对用户是纯黑盒，无法诊断"为什么这条是头条""为什么这条被收进日报"。

## 诊断

质量排序其实是**三层叠加**的黑盒，跨两个业务域，且关键中间数据未持久化到 section 级。

### 链路全景

```
tag 匹配到 board（tagmanagement 域，最底层黑盒）
  │  每个 tag 带 MatchReason + Score
  │    MatchReason ∈ {direct_hit, hit_rate, max_sim, weighted}
  │    Score ∈ [0,1]
  ▼
filterTagsByQuality（daily_report_orchestrator.go:386）
  │  按 MatchReason 分流：direct_hit/hit_rate/max_sim 入 kept，weighted 入保底
  │  kept < 10 拉回 weighted；kept > 30 按 MatchTier+Score 截断到 30
  ▼
ClusterTags（daily_report_cluster.go，LLM 聚类，话题归属同层）
  │  tag 分组成 cluster → section
  ▼
section.best_tier / section.avg_score（orchestrator.go:188-189 赋值）
  │  best_tier = tagging.MatchTier(MatchReason)   ← 从 MatchReason 映射的质量等级
  │  avg_score = 组内 tag Score 的平均
  │  MergeSimilarSections（daily_report_merge.go:201-255）合并时取组内最优 tier + 平均分
  ▼
前端 sortDailyReportSections（dailyReportMagazine.ts）
   best_tier 升序 + avg_score 降序
```

### 三个关键缺口

| 缺口 | 位置 | 后果 |
|------|------|------|
| **① 匹配层黑盒** | tag→board 的 MatchReason/Score 在 `tag-to-board-matching`（tagmanagement），用户看不到某个 tag 为什么匹配上、相似度多少 | 最深黑盒，且与已记录的 `embedding-content-mismatch` issue 同源（都是 embedding 区分度问题） |
| **② 中间数据未下沉** | MatchReason/Score 停在 tag 层，**未持久化到 section 级**（section 只有聚合后的 best_tier/avg_score） | 排序理由的"来源 tag 列表 + 各自理由"在聚类后丢失，无法回溯 |
| **③ 聚合规则不可见** | best_tier 取最优、avg_score 取平均，但 MergeSimilarSections 的合并逻辑用户看不到 | "凭什么这条 tier=1"无法解释 |

### 跨域性质

| 层 | 域 | 现有 spec capability |
|----|----|---------------------|
| 匹配 | tagmanagement | `tag-to-board-matching` / `match-score-visualization`（已有，但可视化程度待查） |
| 过滤/聚类/聚合/排序 | topicgraph | `daily-report-system` |

**关键约束**：质量可观测需把 tag 的 MatchReason/Score 下沉到 section，跨了 tagmanagement 域。这是它不能并入 `topic-watchlist-observability` change（只管 topicgraph 归属域）的根本原因。

## 临时缓解

无（纯黑盒，未做任何可观测改造）。

## 待澄清（理清后再 propose）

在单开 change（暂定 `quality-scoring-observability`）前，需先回答：

1. **可观测目标粒度**：暴露到 section 级（"这条 best_tier=1，来自 3 个 direct_hit tag，均分 0.82"）还是到 tag 级（每个 tag 的 MatchReason+Score 明细）？前者轻，后者重但能治①。
2. **数据下沉方案**：MatchReason/Score 是否需持久化到 section（新表或 section 加列）？还是查询时从 tag 关联实时算？前者支持历史回溯，后者零迁移。
3. **与 `match-score-visualization` spec 的关系**：该 capability 是否已覆盖 tag→board 匹配可视化？若是，本次可能只补"下沉到 section + 日报暴露"这部分，不重复造。
4. **与 `embedding-content-mismatch` issue 的关系**：两者同源（匹配层），但一个是"误匹配治本"，一个是"让匹配可见"。是否合并处理？
5. **暴露位置**：日报正文（轻量徽章如"高质量·3源"）还是探究区（hover/详情看明细）？与 `topic-watchlist-observability` 的"日报保持沉浸"原则如何协调。
6. **best_tier 语义**：tagging.MatchTier 的等级定义（tier 1/2/3 分别对应什么 MatchReason）需查清并文档化，否则人话翻译无依据。

## 相关

- 现有 spec：`openspec/specs/tag-to-board-matching/`、`openspec/specs/match-score-visualization/`、`openspec/specs/daily-report-system/`
- 相关 issue：`docs/issues/embedding-content-mismatch.md`（匹配层同源，跨域误判）
- 起源：探索 `topic-watchlist-observability` change 时，用户提出"质量排序也是黑盒需要补充"；因跨域且需先理清，独立记录。

## Comments

（2026-06-25 记录）与 `topic-watchlist-observability` change 分离的原因：归属可观测只动 topicgraph 域且数据已部分持久化；质量可观测跨 tagmanagement 域且需数据下沉，关注点不同，硬塞会让 change 失焦、tasks 膨胀。待上述待澄清项理清后再 propose。
