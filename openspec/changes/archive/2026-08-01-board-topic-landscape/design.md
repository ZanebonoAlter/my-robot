# Design — board-topic-landscape

> 板块内容首屏「话题态势版图」。方案与用户敲定（见会话 explore 结论）。本文固化可实现的决策。

## 1. 核心约束：identity 轨，不用 similarity 轨

Syntopica 话题关系是**双轨**（见 `docs/reference/flow/topic-graph.md` §关系双轨）：

| 轨 | 算法 | 产物 | 可靠性 |
|---|---|---|---|
| similarity | 匈牙利二分法（`daily_report_matching.go`） | section↔section 时间线连线 + emerging/continuing/split/merge/ending 五态 | **长跨度不可靠** |
| identity | 同 `persistent_topic_id`（AND-gate 锚定写入） | 话题泳道连续性 + `planLifecycle` 维护字段 | **可靠** |

本 change 的态势**全部基于 identity 轨**：

- 持久话题字段（`planLifecycle` 维护，全人工归档）：`status` / `hit_count` / `consecutive_hits` / `first_seen_date` / `last_seen_date` / `is_vacuum` / `vacuum_strong` / `source`。
- section 归属（`SaveReport` 事务内写入快照）：`daily_report_sections.persistent_topic_id`（未归属写 NULL）。

**禁止**：读取 `daily_report_section_relations` 表的 similarity 边、或照搬 emerging/.../ending 五态。这两者是 section 级配对的派生，不可靠。

## 2. 态势派生规则

主态势（互斥，按顺序匹配第一个命中）：

| 态势 | 图标 | 派生规则 | identity 字段 |
|---|---|---|---|
| 新冒头 | 🌱 | `status='candidate' AND 1 <= hit_count < upgrade_threshold`（hit=0 纯 orphan 不展示） | status, hit_count |
| 待激活 | 🔴 | `status='candidate' AND hit_count >= upgrade_threshold`（即 `CanActivate=true`） | status, hit_count |
| 活跃 | 🟢 | `status='active' AND consecutive_hits > 0 AND days_since(last_seen_date) <= N` | status, consecutive_hits, last_seen_date |
| 停滞 | ⏸️ | `status='active' AND (consecutive_hits = 0 OR days_since(last_seen_date) > N)` | 同上 |
| 已归档 | ⬛ | `status='archived'` | status |

叠加标记（与主态势正交，可叠加在活跃/停滞上）：

| 标记 | 图标 | 规则 |
|---|---|---|
| 强吸引 | 🌀 | `is_vacuum=true`（卡片角标显示，附 `vacuum_strong` 数值） |

参数：

- `N`（活跃判定窗口）= **7 天**，包级常量 `topicLandscapeActiveWindowDays`（MVP 不进 ai_settings，零迁移；未来需调再提配置项）。
- `upgrade_threshold` 复用既有 `LoadPersistentTopicConfig().UpgradeThreshold`（默认 3）。
- `days_since(last_seen_date)` = 今日（服务器本地时区）减去 `last_seen_date` 的日历天。

派生**在后端**完成（接口直接返回 `stance` 字段），前端不重复算，保证口径单一。

## 3. 数据契约

### `GET /api/semantic-boards/:id/topic-landscape?days=30`

- 路径参数 `id` = semantic_board_id。
- **实现复用**：话题列表用 `ListTopicsByBoardAll`（含 archived/orphan）；**可见过滤不复用 `FilterVisibleTopics`**（那是话题管理 UI 口径，会剔掉 emerging candidate）——版图 SHALL 保留 `hit_count>=1` 全部（含 emerging 新苗头），仅剔 `hit=0` 纯 orphan。路由注册于 `RegisterDailyReportRoutes`（同组）。
- 查询参数 `days` = lifeline 窗口天数，默认 30，允许 7/14/30/90。days=0 视为默认 30。
- 响应（`success:true, data:{...}`）：

```jsonc
{
  "topics": [
    {
      "id": 12, "label": "芯片战", "status": "active", "source": "auto",
      "stance": "active",            // 主态势：emerging|pending|active|stalled|archived
      "is_vacuum": false, "vacuum_strong": 0,
      "hit_count": 47, "consecutive_hits": 22,
      "first_seen_date": "2026-05-01",
      "last_seen_date": "2026-06-22",
      "days_since_last": 0,
      "can_activate": false,
      "lifeline": [                   // 近 N 日按天聚合，identity 轨
        { "date": "2026-06-01", "section_count": 2 },
        { "date": "2026-06-02", "section_count": 0 },  // 空日也补行，见 §4
        { "date": "2026-06-03", "section_count": 3 }
      ]
    }
  ],
  "vitality": {                       // 顶栏一行
    "days": 30,
    "article_count": 142,
    "section_count": 38,
    "active_topic_count": 6,
    "feed_active": null,              // MVP 可空（跨域 feed 查询，后续补）
    "trend": [3, 1, 2, 5, 7, 6, 4, 3] // 近 N 日每日 section 数，缩略折线用
  }
}
```

- 空数组语义：`topics=[]` 表示板块无任何持久话题（含未达 `upgrade_threshold` 的 observing candidate，它们本就对前端隐藏）；`vitality.trend=[]` 表示窗口内无日报。

### lifeline 聚合 SQL（identity 轨，可靠）

```sql
SELECT d::date AS date, COALESCE(cnt, 0) AS section_count
FROM generate_series(
       (now() - INTERVAL '$days days')::date,
       now()::date,
       INTERVAL '1 day'
     ) WITH ORDINALITY AS g(d)
LEFT JOIN (
  SELECT (r.period_date AT TIME ZONE 'Asia/Shanghai')::date AS rd, COUNT(*) AS cnt
  FROM daily_report_sections s
  JOIN board_daily_reports r ON s.report_id = r.id
  WHERE s.persistent_topic_id = $topic_id
  GROUP BY rd
) t ON t.rd = g.d
ORDER BY g.d;
```

> 时区：`period_date` 存 UTC，按服务器配置时区（默认 Asia/Shanghai）转本地日。实现时取 `time.Local`，与日报生成时区一致。

## 4. mini-lifeline：空日补行

热力图/节奏条的可读性依赖**日期轴连续**。某日该 topic 无 section（或该 board 当日无日报）→ `section_count=0`，**渲染空格**（浅灰），不跳过该列。由 `generate_series` LEFT JOIN 保证连续（见上 SQL）。

## 5. 前端布局

```
┌─ 板块内容 tab（BoardCompositionPanel）──────────────────────┐
│  [构成标签管理区 ... 不动]                                   │
│  ════════════════════════════════════════════════════════ │  ← 分隔
│  话题态势版图                                               │
│  ┌─ 活力顶栏 ───────────────────────────────────────────┐ │
│  │ 近30天 142篇·38sec·6活跃话题  ▁▂▃▅▇▆▄▃            │ │
│  └──────────────────────────────────────────────────────┘ │
│  ┌─ 态势分区卡片墙 ─────────────────────────────────────┐ │
│  │ 🟢活跃(6)  ⏸️停滞(5)  🌱新冒头(3)  🔴待激活(2)  ⬛归档│ │
│  │ ┌─────────┐ ┌─────────┐ ...                          │ │
│  │ │芯片战🟢 │ │GPT🌀    │                              │ │
│  │ │连续22·47│ │吞噬53   │                              │ │
│  │ │▓▓░▓▓▓▓│ │▓▓▓▓▓▓▓▓│                              │ │
│  │ └─────────┘ └─────────┘                              │ │
│  │ ↑ click → 话题总览 tab 该 topic 泳道                 │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
```

组件族（`front/app/features/tags/components/topic-landscape/`）：

- `TopicLandscapePanel.vue` — 容器，拉接口、空态分发。
- `VitalityBar.vue` — 活力顶栏（数字 + 缩略折线）。
- `StanceCardWall.vue` — 分区卡片墙，按 stance 分组渲染，已归档默认折叠。
- `TopicStanceCard.vue` — 单卡（label + stance 图标 + 关键数字 + mini-lifeline + 点击跳转 + 待激活红描边）。
- `MiniLifeline.vue` — 近 N 日节奏条（格子深浅 = section_count）。

交互：

- 卡片 click → emit `selectTopic` → TagsPage 调既有 `openTopicOverviewDetectiveWall(topicId)` 开侦探墙 overlay 聚焦该 topic（项目「深挖 topic」标准动作）。
  - **后续优化（非本 change）**：若产品要「留在 topic-overview tab 内选中 topic、不开 overlay」，需给 BoardThreadBrowser 加 initial-topic prop（当前无，超最小改动）。
- 待激活卡片红色描边 + 角标「待激活」，hover 提示「够格转正，点话题管理激活」。
- 已归档分组默认折叠，带计数。

## 6. 空态

板块无日报（`board_daily_reports` 无该 board 记录）→ 接口返回 `topics=[]` 且 `vitality.trend=[]` → 前端渲染空态卡：

> 「该板块还没有日报，话题态势需要日报数据。 [生成日报]」

「生成日报」按钮触发 `POST /api/daily-reports/generate` `{board_id, date:今日}`（复用既有端点 + WS 进度）。

## 7. 不做的事（明确排除）

- ❌ 每日 top 标签榜 / 近 N 日最火标签（舆情榜单，违背定位）。
- ❌ section↔section 全局热力图（similarity 轨，不可靠）。
- ❌ today_top / 今日最热（日报 tab 的职责）。
- ❌ 自动归档停滞话题（违反「candidate→active→archived 全人工归档」不变量）。
- ❌ 改动任何既有状态机 / 锚定 / 日报生成逻辑。

## 8. 风险与缓解

| 风险 | 缓解 |
|---|---|
| 话题数很多时卡片墙过长 | 默认按 stance 分组 + 各组限高/折叠；已归档折叠；超阈值（如 >50 活跃）给「按命中数排序取 top」开关 |
| 历史未锚定 section（`persistent_topic_id=NULL`）多 | 不影响——态势只按已锚定 topic 聚合，未锚定自然不出现；符合既有语义 |
| `is_vacuum` 语义用户不懂 | 卡片角标 + tooltip 解释「强吸引：该话题近期吸走大量 section」 |
| lifeline 时区错位 | 用 `time.Local` 与日报生成一致，SQL 显式 `AT TIME ZONE` |
