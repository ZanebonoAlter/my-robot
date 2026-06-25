# 语义版块流程（Semantic Board）

> 大功能：辅助标签入库、SemanticBoard 匹配/升级/回填、叙事面板。
> 跨端。互补：`flow/daily-report.md`、`flow/topic-graph.md`。

## 辅助标签入库（L1/L2/L3）

```mermaid
flowchart TD
  TAG[tagging/extraction: LLM 提取 tag + 3-5 辅助标签] --> SVC[auxiliary_label_service.go]
  SVC --> L1{L1: slug/alias 精确匹配}
  L1 -->|命中| R1[复用 ref_count++]
  L1 -->|未命中| L2{L2: embedding ≥ 0.95}
  L2 -->|命中| R2[小方加入 aliases ref_count++]
  L2 -->|未命中| L3[L3: 新建 semantic_label<br/>label_type=auxiliary + 生成 embedding]
  R1 & R2 & L3 --> REL[topic_tag_semantic_labels 记录关联]
```

## SemanticBoard 匹配

```text
semantic_board_matching.go
  → 读取 tag 辅助标签 + active Board composition
  → 直接命中 / 命中率 / max_sim / 加权综合（三规则）
  → 写入 topic_tag_board_labels（最多 3 个 Board）
```

## 升级建议

```mermaid
sequenceDiagram
  participant U as 用户
  participant FE as 前端
  participant BE as backend
  U->>FE: 触发升级建议
  FE->>BE: POST /api/semantic-boards/upgrade-suggest
  BE->>BE: 预聚类 + co-tag 上下文 + LLM 判断
  BE-->>FE: create_new / merge_into_existing / skip
  U->>FE: 确认
  FE->>BE: POST /api/semantic-boards/upgrade-execute
  BE->>BE: 创建/更新 SemanticBoard + board_composition
  BE-->>FE: 可触发回填
```

## SemanticBoard 管理 / 回填

```text
SemanticBoard 管理面板
  → 辅助标签入库: L1 slug匹配 → L2 embedding合并 → L3 新建
  → SemanticBoard 匹配: 三规则挂载 → topic_tag_board_labels
  → 升级建议: 手动触发 → 预聚类 → LLM建议 → 用户确认
  → 辅助标签治理: 禁用、alias合并、composition移除
  → 回填: all / unassigned / board 三种模式
```

## 叙事面板数据流

```text
NarrativePanel
  → loadBoardTimeline(date) → GET /api/narratives/boards/timeline
  → loadScopes(date) → GET /api/narratives/scopes
  → loadNarratives(date) → GET /api/narratives?date=...
  → switchScope('category') → loadScopes → 展示 board_count
  → triggerGeneration() → POST /api/narratives/regenerate

SemanticBoardPanel
  → loadBoards() → GET /api/semantic-boards
  → viewBoard(id) → GET /api/semantic-boards/:id
  → viewComposition(id) → GET /api/semantic-boards/:id/composition
  → viewUpgradeCandidates() → GET /api/semantic-boards/upgrade-candidates
  → upgradeSuggest() → POST /api/semantic-boards/upgrade-suggest
  → upgradeExecute() → POST /api/semantic-boards/upgrade-execute
  → backfill() → POST /api/semantic-boards/backfill
```

## 代码入口

- 后端：`internal/tagmanagement/{handler,service}/`（auxlabel、board：backfill/matching/upgrade/clustering）
- 前端：`front/app/features/tags/`（SemanticBoardPanel、NarrativePanel）

## Event 标签延迟 embedding

```text
Event 标签延迟 embedding: 描述+关键词生成后入队
  → 多行 embedding (semantic + event_keyword)
```

Event 类标签不随入库立即向量化，而是等描述与关键词生成后才入队，产出 semantic 与 event_keyword 两路 embedding。

## 资料来源

迁自原 `architecture/data-flow.md`（叙事数据流·SemanticBoard 管理 / 辅助标签与 SemanticBoard 数据流 / 叙事面板数据流）。
