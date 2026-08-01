## MODIFIED Requirements

### Requirement: ClusterTags 注入历史叙事框架

`ClusterTags` SHALL 在调用 LLM 前，查询该 board 下所有 active 与 candidate 状态的 PersistentTopic，注入 system prompt，指示 LLM 优先将标签归入已有框架、仅当属于新叙事时开新组。

**注入内容按话题状态区分**：

- **active 话题** SHALL 注入"近期内容"：除 label 外，SHALL 附带该话题最近 **7 天**的 section 标题 + 每条 section 的代表性 thread 标题（按 `fit_distance` 最小优先，每 section 至多 2 条）。注入内容 SHALL 让 LLM 依据话题近期实际演化内容判断归属，而非仅凭框架标题字面沾边。
- **candidate 话题** SHALL 仅注入 label（维持现状，不动）——candidate 不稳定、易 decay，注入内容意义小。

每话题注入的 section 条数 SHALL 有上限（按 `last_seen` 倒序截断，防 prompt token 爆炸）。拉取话题近期内容失败时，SHALL 降级为 label-only 注入（不阻断 ClusterTags）。

LLM 输出 schema SHALL 为每个 group 增加 `matched_topic_id` 字段（可为 null）。

`ClusterTags` SHALL 校验返回的 matched_topic_id 合法性（必须存在于传入集合），非法值降级为 null。

> 注：本增强只改喂给 LLM 的输入信息量（label → label + 近期内容），SHALL NOT 改动 embedding AND-gate、双重确认归属规则、聚类 JSON schema、persistent_topic 生命周期——属于上下文工程，不是动算法。

#### Scenario: 聚类优先复用已有框架

- **GIVEN** board 有 active topic "AI 编程工具平台化竞争"，当天标签含 "开发者 Agent 平台化"
- **WHEN** 执行 ClusterTags
- **THEN** LLM prompt SHALL 包含该 topic 及其近期 section/thread 内容，输出 group 的 matched_topic_id 指向它

#### Scenario: 注入候选与正式两类框架

- **WHEN** 注入 existingTopics
- **THEN** 列表 SHALL 同时包含 active 和 candidate 状态的话题；active SHALL 附带近期内容，candidate SHALL 仅 label，各自标注状态

#### Scenario: active 话题注入近期内容

- **GIVEN** active topic #8 "以黎冲突升级" 最近 7 天有 section "真主党越境打击"、"以军空袭黎南部"
- **WHEN** 执行 ClusterTags
- **THEN** topic #8 的注入项 SHALL 包含这些 section 标题及其代表性 thread 标题，使 LLM 据此区分该话题与字面沾边的其他话题（如"美伊战事"）

#### Scenario: 注入内容拉取失败降级

- **GIVEN** active topic #8 的近期内容查询失败
- **WHEN** 执行 ClusterTags
- **THEN** topic #8 SHALL 降级为 label-only 注入，聚类 SHALL NOT 因此中断

#### Scenario: candidate 话题不注入内容

- **GIVEN** candidate topic #15（status=candidate）
- **WHEN** 执行 ClusterTags
- **THEN** topic #15 的注入项 SHALL 仅含 label，SHALL NOT 附带近期 section/thread 内容
