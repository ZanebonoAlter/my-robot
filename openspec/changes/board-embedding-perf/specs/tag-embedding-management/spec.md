## Purpose (Delta from `tag-embedding-management`)

优化 `parsePgVector` 调用，通过 board embeddings 缓存消除重复解析。

## Requirements

### Requirement: Board Embeddings 缓存存储已解析数据

Board embeddings 缓存 SHALL 存储 `map[uint][]float64`（已解析的 float64 切片），而非原始 pgvector 字符串。缓存加载时执行 `parsePgVector`，后续读取直接使用解析结果。

#### Scenario: parsePgVector called once per cache load

- **WHEN** board embeddings 缓存首次加载（或失效后重新加载），DB 中有 200 个 board embeddings
- **THEN** 系统 SHALL 调用 `parsePgVector` 恰好 200 次（每个 embedding 一次），结果存入缓存

#### Scenario: Subsequent reads skip parsePgVector

- **WHEN** 10 次 `MatchTopicTag` 调用发生在缓存有效期内
- **THEN** 系统 SHALL NOT 为 board embeddings 调用 `parsePgVector`，直接使用缓存的 `[]float64`

### Requirement: Tag Embedding 不缓存

Tag identity embedding 和 tag auxiliary embeddings SHALL NOT 被缓存。每次 `MatchTopicTag` 调用 SHALL 从 DB 加载当前 tag 的 embedding 数据（按 topic_tag_id 索引查询，开销极低）。

#### Scenario: Tag embedding loaded per call

- **WHEN** `MatchTopicTag` 被连续调用 100 次处理不同 tags
- **THEN** 系统 SHALL 对每次调用执行 tag embedding 的 DB 查询，不缓存 tag embedding
