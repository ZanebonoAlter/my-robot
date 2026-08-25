<!-- constraint-domains: daily-report, topic-graph -->
<!-- complexity: complex -->

## Why

现有 topic-watch（label/keyword 双轨）是只读覆盖层：命中只写 `topic_watch_hits` 并在日报顶部导航，用户仍要自己跳到对应 section 拼凑全貌。用户真正想要的是"日报里直接长出专属领地"——关键字场景把当天含关键字的文章（含 tag 体系漏网的）聚合为固定名称话题，一句话场景通过 embedding 检索当天相关辅助标签、聚合为一个能跨天延续的持久话题。watch 需要从管线旁观者变成参与者。

## What Changes

- **新增 watch 物化轨**（`board_topic_watches.type` 扩展两值）：
  - `keyword_topic`：日报生成时扫描当天全部未归档文章（Title + AIContentSummary/FirecrawlContent/Content/Description 择优，严格版文本层），按既有 DNF 关键字表达式匹配，命中文章机械聚合为一条固定名称 section（如「关键字『harness』相关话题」），每篇命中文章一条 thread，零 AI。不建持久话题、无跨天延续。
  - `sentence_topic`：watch 创建时将一句话 embedding 一次并缓存；日报生成时用该向量在 board 绑定的辅助标签池（BoardComposition + SemanticLabel.Embedding）做余弦检索 top-K，命中的辅助标签经 TopicTagSemanticLabel → 当天有文章的 event tag 聚合为一条 section，挂到该 watch 专属的 `source=manual` active 持久话题，享受既有生命周期（consecutive_hits、lane 锚定）。
- **`board_topic_watches` 加 `query` 列**：sentence 轨区分话题名（label，展示用）与检索句（query，embedding 用）；为空时回退用 label。
- **物化 section 打 `lane_tier=watch_keyword / watch_sentence`**，前端可区分样式；物化 section 不参与同日合并与 section 关系计算。
- **删除联动**：删除 sentence_topic watch 需用户确认并归档其持久话题（符合"仅用户显式操作可归档"红线）；删除 keyword_topic watch 仅停止后续物化，历史 section 保留。
- **旧轨不动**：label / keyword（提示轨）行为保持只读命中；EvaluateWatchHits 跳过物化轨（物化 section 不再重复产生命中提示）。收尾追加：**旧提示轨创建入口在前端退役隐藏**（新建对话框仅物化轨双选；存量 label/keyword 关注继续展示/暂停/删除，API 兼容保留）。
- **v1 不做历史回填**：物化从下一期日报开始生效，不往已保存日报补插 section。

## Capabilities

### New Capabilities

- `watch-materialized-topic`: 关注标记物化话题——keyword_topic / sentence_topic 两轨在日报生成管线内物化为 section 的完整行为（文章扫描口径、辅助标签检索、持久话题联动、降级与失败语义、计数口径）。

### Modified Capabilities

- `topic-watch`: 实体模型扩展（type 新增 keyword_topic/sentence_topic、新增 query 列及 embedding 缓存字段）；Purpose 的"只读"边界修订为"提示轨只读，物化轨另行物化"；CRUD API 语义扩展（创建带 type/query、删除 sentence 轨的确认归档联动）。
- `daily-report-system`: 日报生成编排流水线在同日合并之后插入"Watch 物化追加"步骤（失败不阻断、降级跳过）；lane_tier 取值集合扩展 watch_keyword / watch_sentence。

## Impact

- **后端**：`internal/topicgraph/service/daily_report_orchestrator.go`（管线追加物化 phase）、`daily_report_watch.go`（EvaluateWatchHits 跳过物化轨）、`keyword_match.go`（文章层扫描复用 DNF 匹配）、`repository/daily_report_register_models.go` + `topic_watch_repository.go`（新列/新 type）、`internal/platform/database/postgres_migrations.go`（type CHECK 扩值 + query/embedding 缓存列，新 migration）；sentence 轨建话题复用既有 CreateTopic/planLifecycle 机制（`daily_report_topic_repository.go`）。
- **前端**：watch 管理界面（类型选择、query 输入、删除确认弹窗）、日报时间线对 `lane_tier=watch_*` section 的样式区分、`app/api/topicWatches.ts` 类型扩展。
- **数据**：存量 watch 行不受影响（type 默认 label）；新列均可空，无破坏性迁移。
- **依赖**：无新外部依赖；sentence 轨 embedding 走既有 airouter.Embed，watch 创建时一次、非每期。
