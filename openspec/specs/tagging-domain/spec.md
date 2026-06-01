# Tagging Domain

## Purpose

(TBD)

## Requirements

### Requirement: Tagging domain package structure

系统 SHALL 将标签相关代码组织为 `internal/domain/tagging/` 域，包含以下子包：

- `tagging/` 根层：标签生命周期编排（tagger.go）、共享类型（types.go）、共享 helper（helpers.go）、queue 入口（workers.go、tag_queue.go）
- `tagging/extraction/`：纯文本标签提取，输入文章文本，输出 `[]TopicTag`
- `tagging/analysis/`：主题分析 CRUD + 分析队列
- `tagging/embedding/`：向量化服务 + embedding/merge 队列
- `tagging/merge/`：标签合并、聚类、清理
- `tagging/watched/`：关注标签管理
- `tagging/semantic/`：辅助标签入库、SemanticBoard 匹配、升级建议、回填

#### Scenario: 依赖方向全部单向

- **WHEN** 检查 tagging 域内所有子包的 import 关系
- **THEN** 依赖方向为：`extraction → helpers/embedding`、`merge → helpers/embedding`、`semantic → helpers/embedding`、`watched → helpers`；不存在任何反向依赖或循环依赖

#### Scenario: extraction 子包只做文本提取

- **WHEN** `extraction.ExtractTopics(input)` 被调用
- **THEN** 返回 `[]TopicTag` 原始标签列表，不执行 embedding 匹配、LLM 判断、合并或层级放置

### Requirement: Domain package singular naming

系统 SHALL 使用单数包名：`feed/`、`article/`、`category/`、`content/`（取代 `feeds/`、`articles/`、`categories/`、`contentprocessing/`）。

#### Scenario: Import 路径全部更新

- **WHEN** 编译后端
- **THEN** 所有 `internal/domain/feeds` 引用变为 `internal/domain/feed`，以此类推，编译无错误

### Requirement: Models ownership by domain

系统 SHALL 将 `models/` 中的 struct 按域分散：
- `feed/model.go` 持有 Feed、FeedStats
- `article/model.go` 持有 Article
- `category/model.go` 持有 Category
- `content/model.go` 持有 FirecrawlJob
- `preferences/model.go` 持有 UserPreference、ReadingBehavior
- `aiadmin/model.go` 持有 AIProvider、AIRouteProvider、AISettings、AICallLog
- `narrative/model.go` 持有 NarrativeSummary、NarrativeBoard、BoardConcept
- `tagging/types.go` 持有 TopicTag 等核心标签实体
- `tagging/embedding/models.go` 持有 Embedding 相关 model，**其中 TopicTagEmbedding 不包含 Vector 文本字段**
- `tagging/semantic/models.go` 持有 SemanticLabel、BoardComposition 等语义标签 model
- `models/` 仅保留 SchedulerTask

#### Scenario: 改 TopicTag 不触发 feed 重编译

- **WHEN** 修改 `tagging/types.go` 中 TopicTag 的定义
- **THEN** `feed/`、`article/`（除直接引用 TopicTag 的代码外）不重编译

#### Scenario: 改 Feed 不触发 tagging 重编译

- **WHEN** 修改 `feed/model.go` 中 Feed 的定义
- **THEN** `tagging/` 及其子包不重编译

#### Scenario: TopicTagEmbedding 无 Vector 字段

- **WHEN** 检查 `TopicTagEmbedding` struct 定义
- **THEN** 不存在 `Vector` 或 `vector` 相关的 text 类型字段，仅保留 `EmbeddingVec`（pgvector 列）用于向量存储

### Requirement: Unified worker lifecycle

系统 SHALL 通过 `tagging/workers.go` 暴露 `StartAllWorkers()` 和 `StopAllWorkers()` 函数，统一管理 TagQueue、EmbeddingQueue、MergeReembeddingQueue、BackfillQueue 四个 worker 的启动和停止。

#### Scenario: runtime.go 收敛

- **WHEN** `runtime.go` 调用 `tagging.StartAllWorkers()` 和 `tagging.StopAllWorkers()`
- **THEN** 4 个 worker 按正确顺序启动和停止，功能等价于原来手动调用

### Requirement: No behavior change

重构 SHALL NOT 改变任何 API 路由、数据库 schema（除移除废弃列外）、业务逻辑或测试断言。所有 `go test ./...` 在重构后 MUST 全部通过。

#### Scenario: 全量测试通过

- **WHEN** 重构完成后运行 `go test ./...`
- **THEN** 所有测试通过，无编译错误

### Requirement: Embedding 写入只使用 pgvector 列

系统 SHALL 在生成 embedding 时仅将向量数据写入 `embedding` (vector(2560)) 列，不再写入冗余的 JSON 文本列。

#### Scenario: GenerateEmbedding 不写入 vector text

- **WHEN** `GenerateEmbedding` 或 `GenerateSemanticEmbedding` 创建新的 `TopicTagEmbedding` 记录
- **THEN** 仅设置 `EmbeddingVec`（pgvector 格式），不写入任何 JSON 文本向量数据

### Requirement: FindSimilarTags 使用原始浮点数组构建查询

系统 SHALL 在 `FindSimilarTags` 中直接使用 `GenerateEmbedding` 返回的原始 `[]float64` 构建 pgvector 查询字符串，不从数据库读取文本格式的向量数据再反序列化。

#### Scenario: 相似标签查询不依赖 vector 文本列

- **WHEN** `FindSimilarTags` 被调用以查找相似标签
- **THEN** 使用 `GenerateEmbedding` 返回的原始浮点数组通过 `floatsToPgVector` 构建 SQL 参数，不执行对 `vector` 列的读取或反序列化

### Requirement: 数据库 vector 列被移除

系统 SHALL 通过数据库 migration 从 `topic_tag_embeddings` 表中移除 `vector` text 列。

#### Scenario: vector 列不存在

- **WHEN** 检查 `topic_tag_embeddings` 表结构
- **THEN** 不存在名为 `vector` 的列
