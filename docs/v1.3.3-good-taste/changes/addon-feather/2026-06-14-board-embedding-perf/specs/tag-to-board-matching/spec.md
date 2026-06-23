## Purpose (Delta from `tag-to-board-matching`)

优化 `MatchTopicTag` 匹配流程：增加缓存避免重复 DB 加载，backfill 改为批处理并行。

## Requirements

### Requirement: MatchTopicTag 使用缓存数据

`MatchTopicTag` SHALL 从 board match cache 获取 board auxiliaries、board embeddings 和 AI config，而非每次调用都查询数据库。缓存未命中时由缓存层负责加载。

#### Scenario: Cached board data used in match

- **WHEN** `MatchTopicTag` 被调用且 board auxiliaries 缓存有效
- **THEN** 系统 SHALL 使用缓存的 board auxiliaries 和 board embeddings 进行匹配，不执行 `loadBoardAuxiliaries` 或 `loadBoardEmbeddings` 的 DB 查询

#### Scenario: Cache transparent on miss

- **WHEN** `MatchTopicTag` 被调用且缓存为空（首次调用或失效后）
- **THEN** 缓存层 SHALL 自动从 DB 加载数据、缓存并返回，`MatchTopicTag` 逻辑无需感知缓存存在

### Requirement: Backfill 批处理并行

`processJob` SHALL 使用 `errgroup.Group` 并行处理 topic tags，并发数由 `semantic_board_backfill_concurrency`（默认 4）控制。单个 tag 失败 SHALL NOT 取消整个 job。

#### Scenario: Parallel processing with default concurrency

- **WHEN** backfill job 包含 100 个 tags，`semantic_board_backfill_concurrency=4`
- **THEN** 系统 SHALL 最多同时处理 4 个 tag，前一个完成即开始下一个

#### Scenario: Individual failure does not cancel job

- **WHEN** tag #123 匹配失败（如 embedding 缺失），但其他 tags 正常
- **THEN** 系统 SHALL 记录 tag #123 的失败，继续处理剩余 tags

#### Scenario: Concurrency configurable via ai_settings

- **WHEN** 用户将 `semantic_board_backfill_concurrency` 从 4 改为 8
- **THEN** 下一个 backfill job SHALL 以最多 8 个并发处理 tags

#### Scenario: All tags processed despite failures

- **WHEN** backfill job 包含 50 个 tags，其中 3 个失败
- **THEN** 系统 SHALL 处理全部 50 个 tags，最终报告 Processed=47, Failed=3，状态为 completed
