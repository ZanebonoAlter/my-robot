# Design: preference-vector-feed-discovery

## Context

旧「阅读偏好」是 write-only 死功能（探索结论）：

- 面板显示 `read_score`/`interest_score`，后端模型只有 `preference_score`，永远不返回 → UI 恒为 0；
- 偏好分数零消费：排序、AI 总结、前端 `getPreferenceScore()` 均无调用；
- 文档宣称「影响排序与 AI 总结参考」的闭环从未写码。

但地基极好，本设计尽量复用现成资产：

| 资产 | 位置 | 复用方式 |
| ---- | ---- | -------- |
| 阅读行为采集 | `reading_behaviors` + `useReadingTracker` | 偏好权重数据源（保留不动） |
| 文章↔标签 | `article_topic_tags` | 互动文章的标签集合 |
| 标签↔版块 | `topic_tag_board_labels`（四规则匹配产物） | 偏好分桶到 SemanticBoard |
| 标签向量 | `topic_tag_embeddings`（identity/semantic 双轨 + text_hash） | 偏好向量的加数，**零新 embedding 调用** |
| 板块向量 | SemanticBoard embedding（board-direction-check 引入） | 冷启动种子落版块 |
| embedding 通路 | `airouter.EmbeddingRequest` + `embedding_config` | 路由 embedding / 问答 embedding |
| LLM 通路 | `airouter` 主路由 + `aisettings` | 推荐精排、问答 |
| 调度框架 | `internal/admin/scheduler` registry | 偏好重算 / 目录同步两个新 job |
| 订阅落地 | `POST /feeds`、`POST /feeds/fetch` | 一键订阅 / 填参验证 |
| RSSHub 实例 | 自建 `47.110.71.194:1200`（实测 `/api/namespace` 返回 3245 条路由全量元数据，91.9% 带 example） | 路由目录来源 |

## Goals / Non-Goals

**Goals:**

- 偏好 = 按 SemanticBoard 聚合的 embedding 向量（`preference_vectors`），scheduler 定期重算，画像可读。
- RSSHub 路由目录本地化：同步、参数标记、可用性校验、embedding。
- 推荐流：向量粗筛 → LLM 精排 → 卡片状态机，手动刷新，可订阅落地。
- 问答式交互：即时推荐 + 种子偏好（冷启动）。
- 旧偏好功能（`user_preferences` 表/scheduler/端点/面板）整体废弃。

**Non-Goals:**

- 不改文章排序、AI 总结 prompt（偏好消费闭环本期只做「源发现」这一个方向）。
- 不做非 RSSHub 源的发现（普通网站 RSS autodiscovery 是另一问题）。
- 不做推荐的自动定时推送（手动刷新为主）。
- 不爬 RSSHub 官网文档站；目录唯一来源是自建实例 API。
- 不改动 `reading_behaviors` 采集链路本身。

## Decisions

### D1：偏好向量 = 行为加权的标签向量质心，按版块分桶

对每个 SemanticBoard（+ 一个 `board_id=NULL` 全局桶）：

```
vec(board) = normalize( Σ  w(tag) × tag_embedding(tag) )
w(tag)     = Σ over 互动文章  behavior_weight × exp(-days/30)
behavior_weight: favorite=1.0 / 深读(scroll≥80% 或 time≥120s)=0.6 / 普通 open=0.3
```

- 标签取自用户互动文章的 `article_topic_tags`；标签→版块归属取 `topic_tag_board_labels`（一标签最多 3 版块，每个归属都计入）；未挂任何版块的标签只进全局桶。
- tag_embedding 取 `topic_tag_embeddings` 的 semantic 轨（与板块向量同空间）。
- **全量重建、幂等**（沿用旧 `UpdateAllPreferences` 哲学）：每次重算清空重算，表始终是行为数据的纯派生。
- 备选方案（否决）：直接对文章正文 embedding——成本高一两个数量级，且标签已经是 LLM 提炼过的语义单元，信噪比更好。

### D2：路由目录来源 = 自建实例 `/api/namespace`，定时同步 + hash diff

- 实测该端点返回全量路由元数据（path/name/url/parameters/description/example/maintainers），无需爬官网。
- 同步 job（默认每日）：全量拉取 → 按 `content_hash`（namespace+path+name+description+parameters 的 hash）diff → 新增/变更入库，消失的标 `gone`。
- 实例地址走配置（`rsshub_base_url`，默认取现有 dump-sanitizer 已知的自建实例）。

### D3：参数需求在入库时用数据库字段标记（用户拍板）

解析 path 中的 `:param` 段：

- `usable_directly` = path 无参数段，或所有参数段均带 `?`（可选）；
- `requires_parameters` = 存在不带 `?` 的 `:param`。

推荐层按此分流：`usable_directly` 卡片一键订阅（example 或 path 直拼实例 host）；`requires_parameters` 卡片展示 `parameters` 元数据（中文说明，目录自带）提示用户填写，填后走 `POST /feeds/fetch` 验证再 `POST /feeds`。

### D4：可用性校验 = example 路径异步限流 GET

- 91.9% 路由带 example；校验 worker 以低并发（如 2 req/s，可配）对 example 发 GET，超时/非 200/空 items → `status=broken`，否则 `ok`；无 example 的保持 `unknown`。
- 校验不进推荐粗筛的硬过滤（`unknown` 仍可被推荐，卡片标注「未验证」），`broken` 硬排除。避免校验覆盖率绑架推荐质量。

### D5：推荐 = pgvector 粗筛 + LLM 精排，两段式

1. **粗筛**：每个有偏好向量的版块，用 pgvector `<=>` 对 `route_embeddings` 取 top-N（默认每版块 8 条，可配），排除 `status=broken`、**已接受或已 dismiss 的 route（按 route_id 维度状态机去重，`requires_parameters` 路由无需预判最终 url）**、`usable_directly` 路由额外按 `feeds.url` 去重、dismiss 冷却期内的。
2. **精排**：候选路由元数据（name/ns/description/parameters 摘要）+ 版块画像摘要（版块名 + top 标签）给 LLM，输出保留子集 + 每条一句推荐理由。
3. 落 `feed_recommendations`：`recommendation_hash`（route_id+board_id）幂等，同 hash pending 不重复入库。**hash 不含 source**：`qa` 与 `manual_refresh` 共享同一幂等池——qa 先占坑的 route+board，手动刷新不再重复入库（符合「同源不重复推荐」语义），反之亦然；dismiss 冷却同样跨 source 生效（用户拒绝的源换皮不再推）。此为预期，非 bug。

备选（否决）：纯向量直出不要 LLM——推荐理由是用户判断「订不订」的核心信息，且参数类路由需要 LLM 把参数说明翻译成人话。

### D6：推荐卡片状态机 + dismiss 冷却（借鉴 board-upgrade 模式）

```
pending → accepted（订阅成功，记录 feed_id）
pending → dismissed（冷却期默认 30 天，期内同 hash 不再入库）
```

沿用 `board_upgrade_suggestions` 已验证的 `suggestion_hash` 幂等 + `CountDismissedInCooldown` 模式，不发明新轮子。**幂等与冷却均按 `recommendation_hash`（route_id+board_id）跨 source（qa/manual_refresh）生效**——同一条源无论由问答还是刷新首次产出，都只占一个 pending 坑，dismiss 一次即对该 route+board 跨全部 source 冷却（见 D5）。

### D7：问答 = 即时检索直返 + 种子偏好写库

- 用户提问 → 问题文本 embedding → 对 `route_embeddings` 粗筛 → LLM 精排 → 即时返回（同时以 `source=qa` 落推荐表，接受/拒绝走同一状态机）。
- 同时：问题文本 embedding 与板块向量匹配，相似度 ≥ 阈值（默认 0.5，可配）落到对应版块、否则落全局桶，以 `source=seed` 写入 `preference_vectors`。**种子按加权合并累积**（保 `UNIQUE(board_id, source)` 单行）：upsert 时 `new_vec = normalize(α×incoming + (1−α)×existing)`，α 默认 0.4（可配）；`tag_weights` 同步合并 top 列表。行为重算 MUST NOT 覆盖 `source=seed` 行——种子与行为分分行，重算只动 `source=behavior` 行，`source=seed` 行由问答独立维护。
- 冷启动路径就此闭环：无行为数据 → 问答 → 种子偏好 → 立即可刷新出推荐。

### D8：两个新 scheduler job，沿用 registry 模式

| job | 默认间隔 | 职责 |
| --- | -------- | ---- |
| `preference_profile_update` | 3600s | D1 全量重算（零 LLM，纯 SQL+向量算术） |
| `rsshub_catalog_sync` | 每日 | D2 同步 + D4 增量可用性校验 + 新路由 embedding（走 `EmbeddingQueue` 同款队列模式） |

推荐生成**不做** job，手动刷新（`POST /discovery/recommendations/refresh`）。

### D9：旧功能废弃清单

删除：`models/user_preference.go`、`admin/service/preferences_service.go`、`admin/handler/preferences_handler.go`（user-preferences 部分）、`admin/scheduler/job_preference_update.go`、runtime/router/routes/wire 相应注册、前端 `ReadingPreferencesPanel.vue` / `useReadingPreferences.ts` / `stores/preferences.ts` / `types/reading_behavior.ts` 的 `UserPreference`。设置 `preferences` section 保留但换成新「兴趣画像」视图。

## Data Model

```sql
preference_vectors(
  id, board_id NULL REFERENCES semantic_boards,  -- NULL = 全局桶
  source varchar,            -- behavior | seed
  embedding vector, dimension int, model varchar,
  tag_weights jsonb,         -- 画像可视化用：{tag_label: weight} top 列表
  last_computed_at, created_at, updated_at,
  UNIQUE(board_id, source)
)

rsshub_routes(
  id, namespace varchar, path varchar, name varchar, url varchar,
  description text, parameters jsonb, example varchar,
  requires_parameters bool, usable_directly bool,
  content_hash varchar,                        -- D2 diff
  status varchar,                              -- unknown | ok | broken | gone
  last_checked_at timestamptz NULL, created_at, updated_at,
  UNIQUE(namespace, path)
)

route_embeddings(
  id, route_id REFERENCES rsshub_routes,
  embedding vector, dimension int, model varchar, text_hash varchar,
  created_at, updated_at, UNIQUE(route_id)
)

feed_recommendations(
  id, route_id REFERENCES rsshub_routes, board_id NULL,
  source varchar,            -- manual_refresh | qa
  score float, llm_reason text,
  status varchar,            -- pending | accepted | dismissed
  accepted_feed_id NULL REFERENCES feeds,
  recommendation_hash varchar UNIQUE,          -- D6 幂等
  dismissed_at timestamptz NULL, created_at, updated_at
)
```

## API 草案

| 端点 | 用途 |
| ---- | ---- |
| `GET /api/preference-profile` | 兴趣画像（各版块 top 标签+权重+向量新鲜度） |
| `POST /api/preference-profile/recompute` | 手动触发重算 |
| `GET /api/discovery/recommendations?status=pending` | 推荐卡片列表 |
| `POST /api/discovery/recommendations/refresh` | 换一批（粗筛+精排，幂等落库） |
| `POST /api/discovery/recommendations/:id/accept` | 接受 `{category_id?, parameters?}` → fetch 验证 → CreateFeed |
| `POST /api/discovery/recommendations/:id/dismiss` | 拒绝（冷却） |
| `POST /api/discovery/ask` | 问答 `{question}` → 即时推荐 + 种子写入 |
| `POST /api/discovery/catalog/sync`、`GET /api/discovery/catalog/status` | 目录同步运维 |

## Risks / Trade-offs

- [标签 embedding 维度与新增向量维度不一致（换过 embedding 模型）] → 入库记 `dimension`/`model`，重算/粗筛前校验同维同模型，不一致则**报错并阻断该轮重算/粗筛**（不静默算错）。恢复路径：以当前 `embedding_config` 的模型/维度为准——`topic_tag_embeddings` 旧轨由其既有重嵌链路（text_hash 变更入队）刷新；`route_embeddings` 同理走 embedding 队列重嵌；`preference_vectors` 的 `source=behavior` 行在下一次重算自动按新 tag 向量重建（纯派生）；`source=seed` 行**非派生不可自动重建**，需重新问答或手动触发种子重嵌（重嵌时清空对应 seed 行让用户重新表达，避免用错维度向量污染画像）。
- [RSSHub 实例不可达导致目录同步失败] → 同步失败仅记日志保留旧目录（同 scheduler「失败不阻塞兄弟 job」约定），推荐继续用存量目录。
- [可用性校验打满实例/被目标站限流] → 校验并发与速率可配（默认 2 req/s），校验是后台异步，不阻塞同步主流程。
- [LLM 精排幻觉推荐理由] → prompt 只给候选真实元数据，要求理由引用路由 name/description；理由仅作展示，不影响订阅动作本身的验证（fetch 兜底）。
- [行为数据稀薄导致偏好向量噪声大] → 权重衰减 + 每桶要求最小互动标签数（如 <3 个标签则该桶不产出向量，退全局桶）；问答种子是主要冷启动手段。
- [`topic_tag_board_labels` 覆盖率不足（标签未挂版块）] → 未挂版块的标签进全局桶，全局桶同样参与推荐，不丢信号。

## Migration Plan

1. 新增四表（migrator 路径，pgvector 列沿用 `topic_tag_embeddings` 的写法）。
2. 上线目录同步 job → 首次全量同步 + embedding（一次性 ~3245 条）。
3. 上线偏好重算 job + 画像 API。
4. 上线推荐/问答 API + 前端发现页 + 设置画像视图。
5. 删除旧偏好功能（表数据为纯派生，直接 DROP，无数据迁移）。
6. 回滚：按 5→1 逆序；旧功能删除后回滚=恢复代码，行为数据未动可随时重建。

## 已拍板决策（apply 前澄清，2026-07-24）

- **A 种子累积**：seed 写入做加权合并（`normalize(α×incoming+(1−α)×existing)`，α=0.4 可配），保 `UNIQUE(board_id, source)` 单行不放宽。多次问答落同版块 = 累积非覆盖。
- **B 去重粒度**：粗筛按 route_id 维度状态机去重（accepted/dismissed 的 route 不再推），`feeds.url` 仅对 `usable_directly` 补一道。
- **C hash 粒度**：`recommendation_hash` = route_id+board_id，**不含 source**；qa 与 manual_refresh 共享幂等池与 dismiss 冷却池，跨 source 生效。
- **D feed_discovery 独立 route**（2026-07-25 验收补充）：推荐精排(rerank)用独立 `CapabilityFeedDiscovery`，不与总结(CapabilitySummary)共用，便于分清用量与单独配模型；问答 embedding 仍用通用 CapabilityEmbedding。
- **E RSSHub 配置 UI**（2026-07-25 验收补充）：`rsshub_base_url` 从硬编码常量改为 `ai_settings` 可配置（key=`rsshub_config`，缺省回落自建实例 `DefaultRSSHubBaseURL`），前端新增 `SettingsSectionRsshub` 让用户填实例地址。
- **M1 qa 桶维度**（2026-07-25 review 文档化）：Ask 问答推荐恒落全局桶（board_id=NULL），不强行匹配版块；与 manual_refresh 的版块桶推荐仅在全局桶层面共享幂等池。已知限制非 bug——避免问答与版块桶 hash 维度错位。

## Open Questions

- 推荐精排的 LLM 模型选型走哪条 capability route（复用现有总结用 route 还是新建 `feed_discovery` route）——apply 时看 `aisettings` 现有 route 表决定。
- 发现页在前端的具体落位（feeds 侧边栏入口 vs 独立路由页）——tasks 阶段按现有 shell 结构定。
- **D1 favorite 权重档的数据源待 apply step1 验证**：`ReadingEventType` 含 `'favorite'`，但 `useReadingTracker` 仅发 open/close/scroll。apply step1 须 grep 确认文章收藏动作真写 `reading_behaviors.event_type='favorite'`；若无上报链路，favorite=1.0 档实际恒空 → 退化为深读/open 两档，届时补收藏上报或下掉 favorite 档（tasks 2.6）。
