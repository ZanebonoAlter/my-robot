## Why

新建泳道的编排态（`2026-07-05-manual-topic-lane` 引入的 `viewMode='compose'`）是**独立全屏视图**，与 lanes 泳道主视图互斥。用户选候选 section 时看不到已有 active 泳道全貌，无法对照避免重复/重叠；不满意要返回 lanes 重看再切回 compose ——「视图来回切换」，交互性与可观性差。本 change 把编排态搬到 lanes 主视图**就地完成**，编排态与浏览态合一。

## What Changes

- 编排态从独立全屏视图（`viewMode='compose'`）改为**就地叠加**在 lanes 泳道主视图上：新增 `composeMode` 布尔叠加态，**不改 `viewMode`**。
- 已有 active 泳道**淡显保留做背景参照**，不再被整体盖掉。
- unassigned（待确认/未分类）泳道成**主战场**，section 节点就地长 checkbox 勾选。
- 勾选实时算贴合度（cosine distance + `distanceTier` 分层 + 离群标黄）并标到节点上。
- 顶部浮工具条（泳道名 + 「聚类质量」单卡 + 保存/取消）；右侧滑出候选 topic 侧边栏（「采纳」预填名 + 预勾 matchThreshold 内最近 section）。
- **允许勾走 active 泳道 section**（= 从原泳道移出到新泳道），勾时实时提示「将从【泳道X】移出」+ 保存前二次确认列移出项。
- **BREAKING**：废弃 `ComposePanel.vue` 全屏编排视图及其入口（`viewMode='compose'`）。

## Capabilities

### New Capabilities

（无 —— 纯交互重构，不引入新能力）

### Modified Capabilities

- `section-lifecycle`: 编排态从独立全屏视图（`viewMode='compose'`）改为 lanes 就地叠加（`composeMode` 叠加态，不改 viewMode）；active 泳道淡显背景 + 可勾走移出；unassigned 主战场就地勾选；候选 topic 侧边栏采纳；贴合度（distance/tier/离群）+ 聚类质量卡就地实时呈现；废弃 ComposePanel 全屏编排视图入口。

## Impact

- **前端**
  - `BoardThreadBrowser.vue`: 新增 `composeMode` 叠加态；section 节点 checkbox + distance/tier 标注；active 泳道淡显 + 可勾走逻辑；unassigned 泳道头部「新建泳道」按钮。
  - 新增 `ComposeInlineToolbar.vue`（顶部浮工具条：名/计数/聚类质量卡/取消/保存）、`ComposeSidebar.vue`（候选 topic 侧边栏 + 采纳）。
  - **废弃** `ComposePanel.vue` + `viewMode='compose'` 入口。
  - 复用 `composeReport.ts`（纯逻辑：`cosineDistance`/`aggregatePreview`/`distanceTier`/`outlierFlags`）、`persistentTopics.ts` API（`getComposeCandidates`/`createManualLane`/`embedQuery`）。
- **后端**：无改动。API 与数据模型完全复用；`createManualLane(label, sectionIds)` 已支持 section 重指新 topic 实现移出，无需额外端点。
- **数据兼容**：无（纯前端交互重构，无 schema 变更）。
- **AI 成本**：无额外（贴合度是纯前端向量运算，复用已有 section embedding）。
