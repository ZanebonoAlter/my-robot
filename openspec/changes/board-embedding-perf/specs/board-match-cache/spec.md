## Purpose

Board matching 的内存缓存层，缓存版块辅助标签、embedding 和 AI 配置，减少重复 DB 查询和 parsePgVector 开销。

## Requirements

### Requirement: Board Auxiliary Labels Cache

`SemanticBoardMatchingService` SHALL 缓存 `loadBoardAuxiliaries` 的结果。首次调用时从 DB 加载并缓存；后续调用直接返回缓存值。缓存 SHALL 在 board composition 变更（upgrade confirm、手动合并）后失效。

#### Scenario: Cache miss loads from DB

- **WHEN** `MatchTopicTag` 首次调用时缓存为空
- **THEN** 系统 SHALL 从 DB 加载 board auxiliaries，存入缓存，并返回结果

#### Scenario: Cache hit returns cached data

- **WHEN** `MatchTopicTag` 第二次调用时缓存有效（未失效）
- **THEN** 系统 SHALL 直接返回缓存的 board auxiliaries，不执行 DB 查询

#### Scenario: Cache invalidates on upgrade confirm

- **WHEN** 用户确认一个 board upgrade 建议（create_new 或 merge_into_existing）
- **THEN** 系统 SHALL 清除 board auxiliaries 缓存，下次 `MatchTopicTag` 调用时重新从 DB 加载

### Requirement: Board Embeddings Cache

`SemanticBoardMatchingService` SHALL 缓存 `loadBoardEmbeddings` 的结果（已解析为 `map[uint][]float64`）。缓存 SHALL 在 board composition 变更后失效，与 board auxiliaries 缓存同生命周期。

#### Scenario: Cache eliminates parsePgVector overhead

- **WHEN** `MatchTopicTag` 第二次调用时 board embeddings 缓存有效
- **THEN** 系统 SHALL 直接返回缓存的 `map[uint][]float64`，不执行 DB 查询和 `parsePgVector` 解析

#### Scenario: Cache invalidates with auxiliaries

- **WHEN** board composition 变更触发缓存失效
- **THEN** 系统 SHALL 同时清除 board embeddings 缓存

### Requirement: AI Config Cache (TTL)

`SemanticBoardMatchingService` SHALL 缓存 `loadConfig` 的结果，TTL 为 5 分钟。过期后下次调用重新从 `ai_settings` 加载。

#### Scenario: Config returned from cache within TTL

- **WHEN** 距上次 `loadConfig` 加载不到 5 分钟
- **THEN** 系统 SHALL 返回缓存的 `SemanticBoardMatchConfig`，不查询 `ai_settings`

#### Scenario: Config reloaded after TTL

- **WHEN** 距上次 `loadConfig` 加载超过 5 分钟
- **THEN** 系统 SHALL 重新从 `ai_settings` 加载 13 行配置，更新缓存并重置 TTL

### Requirement: Active Auxiliary Labels Cache (TTL)

`AuxiliaryLabelService` SHALL 缓存 `loadActiveAuxiliaryLabels` 的结果，TTL 为 5 分钟。过期后下次调用重新从 DB 加载。

#### Scenario: Labels returned from cache within TTL

- **WHEN** 距上次 `loadActiveAuxiliaryLabels` 加载不到 5 分钟，且 `ResolveAuxiliaryLabel` 被调用
- **THEN** 系统 SHALL 使用缓存的标签列表进行匹配，不执行全表扫描

#### Scenario: Labels reloaded after TTL

- **WHEN** 距上次加载超过 5 分钟，且 `ResolveAuxiliaryLabel` 被调用
- **THEN** 系统 SHALL 重新执行 `loadActiveAuxiliaryLabels` 全表扫描，更新缓存

### Requirement: Thread Safety

所有缓存 SHALL 使用 `sync.RWMutex` 保护，支持并发读和互斥写。多个 goroutine 同时调用 `MatchTopicTag`（backfill 并行场景）SHALL NOT 产生数据竞争。

#### Scenario: Concurrent reads during parallel backfill

- **WHEN** 4 个 backfill worker 同时调用 `MatchTopicTag`
- **THEN** 所有 worker SHALL 成功读取缓存，无 panic 或数据竞争

#### Scenario: Concurrent read and cache invalidation

- **WHEN** 一个 goroutine 正在读取缓存，另一个 goroutine 触发缓存失效
- **THEN** 读操作 SHALL 返回失效前的旧数据或等待失效完成后重新加载，不产生竞争条件
