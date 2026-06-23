## Context

系统当前存在两套并行的"embedding 配置"入口，但只有一套真正生效：

| 入口 | 存储位置 | 运行时读取 | 状态 |
|------|---------|-----------|------|
| 设置页 "Embedding" 栏（`EmbeddingConfigPanel` + 板块匹配阈值滑块） | `embedding_config` 表（high/low threshold、dimension、model）+ `ai_settings` 表（`narrative_board_embedding_threshold`） | 零 | 死配置 |
| 标签管理-匹配规则（`MatchingConfigDialog`） | `ai_settings` 表（`semantic_board_match_*` 系列） | `semantic_board_matching.go:443` | 活配置 |

死配置是 airouter 重构的遗留：迁移到 capability-routes 后，model/provider/dimension 由路由决定，旧的 `embedding_config` 表和对应 UI 面板成为孤儿。`EmbeddingConfigService.LoadMatchThreshold` 是半成品（写了从未被调用），实际阈值是 `embedding.go:34` 的硬编码 `MatchThreshold = 0.92`。

## Goals / Non-Goals

**Goals:**
- 移除所有死配置的 UI、API、表行，消除对用户的误导（尤其 `embedding_model` 的假 warning）。
- 固化"配置入口收敛"为 spec 契约，防止未来重构再次引入并行死入口。

**Non-Goals:**
- **不**改 `MatchThreshold = 0.92` 硬编码或接入任何可配置阈值——本次只做减法，不引入新行为。若未来需要让 tag 匹配阈值可调，应另立 change，且入口应落在 `semantic_board_match_*` 体系内。
- **不**删 `embedding_config` 表——`cluster_*` 系列（5 个 key）被 `tag_clustering.go` 真实使用，仅清死 key 行。
- **不**给 `cluster_*` 加 UI 入口——超出本次范围，当前依赖 seed 默认值运行正常。

## Decisions

### 决策 1：死 API 直接删除，不保留 410 Gone 响应
**选择**：直接移除 `GET/PUT /embedding/config` 路由和 handler。
**理由**：这些接口是单用户内部系统的一部分，无外部消费者；接口本身写入的值从不被读取，保留 410 只是无意义的兼容负担。
**备选（否决）**：保留路由返回 410 + deprecation header——增加复杂度，收益为零。

### 决策 2：数据清理走 `postgres_migrations.go` 机制
**选择**：新增一条 migration，`DELETE FROM embedding_config WHERE key IN (...)` 和 `DELETE FROM ai_settings WHERE key = 'narrative_board_embedding_threshold'`，幂等（`IF EXISTS` 语义，DELETE 天然幂等）。
**理由**：与现有 migration 体系一致，可追溯、可重放，无需手动 SQL。
**备选（否决）**：启动时自动清理脚本——破坏 migration 的单一职责，难以审计。

### 决策 3：前端配置入口收敛，不做引导迁移
**选择**：直接移除设置页 embedding 栏，不在原位置插入"请前往标签管理-匹配规则"的引导。
**理由**：两个入口本就独立，用户从未在死配置上获得过实际效果，不存在"迁移用户习惯"的需求。`MatchingConfigDialog` 的入口（标签管理页）已稳定存在。
**备选（否决）**：保留空壳面板加引导文案——增加维护负担，且面板文案可能再次过时。

### 决策 4：`cluster_*` 系列保持无 UI 状态
**选择**：本次不为 `cluster_*` 添加配置 UI。
**理由**：这些参数（`cluster_max_tags` 等）调优频率极低，依赖 seed 默认值即可；加 UI 会扩大本次清理的爆炸半径。记录在 design 中作为已知限制。

## Risks / Trade-offs

- **[风险] 误删 `embedding_config` 表中仍被使用的 key** → Mitigation：migration 的 DELETE 用精确的 4 个 key 名白名单（`high_similarity_threshold`、`low_similarity_threshold`、`embedding_dimension`、`embedding_model`），不使用通配；`cluster_*` 和 `event_cluster_*` 明确不在删除列表。
- **[风险] 前端有其他地方引用 `useEmbeddingConfigApi` 未清理干净** → Mitigation：删除后跑 `pnpm exec nuxi typecheck`（必须通过 Windows cmd）+ `pnpm lint`，编译期捕获遗漏引用。
- **[风险] 后端路由移除后有残留注册导致 panic** → Mitigation：`go vet ./...` + `go build ./...` + 针对性 `go test ./internal/tagmanagement/... ./internal/admin/...`。
- **[取舍] `cluster_*` 仍无 UI** → 用户无法在不改 DB 的情况下调整聚类参数。当前可接受（参数稳定，调优罕见）；若需要，后续单独立 change。
- **[取舍] `MatchThreshold = 0.92` 硬编码继续存在** → 本次不解决，但 design 明确记录其为"已知技术债"，避免被误以为已处理。
