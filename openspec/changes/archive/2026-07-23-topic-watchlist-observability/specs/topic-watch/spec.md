## ADDED Requirements

### Requirement: 关注标记实体模型

系统 SHALL 维护 `board_topic_watches` 表，持久化用户在某个板块下主动声明的关注。每行 SHALL 包含：`semantic_board_id`（所属板块）、`label`（用户撰写的一句话关切描述）、`status`（`active` / `paused`）、`created_at`、`updated_at`。`status` SHALL 受 CHECK 约束为 `active` / `paused` 二态。

关注标记 SHALL 与 `board_persistent_topics`（持久话题）是**两个互不相关的实体**：独立表、独立 ID 序列、不共享任何外键或归属逻辑。

#### Scenario: 创建关注标记

- **WHEN** 用户在板块下创建关注，提交 label "美伊会不会真打起来"
- **THEN** 系统 SHALL 在 `board_topic_watches` 写入一行，`status=active`，`created_at=updated_at=now`

#### Scenario: 状态约束

- **WHEN** 尝试写入 `status='candidate'`
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

### Requirement: 关注标记 AI 命中判定

日报生成流程末尾（section 持久化与 persistent_topic 归属完成之后），系统 SHALL 对该 board 下所有 `status=active` 的关注标记执行 AI 命中判定：将该期日报的全部 section 与每个关注的 label 一并提交 AI，判定哪些 section 与该关注相关。

判定 SHALL 走 **AI 单信号**（SHALL NOT 使用 embedding 相似度，SHALL NOT 走 persistent_topic 的 embedding+LLM 双重确认 AND-gate），因为关注是用户意图声明而非聚类产物。

判定结果 SHALL 记录到 `topic_watch_hits` 表：`watch_id` / `section_id` / `report_id` / `period_date`，并带 AI 给出的命中理由一句话。

#### Scenario: 命中记录

- **WHEN** 关注「美伊会不会真打起来」对某期日报的 section #123 命中
- **THEN** 系统 SHALL 写入 `topic_watch_hits(watch_id, section_id=123, report_id, period_date, reason)`

#### Scenario: 不走双重确认

- **WHEN** 某 section 与关注的 embedding 相似度很低，但 AI 判定相关
- **THEN** 系统 SHALL 仍记录命中（AI 单信号即生效），SHALL NOT 因 embedding 距离否决

#### Scenario: 批量单次请求

- **WHEN** 同一期日报有 N 个 section、M 个 active 关注
- **THEN** AI 命中判定 SHALL 以批量方式调用（单期日报的 section 与关注在一次或按关注分组的少量请求内完成），SHALL NOT 对每个 section 单独发起请求

### Requirement: 关注标记与持久话题隔离

关注命中 SHALL 是**只读叠加标记**，SHALL NOT 改变任何 section 的归属或持久话题的状态：

- section 的 `persistent_topic_id` SHALL NOT 因关注命中而改变
- 关注命中 SHALL NOT 触发任何 `persistent_topic` 的 `consecutive_hits` / 生命周期更新
- 关注标记 SHALL NOT 拥有 candidate / active / archived 升级或 decay 生命周期

#### Scenario: 命中不改归属

- **GIVEN** section #123 的 `persistent_topic_id=8`
- **WHEN** 关注标记命中 section #123
- **THEN** section #123 的 `persistent_topic_id` SHALL 保持 8 不变

#### Scenario: 命中不推进持久话题生命周期

- **GIVEN** persistent_topic #8 `consecutive_hits=2`
- **WHEN** 关注标记命中属于 topic #8 的 section
- **THEN** topic #8 的 `consecutive_hits` SHALL 保持 2 不变

### Requirement: 关注标记日报顶部独立栏位

日报详情页 SHALL 在正文之上、其余导航之下，提供"关注标记"独立栏位，展示该期日报被各 active 关注命中的 section。

栏位 SHALL 对每个命中的关注分组，组内列出命中 section 的标题与 AI 给出的一句话理由。栏位 SHALL NOT 重复展示 section 正文（正文仍在下方常规分区）。

当天无任何关注命中时，栏位 SHALL 显示空态（"今天无你关注的动态"）或整体隐藏，SHALL NOT 占据显著空白。

#### Scenario: 命中分组展示

- **GIVEN** active 关注「美伊会不会真打起来」命中 2 个 section
- **WHEN** 用户打开该期日报
- **THEN** 顶部栏位 SHALL 在「美伊会不会真打起来」分组下列出这 2 个 section 的标题与命中理由

#### Scenario: 无命中空态

- **WHEN** 某期日报无任何 active 关注命中
- **THEN** 顶部栏位 SHALL 显示空态或隐藏，SHALL NOT 渲染空分组

### Requirement: 关注标记管理 API

系统 SHALL 提供关注标记的 CRUD API（针对单用户、单 board 场景）：

- `POST /api/semantic-boards/:boardId/topic-watches`：创建关注（body: label）
- `GET /api/semantic-boards/:boardId/topic-watches`：列出该 board 全部关注（含 status）
- `PATCH /api/topic-watches/:id`：更新 label 或切换 status（active/paused）
- `DELETE /api/topic-watches/:id`：删除关注（连同其命中记录）

`paused` 状态的关注 SHALL NOT 参与当期日报的 AI 命中判定。

#### Scenario: 暂停的关注不参与判定

- **GIVEN** 关注 #5 `status=paused`
- **WHEN** 日报生成执行命中判定
- **THEN** 系统 SHALL 跳过关注 #5，SHALL NOT 为其写命中记录

#### Scenario: 删除关注级联清理命中

- **WHEN** 用户删除关注 #5
- **THEN** 系统 SHALL 删除关注 #5 及其全部 `topic_watch_hits` 记录
