## MODIFIED Requirements

### Requirement: 关注标记实体模型

系统 SHALL 维护 `board_topic_watches` 表，持久化用户在某个版块下主动声明的关注。每行 SHALL 包含：`semantic_board_id`（所属版块）、`label`（话题名或关切描述）、`type`（匹配轨：`label` / `keyword` / `keyword_topic` / `sentence_topic`，默认 `label`）、`query`（可空，sentence_topic 的检索句，为空时回退使用 label）、`embedding_cache`（可空，sentence_topic 检索句的向量缓存）、`status`（`active` / `paused`）、`created_at`、`updated_at`。`status` SHALL 受 CHECK 约束为 `active` / `paused` 二态；`type` SHALL 受 CHECK 约束为上述四值。

label / keyword（提示轨）关注 SHALL 与 `board_persistent_topics`（持久话题）保持互不相关：不共享外键或归属逻辑。keyword_topic 关注 SHALL 不持有任何持久话题关联。sentence_topic 关注 SHALL 持久化其专属持久话题的关联（关联建立于首次物化时，见 watch-materialized-topic 能力）；关注实体自身 SHALL NOT 拥有 candidate / active / archived 生命周期。

#### Scenario: 创建关注标记

- **WHEN** 用户在版块下创建关注，提交 label "美伊会不会真打起来"
- **THEN** 系统 SHALL 在 `board_topic_watches` 写入一行，`type=label`，`status=active`，`created_at=updated_at=now`

#### Scenario: 创建一句话物化关注

- **WHEN** 用户创建 sentence_topic 关注，提交话题名「AI 编程工具进展」与检索句「AI coding assistant 的进展和竞争格局」
- **THEN** 系统 SHALL 写入一行 `type=sentence_topic`，`label=话题名`，`query=检索句`，并生成检索句向量写入 `embedding_cache`

#### Scenario: 类型约束

- **WHEN** 尝试写入 `type='sentence'`
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

#### Scenario: 状态约束

- **WHEN** 尝试写入 `status='candidate'`
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

### Requirement: 关注标记与持久话题隔离

命中提示记录 SHALL 保持只读叠加语义，对全部 type 成立：命中 SHALL NOT 改变任何 section 的 `persistent_topic_id`，SHALL NOT 触发任何持久话题的 `consecutive_hits` / 生命周期更新。keyword_topic / sentence_topic 物化轨 SHALL NOT 产生命中提示记录。

sentence_topic 关注专属持久话题的生命周期推进 SHALL 仅由物化 section 的归属驱动（与普通 section 归属同一机制），SHALL 遵循 persistent-topic 能力的既有规则，SHALL NOT 由命中判定驱动。

#### Scenario: 命中不改归属

- **GIVEN** section #123 的 `persistent_topic_id=8`
- **WHEN** label 提示轨关注命中 section #123
- **THEN** section #123 的 `persistent_topic_id` SHALL 保持 8 不变

#### Scenario: 命中不推进持久话题生命周期

- **GIVEN** persistent_topic #8 `consecutive_hits=2`
- **WHEN** label 提示轨关注命中属于 topic #8 的 section
- **THEN** topic #8 的 `consecutive_hits` SHALL 保持 2 不变

#### Scenario: 物化轨生命周期由归属驱动

- **GIVEN** sentence_topic 关注拥有专属话题 T
- **WHEN** 当期物化 section 归属 T
- **THEN** T 的生命周期 SHALL 按持久话题既有规则推进，与普通 section 归属话题的机制一致

### Requirement: 关注标记管理 API

系统 SHALL 提供关注标记的 CRUD API（针对单用户、单 board 场景）：

- `POST /api/semantic-boards/:id/topic-watches`：创建关注（body: label, type, query）。keyword / keyword_topic 类型 SHALL 校验关键字表达式合法（复用既有 DNF 校验）；sentence_topic SHALL 校验检索句非空，创建时 SHALL 尝试生成检索句向量缓存，生成失败时关注 SHALL 照常创建（缓存留空，首次日报生成时惰性补算）。
- `GET /api/semantic-boards/:id/topic-watches`：列出该 board 全部关注（含 type / query / status）。
- `PATCH /api/topic-watches/:id`：更新 label / query 或切换 status（active/paused）。更新 label / query SHALL 使 `embedding_cache` 失效。
- `DELETE /api/topic-watches/:id`：删除关注。keyword_topic：仅停止后续物化；sentence_topic：SHALL 要求显式确认并归档专属话题（联动细节见 watch-materialized-topic 能力）；删除 SHALL 级联清理命中记录（提示轨）。

`paused` 状态的关注（任何 type）SHALL NOT 参与当期日报的命中判定与物化。

#### Scenario: 暂停的关注不参与判定

- **GIVEN** 关注 #5 `status=paused`，`type=sentence_topic`
- **WHEN** 日报生成执行命中判定与物化
- **THEN** 系统 SHALL 跳过关注 #5，SHALL NOT 为其写命中记录或产出物化 section

#### Scenario: 删除关注级联清理命中

- **WHEN** 用户删除 label 提示轨关注 #5
- **THEN** 系统 SHALL 删除关注 #5 及其全部 `topic_watch_hits` 记录

#### Scenario: 非法表达式拒绝创建

- **WHEN** 用户创建 keyword_topic 关注，表达式以 `|` 结尾
- **THEN** 系统 SHALL 拒绝创建并返回校验错误

#### Scenario: 更新检索句失效缓存

- **WHEN** 用户 PATCH sentence_topic 关注的 query 字段
- **THEN** 该关注的 `embedding_cache` SHALL 被置空，等待下次日报生成惰性补算
