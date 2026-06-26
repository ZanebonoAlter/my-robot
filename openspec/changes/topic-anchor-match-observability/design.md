## Context

日报 section 有**两套独立维度的匹配血缘**，本 change 给第二套装监控：

| 维度 | 匹配对象 | 字段 | 已可视化？ |
|------|---------|------|-----------|
| System 1（标签↔板块） | 每 tag 为什么/多强打进**板块** | `best_tier`/`avg_score`/`quality_breakdown` | ✅ `quality-scoring-observability`（正文 tier 色点 + hover 探针 per-tag 明细） |
| System 2（section↔话题） | 这个 section 多紧地锚到**持久话题** | `topic_match_distance`(余弦距离)/`topic_match_confidence`(`anchor_hit`/`auto_new`/`unmatched`)/`persistent_topic` | ❌ **本 change** |

数据流现状（**后端链路完全就绪**）：

```
daily_report_assignment.go: planTopicAssignments
   anchor_hit  → distance=pt.dist (≤ MatchThreshold=0.30), confidence=anchor_hit
   auto_new    → distance=nearestDist (> 0.30, 新开候选),     confidence=auto_new
   unmatched   → distance=0,                                  confidence=unmatched (无 embedding)
   ▼ 持久化 daily_report_sections: topic_match_distance, topic_match_confidence, persistent_topic_id
   ▼
daily_report_repository.go 四接口 SELECT（detail/timeline/lifeline/topic-lifeline）
   line 395-396 / 528-529 / 700-701 均已取 topic_match_distance, topic_match_confidence
   ▼
前端 api/dailyReports.ts: DailyReportSection + SectionTimelineNode 已声明三字段
   ▼ ⚠️ 断点：前端只消费 persistent_topic_id/status/color（分区分组），
        topic_match_distance / topic_match_confidence 被丢弃，零展示
```

真实分布佐证（report 102，均 anchor_hit）：霍尔木兹海峡 `0.0477`（极紧）→ 以黎冲突 `0.1344`（紧）→ 特朗普/AI `0.2996`（边缘松锚）。同处 active 区、同话题下，紧实度差距近 6 倍却长得一模一样——这就是本 change 要补的可观测缺口。

约束：

- 纯前端 change（Nuxt 4 + Vue 3）。后端零改动、零迁移、零回刷。
- 复用 `quality-scoring-observability` 建立的展示分层哲学：**正文极轻（无数字），分数文字只进探究区**。
- confidence 三态已通过 `dailyReportMagazine.ts:73-78` 的三区（active / candidate / unassigned）**间接表达**，本 change 不重复造这个分区信号，聚焦"同区内/同话题下的紧实度区分"。
- 单用户、无 auth。

## Goals / Non-Goals

**Goals:**
- 正文 section 卡片头部并列展示两套信号：tier 徽章（System 1，已有）+ 锚定紧实度点（System 2，新增），均无数字。
- hover 探究区顶部新增「话题锚定」行：话题名 + 距离数值 + 中文紧实度标签。
- 历史/未锚定 section（无 `topic_match_distance` 或 `confidence=unmatched`）统一降级，不报错。
- 工具函数单测覆盖三态 + 紧/松分档 + 降级。

**Non-Goals:**
- 不动持久话题锚定算法（`daily_report_assignment.go` 的 `MatchThreshold`/双重确认逻辑，归领域 change）。
- 不在正文暴露 distance 原始数值（只进探究区，保沉浸）。
- 不做话题生命线 / 话题清单的锚定分布统计（留作后续 change，避免 scope 膨胀）。
- 不改 System 1 可视化（`SectionTierBadge`/`quality_breakdown` 行为不变）。
- 不动后端任何代码与数据契约（字段已就绪）。

## Decisions

### D1. 纯前端，后端零改动（链路已通）

detail / timeline / lifeline / topic-lifeline 四接口均已 SELECT `topic_match_distance`/`topic_match_confidence`（`daily_report_repository.go:395-396/528-529/700-701`），前端 `api/dailyReports.ts` 类型已声明。本 change 仅前端消费侧补展示，零后端风险。

**备选（否决）**：若后端没取这列，本需改 SQL——但已确认取了，无需。

### D2. 紧实度分档：双阈值三档 `TIGHT=0.05 / LOOSE=0.15`（数据驱动）

锚定紧实度五态，**形态承担主信息**（不透明度递减 + 空心），颜色用单一中性强调 token（不混 System 1 四色，避免与 tier 徽章的绿/蓝/橙/灰撞色混淆）：

| confidence | distance | 形态 | 语义 |
|-----------|----------|------|------|
| `anchor_hit` | `≤ 0.05` | 实心点，accent token（100%） | 极紧锚定（标题级/近标题） |
| `anchor_hit` | `(0.05, 0.15]` | 半透明点，accent token（`color-mix 55%`） | 稳锚定（内容贴合） |
| `anchor_hit` | `(0.15, 0.30]` | 淡半透明点，accent token（`color-mix 30%`） | 松锚定（边缘） |
| `auto_new` | 任意 | 空心点，accent token | 新话题候选（未锚到已有） |
| `unmatched` / 无 distance | — | 空心点，灰 token | 未锚定 |

**理由（数据驱动，基于 2026-06-26 全量 35 条 section 实测）**：

26 条 anchor_hit 的 distance 呈**三段聚集**，双阈值恰好切分三段：

| distance 区间 | 条数 | 特征 | 对应档 |
|--------------|------|------|--------|
| `[0, 0.05]` | 16 | 标题级/近标题（话题名≈section 名，distance 天然≈0） | 极紧 |
| `(0.05, 0.15]` | 7 | 内容真贴合、表述不同（如以黎冲突 0.1097） | 稳锚 |
| `(0.15, 0.30]` | 3 | 松锚，接近 0.30 临界（⚠️ 差点没够上） | 松锚 |

> 单阈值 0.15 会把 26 条压成 23紧/3松（区分度≈0）；双阈值三档还原出真实的三段聚集——**这才是用户想看的"锚实度差异"**。0.05/0.15 把 MatchThreshold(0.30) 三等分，既是自然的几何分界，又对齐数据聚集点。

形态递减（实心 100% → 半透明 55% → 淡半透明 30% → 空心）天然表达紧实度梯度，对色盲友好；auto_new 与 unmatched 都空心（"身份未坐实"），靠颜色（accent vs 灰）区分。auto_new 段另有 2 条 distance≈0（LLM 未指派已有话题，双重确认未过），印证 `confidence` 须作主判据。

**备选（否决）**：
- 单阈值 0.15：23紧/3松，区分度≈0，等于没做（被 2026-06-26 实测否决）。
- 降到 0.05 单阈值二档：18紧/8松，但极紧段（标题级 trivial）混入"紧"，丢失"内容贴合 vs 标题复用"的区分。
- distance 连续映射透明度：正文出现"渐变点"，视觉噪声大且无离散语义锚点。

### D3. 新建 `SectionAnchorBadge.vue`，不改 `SectionTierBadge`

`SectionTierBadge` 是 single-responsibility（System 1 的 best_tier），话题锚定是独立维度。新建并列组件避免：① 混淆两套语义；② 破坏已有组件的单测/调用点；③ 徽章 props 膨胀。

两徽章并列挂载于 `DailyReportTopicSection.vue` 的 `.drm-section-card__head`（tier 点在前、锚定点在后，间距 0.25rem）。锚定点尺寸略小（0.4rem vs tier 的 0.5rem），强化"主信号(tier) + 辅信号(anchor)"的视觉层级。

### D4. 探究区加「话题锚定」header 行（探针顶部）

`SectionQualityExplore.vue` 当前只接 `breakdown`。新增三个可选 props（不破坏现有调用）：`topicLabel?: string`、`topicDistance?: number`、`topicConfidence?: string`。有值时在 per-tag 列表**上方**渲染一行：

```
🔗 话题锚定 · {topicLabel} · 距离 0.13 · 紧锚定
```

距离数值与中文标签（"紧锚定"/"松锚定"/"新话题候选"/"未锚定"）在此展示；无值时整行不渲染（探针仍展示 per-tag 明细，或历史 section 显示"无质量明细"）。

### D5. 与 `quality-scoring-observability` / `topic-watchlist-observability` 的边界

- **`quality-scoring-observability`**（已收尾）：装的是 System 1（标签↔板块）。本 change 装的是 System 2（section↔话题）。两者"同一套日报 section、两个独立匹配维度、两个展示面"，正文徽章并列、探究区探针分区（System 2 行在顶部，System 1 per-tag 列表在下）。本 change 不改其 spec。
- **`topic-watchlist-observability`**（进行中）：归属可观测，动 topicgraph 域且关注"话题本身的可观测（命中、活跃、断裂）"。本 change 关注"section 锚到话题的紧实度暴露"，数据血缘来自日报 section 字段，关注点正交。两者共享"正文保沉浸"原则。
- **`embedding-content-mismatch`**（issue，待 propose）：治本（改 embedding/匹配区分度），本 change 只读暴露 System 2 锚定结果，不改算法。保持分离。

## Risks / Trade-offs

- **[两徽章并列可能让正文头部变挤]** → 锚定点用更小尺寸（0.4rem）+ 紧凑间距（0.25rem），总占用仍 < 一字符宽。验证：在 `DailyReportTopicSection` 单测里断言两徽章同时渲染、head 不溢出。
- **[双阈值 `0.05/0.15` 是基于 2026-06-26 数据标定的，可能不普适]** → 做成 `topicAnchor.ts` 的**导出常量**（`TIGHT_THRESHOLD=0.05`、`LOOSE_THRESHOLD=0.15`），单测覆盖两道线边界值（0.05/0.15 ±0.001），后续若需调只改一处。不读后端配置（避免引入前后端耦合，且后端 MatchThreshold 本身也是硬编码默认值）。
- **[auto_new 的 distance 是"到最近邻"的距离，语义与 anchor_hit 不同（一个是"没够上"）]** → 探究区标签显式区分（"新话题候选"而非"松锚定"），正文形态也用空心（区别于 anchor_hit 的半透明），避免用户把 auto_new 的高 distance 误读为"松锚"。
- **[历史 section 的 topic_match_distance 可能为 0（零值）而非 null]** → `confidence=unmatched` 是判定的主信号，不单看 distance==0。工具函数优先判 confidence，distance 仅在 anchor_hit 内用于紧/松分档。
- **[正文第二个点增加视觉复杂度，可能违反兄弟 change 的"纯沉浸"原则]** → 经克制处理（无数字、单一形态语义、尺寸更小）。这是继 quality-scoring tier 徽章之后的第二个破例点，理由同 D4（quality-scoring design）：锚定紧实度直接影响 section 在话题内的"叙事可信度"，给既成事实一个色彩提示属"解释"非"打扰"。若 review 认为过重，可降级为方案 A（全进探究区），spec 已分层设计支持回退。
