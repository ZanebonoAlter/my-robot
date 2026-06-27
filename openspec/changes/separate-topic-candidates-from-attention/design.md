## Context

PersistentTopic 同时承担两类职责：后台用它给每日 section 建立跨天身份锚点，前台又按它的当前 `status` 决定日报阅读分区。当前 `buildQualityZones` 把 active 放入“关心的话题”，把所有“已归属但非 active”的 section 放入“突发的新话题”。因此，算法只要为无法确认归属的 section 创建 candidate，用户就会看到一个新的高注意力分区。

这套映射还有两个结构性问题：

1. candidate 失去连续命中后只清零 `consecutive_hits`，不会退出可归属/可注入集合，候选池会长期累积。
2. 日报详情读取 PersistentTopic 的当前状态；话题后续从 candidate 变 active 或 active 变 archived 时，历史日报可能被重新分区。

相关改动横跨 Go 生命周期与聚类、PostgreSQL 数据模型、日报 API、Nuxt 阅读层，并与规划中的 `topic-watchlist-observability` 存在产品语义交叉。实现必须保持二者隔离：PersistentTopic 表示叙事身份，watch 表示用户主动关注。

## Goals / Non-Goals

**Goals:**

- candidate 继续服务跨天归属，但不再自动成为日报的用户注意力入口。
- active 且在报告生成时已确认的话题才进入“关心的话题”。
- 历史日报的阅读分区不随 PersistentTopic 后续状态变化。
- 陈旧 candidate 自动退出可归属锚点池；聚类 prompt 中的 candidate 有明确时间窗口和数量上限。
- 所有行为可配置、可观测、可用纯函数和集成测试验证。

**Non-Goals:**

- 不取消 PersistentTopic 或 `auto_new` candidate 创建。
- 不在本 change 中实现 topic watch CRUD、命中判定或顶部关注栏。
- 不改变 active 的人工确认门槛，不把 candidate 自动升级为 active。
- 不重做日报视觉主题、侦探墙或 section 关系算法。
- 不在第一阶段增加“新线索”徽章；若以后需要，应另以质量/显著性规则设计，不能直接映射 candidate。

## Decisions

### 1. 将叙事身份状态与阅读注意力分区解耦

日报 section 新增可空快照字段 `topic_status_at_report`，在 section 完成 PersistentTopic 归属的同一事务内写入当时的 `candidate|active` 状态；未归属写 NULL。API 原样返回该字段。

前端只按快照划分两个主区：

- `topic_status_at_report=active` → “关心的话题”
- candidate、archived、NULL 或旧数据缺失 → “其他动态”

当前 `persistent_topic.status` 仍用于话题管理，不再用于历史日报分区。旧数据没有快照时统一降级到“其他动态”，宁可少制造注意力，也不根据当前状态猜测历史语义。

**备选：继续读取当前 topic.status。** 否决，因为它会让历史报告随未来状态变化。

**备选：从 `topic_match_confidence=auto_new` 推断。** 否决，因为它只能说明首次归属方式，无法证明报告生成时用户是否已确认该话题。

### 2. 日报移除 candidate 专属主分区

`QualityZone` 从 `active|candidate|unassigned` 收敛为 `active|briefs`。candidate section 可以继续按 `persistent_topic_id` 在“其他动态”内部聚合，保留叙事阅读连续性，但不显示“突发”“Developing”或 candidate 状态徽章，也不自动展开生命线。

分区内部仍按 `best_tier ASC, avg_score DESC` 排序，candidate 身份不提供额外排序加权。

**备选：保留折叠的“新线索”区。** 暂不采用。即使折叠，它仍建立了一个由算法不确定性驱动的注意力队列；真正值得提醒的线索应由后续 watch/显著性规则产生。

### 3. candidate 使用独立失活窗口

新增配置 `persistent_topic_candidate_decay_window`，默认 7 个自然日。生命周期执行时，candidate 若 `today - last_seen_date > window`，转为 archived；窗口内未命中仍只清零 `consecutive_hits`。active 继续使用现有 30 天 `persistent_topic_decay_window`。

7 天比 active 窗口短，允许周内断续事件重新归属，又避免一次性新闻永久留在候选池。该默认值在实现前通过真实数据可丢弃分析验证；若数据否定该值，应先更新本 change 的设计和规格再实现。

**备选：首次 miss 立即归档。** 否决，因为周末、采集缺口和非每日演化会造成同一叙事反复新建。

### 4. 聚类与归属共享同一个“可锚定话题集合”

新增统一选择逻辑，输入 board、报告日期和配置，输出：

1. 所有 active；
2. `last_seen_date` 仍在 candidate 失活窗口内的 candidate；
3. candidate 按 `last_seen_date DESC, hit_count DESC, id ASC` 排序，最多取 `persistent_topic_candidate_prompt_limit` 条，默认 20。

ClusterTags 注入和后续双重确认归属必须使用同样的集合，避免 LLM 看到了某话题但 assignment 不接受，或 assignment 期待一个未注入的 id。生命周期更新仍查询全部非 archived 话题，以便把窗口外 candidate 转 archived。

**备选：只限制 prompt，assignment 仍加载全部 candidate。** 否决，因为双重确认两侧集合不一致会制造新的 `auto_new`。

### 5. 快照在归属事务内写入

`persistent_topic_id`、距离、置信度和 `topic_status_at_report` 一次更新，避免部分写入。新增列可空，无需回填历史数据；这使迁移可在线执行并保持旧 API 数据兼容。

### 6. 观察指标用于验证“减少噪声但不破坏复用”

沿用现有 persistent-topic 日志并增加：输入 active 数、窗口内 candidate 总数、实际注入 candidate 数、被窗口过滤数、被上限截断数、当日 auto_new 数。不得记录完整 prompt、文章正文或 embedding。

## Risks / Trade-offs

- **[候选淘汰过快导致同一叙事重建]** → 真实数据原型校准 7 天窗口；保留配置项并测试边界日。
- **[candidate 上限截掉应复用的话题]** → 最近命中优先、累计命中次优；记录截断数，并验证 auto_new 比例没有异常上升。
- **[旧日报全部落入“其他动态”显得保守]** → 接受这一降级；它比根据当前状态伪造历史注意力更可信。
- **[active 数量本身过多仍占 prompt]** → 本 change 不限制用户已确认的 active；后续可依据观测数据另行治理。
- **[与 watchlist UI 重复]** → 本 change 不新增注意力组件，并在 reference 文档明确“身份锚点 vs 主动关注”。

## Migration Plan

1. 用真实数据库只读快照做可丢弃分析，统计 candidate 年龄、复现间隔和不同窗口/上限下的保留率，确认或修订默认值。
2. 增加可空 `topic_status_at_report` 列及两个 ai_settings 配置，迁移必须幂等。
3. 先发布后端：写入快照、收紧 candidate 生命周期和锚点选择；API 对旧数据返回 NULL。
4. 再发布前端：读取快照，移除 candidate 分区，旧数据保守归入“其他动态”。
5. 观察 auto_new、候选截断和归档数量；出现明显复用退化时，可把窗口/上限调大。

**回滚：** 前端可回滚到旧分区而不影响新增列；后端逻辑可回滚，新增可空列和配置保留不删。已自动归档的 candidate 可通过现有话题管理接口恢复，禁止通过破坏性迁移批量删除。

## Calibration Evidence

2026-06-27 使用本地 PostgreSQL 真实数据做只读校准：以每个 board 的最新日报日期作为 `as_of`，`hit_count > 1` 作为“曾复现、具有再次复用价值”的保守代理，`potential_auto_new` 表示因时间窗口或数量上限被排除的曾复现 candidate 数。原型只执行聚合 SQL，未写数据库、未进入生产代码。

| Board | Candidate 总数 | W7/L12 potential_auto_new | W7/L20 potential_auto_new |
|---:|---:|---:|---:|
| 1974 | 17 | 1 | 0 |
| 1980 | 29 | 7 | 3 |
| 2128 | 9 | 0 | 0 |
| 2165 | 7 | 0 | 0 |
| 2197 | 22 | 3 | 0 |
| 2242 | 8 | 0 | 0 |
| 2272 | 23 | 0 | 0 |
| 3030 | 16 | 0 | 0 |
| 15219 | 15 | 2 | 0 |
| 15220 | 50 | 5 | 0 |
| **合计** | **196（窗口内）** | **18** | **3** |

窗口对比显示 W3 会额外排除 11 个曾复现 candidate；W7 与 W14 在当前样本结果相同，因此保留更克制的 7 天默认值。上限对比显示 L12 对复现候选伤害明显，而 L20 仍能从窗口内 196 条 candidate 中裁掉 44 条、仅触及 3 条曾复现候选，因此默认上限由设计初稿的 12 修订为 20。

该分析无法直接重放历史 LLM 决策，`potential_auto_new` 是风险代理而非精确预测；实现后的结构化日志继续观察真实 auto_new 变化。

## Open Questions

- 默认 7 天窗口和 20 条 candidate 上限已由上述真实数据原型确认；上线后仍需依据真实 auto_new 与截断日志复核。
- `topic-watchlist-observability` 落地后，“关心的话题”是否最终改名为“持续话题”，把“关心”一词完全让给 watchlist，需要在该 change 实施前统一文案；不影响本 change 的数据边界。
