## Context

日报 section 的顺序由 `best_tier`（升序）+ `avg_score`（降序）决定（`dailyReportMagazine.ts:64` 前端排序），但这两个标量怎么来、含义什么，对用户是纯黑盒。本 change 不动匹配算法，只把"生成时刻的匹配血缘"快照到 section 级并暴露。

数据流现状（带断点标注）：

```
topic_tag_board_labels (tagmanagement 域，可被 rematch 重写)
   列: match_reason, score, downgraded
   │  collectBoardTags (orchestrator.go:270)
   │  SELECT match_reason, score …  ⚠️ 不取 downgraded（事实1）
   ▼
[]TagInput{ MatchReason, Score }        ← ⚠️ 结构体无 Downgraded 字段
   │  filterTagsByQuality (orchestrator.go:386)
   │   截断排序 MatchTier(reason, false) ⚠️ 硬编码 downgraded=false
   ▼
ClusterTags(LLM) → ClusterGroup{ TagIDs, GroupName, MatchedTopicID }
   │                ⚠️ ClusterGroup 不带匹配明细，但 tags[] 切片仍在作用域
   ▼
section 组装 (orchestrator.go:164-192)   ★ 聚合发生处
   bestTier=min(tier), avgScore=mean(score)
   ⚠️ 每 tag 的 (reason,score,downgraded) 被压成 2 标量，明细丢弃
   ▼
MergeSimilarSections (merge.go:73)  按 embedding 距离合并
   合并重算 bestTier=min / avgScore=mean(从 tagScoreMap)   ← 规则一致
   ▼
持久化 DailyReportSection: best_tier, avg_score(冻结✓), cluster_tag_ids(jsonb✓)
   ⚠️ 无每 tag 的匹配明细列
   ▼
前端 sortDailyReportSections: best_tier 升序 + avg_score 降序（零解释展示）
```

约束：

- 后端 Go/Gin + GORM + PostgreSQL/pgvector；前端 Nuxt 4 + Vue 3。
- 本 change 只读消费 `tag-to-board-matching` / `match-score-visualization` 已建立的匹配质量语义（match_reason 四色系、降级标记、matchReasonColor/matchInfoLabel），不改匹配算法。
- 单用户、无 auth；日报正文目前零质量提示。

## Goals / Non-Goals

**Goals**

- section 级匹配血缘（每 tag 的 reason/score/downgraded）在生成时刻**冻结快照**到 section，可追溯、不随 rematch 漂移。
- 修掉 🔴 事实1：`downgraded` 不再在进管线时丢失；截断排序用真实降级标记。
- `MatchTier` 0/1/2/3 语义文档化为所有人话翻译的唯一依据。
- 探究区展示 tag 级明细（复用现有四色系 + 降级表现）。
- 日报正文出极简 tier 徽章（**仅色彩、无数字**，不破沉浸）。

**Non-Goals（明确不做）**

- 不动匹配算法 / embedding 区分度（归 `embedding-content-mismatch`）。
- 不建新表（用 JSON 列，复刻 highlights/raw_clusters 模式）。
- 不回刷历史 section（`quality_breakdown` 历史 NULL，前端降级）。
- 不改 `tag-to-board-matching` / `match-score-visualization` 的 spec 行为（只读消费）。
- 不动 `best_tier`/`avg_score` 的排序规则本身（前端排序逻辑不变）。
- 不在日报正文展示分数/百分比文字（避免破坏沉浸，分数只进探究区）。

## Decisions

### D1. 纯 topicgraph 域闭环，不跨域改匹配代码

**选**：所有改动在 `internal/topicgraph/`。日报管线已在读 `topic_tag_board_labels.match_reason/score`（collectBoardTags:270），本 change 只是补取 `downgraded` 并把明细快照到 section。

**理由**："跨域"是数据血缘上的跨域（日报消费 tagmanagement 产出），不是代码改动上的跨域。把改动收敛在 topicgraph 域，边界干净，不碰匹配算法（那是 embedding-content-mismatch 的活），不与兄弟 change 纠缠。

**备选（否决）**：跨域改 `evaluateSemanticBoardMatches` 输出结构。否决：风险高、与 embedding 治本纠缠，且日报管线本就在读这些字段，只需快照无需改匹配。

### D2. 数据下沉方案：section 加 `quality_breakdown` JSON 列（不建表、不实时 rejoin）

**选**：`daily_report_sections` 加 `quality_breakdown JSONB NULL`，结构：

```json
[
  {"tag_id": 16686, "label": "GPT-5发布", "match_reason": "max_sim", "score": 0.85, "downgraded": false},
  {"tag_id": 16884, "label": "AI芯片",   "match_reason": "direct_hit", "score": 1.0,  "downgraded": false},
  {"tag_id": 16903, "label": "AI竞赛",   "match_reason": "weighted",   "score": 0.59, "downgraded": false}
]
```

在 section 组装处（orchestrator.go:164）从当前作用域 `tags` 切片填充。`MergeSimilarSections` 合并后按合并后的 tagIDSet 重算（与重算 avgScore 同处）。

**理由**：
- 复刻 `BoardDailyReport.highlights` / `raw_clusters` 已有的 JSON 列模式，零新概念、零 join 读路径。
- 生成时刻冻结，**彻底消除 rejoin 漂移**（cluster_tag_ids rejoin topic_tag_board_labels 会被 rematch 重写，生成时的分 ≠ 现在的分）。

**备选（否决）**：
- 新表 `daily_report_section_tag_quality`：过重，一 section 最多 30 tag，建表不值，多一次跨表 join。
- 查询时 rejoin：省事但 score 漂移，且日报读路径多一次 join。

### D3. MatchTier 语义钉死（文档化地基）

`tagging.MatchTier`（`board_match_handler.go:403`）的实际映射，越小越好：

| tier | match_reason | downgraded | 含义（人话） |
|------|-------------|-----------|------------|
| 0 | direct_hit | — | 直接命中（board 构成标签交集达标） |
| 1 | hit_rate | — | 命中率达标（辅助标签命中率超阈值） |
| 2 | max_sim | false | 相似度达标（最相似辅助标签超阈值，非降级） |
| 3 | max_sim | true | 相似度降级匹配（辅助标签数不足，降了 minHits 阈值） |
| 3 | weighted | — | 综合加权达标（最弱规则，常作保底拉回） |

前端 `best_tier` 升序 = 把 direct_hit（tier=0）最多的 section 排最前。本映射在本 design 与 daily-report-system spec 双重钉死，作为所有徽章/人话翻译的唯一依据。

**修硬编码**：`filterTagsByQuality` 截断排序（orchestrator.go:407）由 `MatchTier(reason, false)` 改为 `MatchTier(reason, t.Downgraded)`，使降级 max_sim 正确落到 tier=3。

### D4. 暴露分层：探究区 tag 级明细 + 正文 tier 徽章（无数字）

**探究区（tag 级明细）**：section 详情/hover 展示该 section 全部来源 tag，复用 `match-score-visualization` 的四色系与降级表现：

- 颜色：direct_hit 绿(#22c55e) / hit_rate 蓝(#3b82f6) / max_sim 橙(#f59e0b) / weighted 灰(#94a3b8)。
- 降级（downgraded=true）：边框 50% 不透明 + 分数后 "↓"。
- 分数文字在探究区展示（如 "相似度 0.85↓"）。

工具函数 `matchReasonColor` / `matchInfoLabel` 从 TagsPage 语境上移到共享 utils（当前位于 tags feature 内），日报探究区复用。

**日报正文（tier 徽章，不破沉浸）**：每个 section 出一个极简徽章，**仅色彩、无数字文字**。本 change 唯一破兄弟 change"正文纯沉浸"原则的点，经克制处理。

四态视觉（由主题语义 token 派生，跟随 editorial/dark 双主题，不写死色值）：

| section.best_tier | 徽章形态 | 语义 |
|------------------|---------|------|
| 0 | 实心点，绿 token | 高质量（direct_hit 主导） |
| 1 | 实心点，蓝 token | 较高质量（hit_rate 主导） |
| 2 | 实心点，橙 token | 相似度主导 |
| 3 | 空心点，灰 token | 保底/降级（weighted 或降级 max_sim） |

形态只区分"实心 vs 空心"两档权重 + 四色，不出现任何数字/百分比文字。用户一眼看出相对层级，不被打扰。

**理由**：质量排序直接决定 section 先后——用户眼睛已看到"这条排第一"，给既成事实一个色彩提示属"解释"，不是"打扰"。分数文字会破坏沉浸，故只进探究区。

### D5. 与 embedding-content-mismatch / match-score-visualization / 兄弟 change 的边界

- **embedding-content-mismatch**（治本）：改匹配/embedding 区分度。本 change（装监控）只读暴露。保持分离，合并会失焦。
- **match-score-visualization**（TagsPage surface 的 tag 级可视化）：已实现得很全（chip 色、分数、最强匹配、MatchDetailPanel 降级/方向校验）。本 change 不改其 spec，只把日报 magazine surface 的可观测补上，工具函数复用。两者是"同一套匹配质量语义，两个展示面"。
- **topic-watchlist-observability**（兄弟 change）：归属可观测只动 topicgraph 域且数据已部分持久化；本 change 同域但数据血缘来自 tagmanagement 且需明细快照，关注点不同，已分离。两者共享"正文保持沉浸"原则，本 change 正文徽章是唯一破例点。

### D6. 放宽持久话题锚定的双重确认（治 task 2.1 的二阶副作用）

**选**：`planTopicAssignments`（`daily_report_assignment.go`）的 LLM 闸由“`matched_topic_id` 必须 == embedding 单一最近邻”放宽为“`matched_topic_id` 指向 **阈值内任一** topic”。锚定到 LLM 指向的那个 topic，使用其真实 embedding 距离。阈值复用 `MatchThreshold`（默认 0.30），无新参数。

**理由（事故）**：task 2.1 修了 `filterTagsByQuality` 截断排序的硬编码（`MatchTier(reason, false)` → `(reason, Downgraded)`），这本身是正确的（D3）。但它有一个 design Risks 未预料的二阶副作用：截断后进 LLM 聚类的 tag 集合变化 → LLM 聚类结果漂移 → `matched_topic_id` 落到 embedding 第 2 近的 topic。旧双重确认要求 `matched_id == 最近邻`，这种漂移即判定失败 → section 误开新 candidate → 持久话题血缘大面积断裂 → 前端“全是突发话题”。

实测数据（06-25 重新生成）：section 到最近 active topic 的 embedding 距离分布与历史一致（p50≈0.27，阈值内 38/53），**embedding 一侧完全健康**；崩的纯粹是 LLM 一致性闸门（`anchor_hit` 从历史 ~12/板 跌到 0~2/板）。证明病根在“对聚类漂移过度敏感”，不在 embedding 或截断改动本身。

**两道闸门保留**：embedding 距离 ≤ 阈值 AND LLM 指向同一 topic。放宽的只是“最近”→“阈值内”。这 **不是** 纯 embedding 匹配（仍要 LLM 认），也 **不是** 纯 LLM 匹配（仍要 embedding 阈值内）。

**备选（否决）**：
- 回滚 task 2.1（改回 `MatchTier(reason, false)`）：放弃 D3 的硬编码修复，且没解决双重确认脆弱性，下次聚类一变还崩。
- 纯 embedding 匹配（去掉 LLM 闸）：误匹配风险高，双重确认设计意图全丢。
- 开新 change 修：被否决——病根由 task 2.1 触发，合并进本 change 作为衍生修复更内聚（用户决策）。

**数据修复**：06-25 已受影响的 section 按本逻辑等价规则（embedding ≤0.30 重指最近 active）一次性救回 36 行，清理 48 个孤儿 candidate，保留 14 个合法新 candidate。

## Risks / Trade-offs

- **[quality_breakdown 体积]** → 一 section 最多 30 tag，每条约 80-120 字节，单 section 上限约 3.5KB。缓解：JSONB 原生压缩；日报 section 数通常个位数到十几，整体可忽略。
- **[历史 section 无明细]** → `quality_breakdown` 历史 NULL，前端降级为"无质量明细"。与兄弟 change 的 matched_topic_id NULL 降级模式一致。
- **[tier 徽章破沉浸的边界]** → 即便无数字，色点仍是对正文的视觉注入。缓解：形态克制（仅实心/空心 + 四色）；如验收发现打扰阅读，可在 design 后期加开关默认关闭。**这是需要人工视觉验收的点**（tasks 含 cmd 浏览器验收）。
- **[MatchTier 改硬编码的副作用]** → 截断排序改用真实 downgraded 后，降级 max_sim tag 从 tier=2 落到 tier=3，可能改变 >30 tag 时的截断边界。缓解：降级 tag 本就该排后，这是修正而非回归；tasks 含对比截断前后结果的测试。
- **[⚠️ 已发生·截断改动的二阶副作用：打散持久话题血缘]** → 截断后进 LLM 聚类的 tag 集合变化，令 LLM 的 `matched_topic_id` 漂移到 embedding 第 2 近的 topic；旧双重确认（`matched_id == 最近邻`）大面积失败，section 误开新 candidate，前端“全是突发话题”。**已治本**（D6 放宽双重确认为“阈值内任一”）+ 一次性数据修复。教训：截断/聚类输入的任何改动都会经“双重确认”放大成话题血缘断裂，design 评估 scope 时须把这条传导链纳入。
- **[复用工具函数的迁移风险]** → `matchReasonColor`/`matchInfoLabel` 上移共享 utils，原 TagsPage 调用点需同步改 import。缓解：codegraph impact 确认调用面；纯位置搬迁不改逻辑。

---

## 6. 项目执行规范约束（实现期强制遵循）

本变更实现期必须遵守 `docs/reference/开发执行规范.md` 与前端架构文档，以下列出与本次强相关的约束（非全量复述）：

### 6.1 后端（§4）

- **业务逻辑按域组织**：所有改动在 `internal/topicgraph/`，handler 薄封装、业务逻辑不在 handler/router 内（§4.4）。
- **Handler 响应格式统一**：`gin.H{"success": bool, "data"|"error"|"message": ...}`（§4.4）。
- **错误包装**：`fmt.Errorf("context: %w", err)`，禁止 panic（§4.4）。
- **JSON snake_case**（§4.4）。
- **测试双层（§4.2）**：
  - `quality_breakdown` JSON 组装、MatchTier 真实 downgraded 调用、filterTagsByQuality 截断分支等**纯逻辑** → 内存 SQLite（`glebarez/sqlite` mode=memory，参考 `feed_service_test.go`）。
  - `quality_breakdown` 列迁移幂等 / 历史行 NULL 兼容 → testcontainer pgvector（`testutil.SetupTestDB(t)`）。
- **迁移**：版本化迁移（`platform/database/postgres_migrations.go`），幂等、有回滚路径（§10）。
- **质量门禁**：`golangci-lint run && go vet && go test && go build`（§4.1）。测试只跑本次修改影响的包（`internal/topicgraph`）。
- **变更控制（§8）**：apply 阶段禁止改 proposal 需求范围；需变更走 delta。

### 6.2 前端（§5 + 架构文档）

- **双主题系统**：editorial（暖白）+ dark（深色），通过 `<html data-theme>` 切换。tier 徽章四态颜色、探究区明细色 MUST 由语义 token（`--color-*`）派生，不写死色值；match_reason 四色复用 `match-score-visualization` 既有定义。
- **统一组件库**：徽章/明细若需交互，用 `AppButton`/`AppDialog`，禁止原生 button 样式类、禁止 `window.alert/prompt/confirm`。
- **API 边界归一**：所有 HTTP 经 `app/api/client.ts` 的 `ApiClient`，不在组件里直接 fetch。
- **命名转换**：snake_case → camelCase 在 normalizer 层（`camelizeKeys()`）完成，模板/组件内只用 camelCase；数字 ID 在 API 边界转字符串。
- **`<script setup lang="ts">`** Composition API（§5.4）。
- **质量门禁**：`pnpm lint && nuxi typecheck && pnpm test:unit && pnpm build`，其中 typecheck/build MUST 经 Windows cmd（AGENTS.md）。

### 6.3 架构体检（§7，强制）

- 每个子任务完成后跑 `codegraph impact <符号>` + `codegraph affected <文件>`；HIGH/CRITICAL 风险必须暂停报告。
- 重点核：`filterTagsByQuality`（MatchTier 调用变更）、section 组装处、`matchReasonColor`/`matchInfoLabel` 上移后的调用面。
- **已知局限**：codegraph 追不到 Gin `group.POST(..., fn)` 注册，但本 change 不新增路由，无此风险。

### 6.4 数据兼容性（§10）

本变更含 DDL（`daily_report_sections.quality_breakdown` 列），必须：
- 既有数据兼容（历史 section NULL 不报错）。
- 迁移可重复执行（幂等）。
- GORM 模型字段变更不破坏 JSON 响应格式（quality_breakdown 为可空新增字段，向后兼容）。
- 列回滚路径明确（DROP COLUMN 可逆）。

### 6.5 文档流转（§12）

- `docs/reference/`（api / database / architecture）在**里程碑收尾时**统一更新，不在本 change 内逐条改活文档。
- 本 change tasks 的文档节列出待更新 reference 清单，标注「里程碑收尾」。
