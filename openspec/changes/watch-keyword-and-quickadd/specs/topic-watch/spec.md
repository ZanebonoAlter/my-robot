## MODIFIED Requirements

### Requirement: 关注标记实体模型

系统 SHALL 维护 `board_topic_watches` 表，持久化用户在某个板块下主动声明的关注。每行 SHALL 包含：`semantic_board_id`（所属板块）、`label`（关注文本——label 类为一句话关切描述，keyword 类为关键字表达式）、`type`（`label` / `keyword`）、`status`（`active` / `paused`）、`created_at`、`updated_at`。`status` SHALL 受 CHECK 约束为 `active` / `paused` 二态；`type` SHALL 受 CHECK 约束为 `label` / `keyword` 二态，默认 `label`。

关注标记 SHALL 与 `board_persistent_topics`（持久话题）是**两个互不相关的实体**：独立表、独立 ID 序列、不共享任何外键或归属逻辑。

历史已存在的 watch 行（迁移前）SHALL 默认 `type=label`，行为完全不变（仍走 AI 单信号）。

#### Scenario: 创建 label 类关注

- **WHEN** 用户在板块下创建关注，type=label，提交 label "美伊会不会真打起来"
- **THEN** 系统 SHALL 在 `board_topic_watches` 写入一行，type=label，status=active，created_at=updated_at=now

#### Scenario: 创建 keyword 类关注

- **WHEN** 用户创建关注，type=keyword，提交 label "ASML|镓锗 出口"
- **THEN** 系统 SHALL 写入一行，type=keyword，status=active

#### Scenario: 状态约束

- **WHEN** 尝试写入 `status='candidate'`
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

#### Scenario: 类型约束

- **WHEN** 尝试写入 `type='regex'`
- **THEN** 系统 SHALL 因 CHECK 约束拒绝

#### Scenario: 历史 watch 默认类型

- **WHEN** 迁移执行后，历史已存在的 watch 行
- **THEN** 其 type SHALL 为 label（迁移默认值），不报错，仍走 AI 判定

### Requirement: 关注标记日报顶部独立栏位

日报详情页 SHALL 在正文之上、其余导航之下，提供"关注标记"独立栏位，展示该期日报被各 active 关注（label 类与 keyword 类统一）命中的 section。

栏位 SHALL 对每个命中的关注分组，组内列出命中 section 的标题与理由。**label 类命中的理由 SHALL 为 AI 给出的一句话自然语言理由**（斜体展示）；**keyword 类命中的理由 SHALL 为机械文本「含关键字『XX』」**（标注实际命中的关键字，不调 AI），视觉上 keyword 分组 SHALL 有微区分（如标签图标）以与 label 类的 AI 理由区分。栏位 SHALL NOT 重复展示 section 正文。

当天无任何关注命中时，栏位 SHALL 显示空态或整体隐藏。

#### Scenario: 两类命中统一分组展示

- **GIVEN** label 类关注「美伊博弈」命中 2 个 section，keyword 类关注「ASML」命中 1 个 section
- **WHEN** 用户打开该期日报
- **THEN** 顶部栏位 SHALL 在「美伊博弈」分组下列出 2 个 section（带 AI 理由），在「ASML」分组下列出 1 个 section（带「含关键字『ASML』」机械理由）

#### Scenario: keyword 命中无 AI 理由

- **WHEN** keyword 类关注命中某 section
- **THEN** 该命中的 reason 字段 SHALL 为机械文本「含关键字『XX』」，SHALL NOT 调用 AI 生成自然语言理由

#### Scenario: 无命中空态

- **WHEN** 某期日报无任何 active 关注命中
- **THEN** 顶部栏位 SHALL 显示空态或隐藏

### Requirement: 关注标记管理 API

系统 SHALL 提供关注标记的 CRUD API（针对单用户、单 board 场景）：

- `POST /api/semantic-boards/:boardId/topic-watches`：创建关注（body: `label` + 可选 `type`，type 缺省为 `label`）
- `GET /api/semantic-atches/:boardId/topic-watches`：列出该 board 全部关注（含 status、type）
- `PATCH /api/topic-watches/:id`：更新 label 或切换 status（active/paused）
- `DELETE /api/topic-watches/:id`：删除关注（连同其命中记录）

`paused` 状态的关注 SHALL NOT 参与当期日报的命中判定（label 与 keyword 两类皆然）。

label 类与 keyword 类在 CRUD 上 SHALL 一视同仁（同一组端点，type 仅影响命中判定方式）。

#### Scenario: 创建 keyword 关注

- **WHEN** POST body 含 label="ASML|镓锗 出口"、type=keyword
- **THEN** 系统 SHALL 创建 type=keyword 的关注

#### Scenario: type 缺省为 label

- **WHEN** POST body 仅含 label，无 type 字段
- **THEN** 系统 SHALL 创建 type=label 的关注（向后兼容）

#### Scenario: 暂停的关注不参与判定

- **GIVEN** 关注 #5 status=paused（无论 label/keyword）
- **WHEN** 日报生成执行命中判定
- **THEN** 系统 SHALL 跳过关注 #5

#### Scenario: 删除关注级联清理命中

- **WHEN** 用户删除关注 #5
- **THEN** 系统 SHALL 删除关注 #5 及其全部 `topic_watch_hits` 记录

## REMOVED Requirements

### Requirement: 关注标记 AI 命中判定

（被 ADDED「关注标记命中判定（label 走 AI / keyword 走文本）」取代——原 requirement 假定全部 watch 走 AI 单信号，keyword 类引入后该假设不再成立。）

## ADDED Requirements

### Requirement: 关注标记命中判定（label 走 AI / keyword 走文本）

日报生成流程末尾（section 持久化与 persistent_topic 归属完成之后），系统 SHALL 对该 board 下所有 `status=active` 的关注标记执行命中判定，**按 type 分叉**：

- **label 类** SHALL 走 **AI 单信号**：将该期日报全部 section 与每个 label 类关注一并提交 AI，判定相关 section（SHALL NOT 使用 embedding，SHALL NOT 走 persistent_topic 双重确认 AND-gate）。
- **keyword 类** SHALL 走 **纯文本匹配**：对该期每个 section，取其全部 thread 的 `title` + `summary` 拼接文本，按关键字表达式判定（SHALL NOT 调用 AI、SHALL NOT 调用 embedding）。

keyword 关键字表达式 SHALL 支持：空格分隔 = AND（全含才命中）、`|` 分隔 = OR（含任一即命中），可混用（先拆 `|` 再拆空格）；匹配 SHALL 大小写不敏感。

两类判定结果 SHALL 合并写入 `topic_watch_hits`：`watch_id` / `section_id` / `report_id` / `period_date`。label 类命中的 `reason` 为 AI 一句话理由；keyword 类命中的 `reason` 为机械文本「含关键字『XX』」（XX 为实际命中的关键字）。复合唯一索引 (watch_id, section_id, report_id) SHALL 保证两类命中幂等去重。

#### Scenario: label 类走 AI 单信号

- **WHEN** label 类关注「美伊会不会真打起来」对某期 section #123 命中
- **THEN** 系统 SHALL 经 AI 判定写入 hit，reason 为 AI 自然语言理由

#### Scenario: keyword 类走文本匹配

- **GIVEN** keyword 类关注 "ASML|镓锗 出口"，某 section threads 文本含 "ASML" 与 "出口"
- **WHEN** 日报生成执行命中判定
- **THEN** 系统 SHALL 经纯文本匹配写入 hit（不调 AI），reason 为「含关键字『ASML』」

#### Scenario: keyword 多词 AND 逻辑

- **GIVEN** keyword 关注 "出口 限制"（空格 AND）
- **WHEN** 某 section threads 含 "出口" 但不含 "限制"
- **THEN** 系统 SHALL NOT 记录命中（AND 要求全含）

#### Scenario: keyword 多词 OR 逻辑

- **GIVEN** keyword 关注 "ASML|镓锗"
- **WHEN** 某 section threads 含 "镓锗"
- **THEN** 系统 SHALL 记录命中，reason「含关键字『镓锗』」

#### Scenario: keyword 大小写不敏感

- **GIVEN** keyword 关注 "ASML"
- **WHEN** 某 section threads 含 "asml"
- **THEN** 系统 SHALL 记录命中

#### Scenario: 两类命中合并写表

- **WHEN** 同一期日报 label 类与 keyword 类各有命中
- **THEN** 两类命中 SHALL 合并写入同一张 topic_watch_hits，顶部栏统一展示

### Requirement: keyword 类即时匹配

用户创建 `type=keyword` 的关注后，系统 SHALL **立即**对该 board 最近 14 天的 section 执行纯文本匹配（同上条 Requirement 的 keyword 匹配规则），命中 SHALL 写入 `topic_watch_hits`（`report_id` 取 section 所属 report、`period_date` 取 section 日期），SHALL NOT 等待下一期日报生成。

即时匹配 SHALL 用 `OnConflict DoNothing`（复用复合唯一索引）保证与后续日报匹配幂等去重。

即时匹配 SHALL NOT 阻断关注创建：若即时匹配失败，关注 SHALL 仍创建成功（type=keyword），失败仅记日志（非致命，与 EvaluateWatchHits 的吞错策略一致）。

label 类关注 SHALL NOT 支持即时匹配（label 类依赖 AI 且匹配语义需当期 section 集合，无法即时）。

#### Scenario: 建关注后立即命中历史 section

- **GIVEN** board 有近 14 天 section，其中 section #200（06-20）threads 含 "ASML"
- **WHEN** 用户创建 keyword 关注 "ASML"
- **THEN** 系统 SHALL 立即写入 hit(watch_id=新, section_id=200, report_id=#200 所属, period_date=06-20)，SHALL NOT 等下一期日报

#### Scenario: 即时与日报匹配幂等去重

- **GIVEN** keyword 关注已即时匹配 section #200 写入 hit
- **WHEN** 后续日报生成再次匹配到 section #200
- **THEN** 系统 SHALL NOT 写入重复行（OnConflict DoNothing）

#### Scenario: 即时匹配失败不阻断建关注

- **WHEN** keyword 关注创建后即时匹配执行失败
- **THEN** 关注 SHALL 仍创建成功（type=keyword），失败仅记日志

#### Scenario: label 类不即时匹配

- **WHEN** 用户创建 label 类关注
- **THEN** 系统 SHALL NOT 执行即时匹配，SHALL 仅在下一期日报生成时判定

### Requirement: 内容流快捷关注入口

系统 SHALL 在内容流提供「＋关注」快捷入口（与日报顶部栏的「新建关注」对话框并存）：

- section 详情、话题详情（话题生命线 / 话题泳道节点详情）旁 SHALL 提供「＋关注」入口。
- 点击「＋关注」SHALL 打开新建关注对话框，`label` SHALL 预填（section 入口预填该 section 的 `cluster_label`，话题入口预填 `topic.label`），`type` 默认 `label`。
- 用户 SHALL 可在对话框切换 `type`（label↔keyword）、修改 label/关键字文本后提交。

快捷入口 SHALL 复用统一的 `createWatch` API（与顶部栏入口同一端点），SHALL NOT 引入独立创建路径。

入口 SHALL NOT 绑定话题总览工作台（工作台 lanes 旁入口属 manual-topic-lane change 范围）。

#### Scenario: 从话题详情一键关注

- **GIVEN** 话题「美伊博弈」详情页
- **WHEN** 用户点击「＋关注」
- **THEN** 新建关注对话框 SHALL 打开，label 预填"美伊博弈"，type 默认 label

#### Scenario: 从 section 详情一键关注并切 keyword

- **GIVEN** section 详情（cluster_label="半导体出口管制升级"）
- **WHEN** 用户点「＋关注」，将 type 切为 keyword，label 改为 "ASML|镓锗"，提交
- **THEN** 系统 SHALL 创建 type=keyword、label="ASML|镓锗" 的关注，并触发即时匹配

#### Scenario: 快捷入口与顶部栏同端点

- **WHEN** 用户从内容流快捷入口创建关注
- **THEN** 系统 SHALL 调用与顶部栏「新建关注」相同的 createWatch API
