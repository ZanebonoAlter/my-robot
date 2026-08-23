# tagging-domain Delta Spec

## MODIFIED Requirements

### Requirement: Tagging domain package structure

系统 SHALL 将标签相关代码组织为 `internal/domain/tagging/` 域，包含以下子包：

- `tagging/` 根层：标签生命周期编排（tagger.go）、共享类型（types.go）、共享 helper（helpers.go）、queue 入口（workers.go、tag_queue.go）
- `tagging/extraction/`：纯文本标签提取，输入文章文本，输出 `[]TopicTag`
- `tagging/analysis/`：主题分析 CRUD + 分析队列
- `tagging/embedding/`：向量化服务 + embedding/merge 队列
- `tagging/merge/`：标签合并、聚类、清理
- `tagging/watched/`：关注标签管理
- `tagging/semantic/`：辅助标签入库、SemanticBoard 匹配、升级建议、回填

编排层（tagger.go）SHALL 在进入提取前按 `articles.content_form` 分流：`aggregate` 走栏目切片 map-reduce 聚合路径，`mono` 与空值走原有提取路径。聚合路径的切片器为纯代码（无 LLM 调用），其逐片 LLM 提取与跨片去重属于编排层职责；`extraction/` 子包继续只做单次纯文本提取。

#### Scenario: 依赖方向全部单向

- **WHEN** 检查 tagging 域内所有子包的 import 关系
- **THEN** 依赖方向为：`extraction → helpers/embedding`、`merge → helpers/embedding`、`semantic → helpers/embedding`、`watched → helpers`；不存在任何反向依赖或循环依赖

#### Scenario: extraction 子包只做文本提取

- **WHEN** `extraction.ExtractTopics(input)` 被调用
- **THEN** 返回 `[]TopicTag` 原始标签列表，不执行 embedding 匹配、LLM 判断、合并或层级放置

#### Scenario: 按 content_form 分流

- **WHEN** 一篇 `content_form = 'aggregate'` 的文章进入打标编排
- **THEN** 走聚合路径（切片 → 逐片融合提取 → 跨片去重），`mono` 或空值文章走原有提取路径

## ADDED Requirements

### Requirement: 单主题打标输入与上限参数

mono 路径（含 content_form 为空的存量文章）SHALL 将摘要输入截断上限设为 4000 runes，文章级标签上限设为 6。存量文章（content_form 为空）SHALL 走 mono 路径且行为一致。

#### Scenario: 单主题长摘要截断

- **WHEN** 一篇 mono 文章的 AIContentSummary 长度为 7000 runes
- **THEN** 进入提取的输入为前 4000 runes

#### Scenario: 存量文章走 mono 路径

- **WHEN** 一篇 change 合并前入库、content_form 为空的存量文章被重新打标
- **THEN** 其处理路径与新 mono 文章一致（4000 截断、上限 6）
