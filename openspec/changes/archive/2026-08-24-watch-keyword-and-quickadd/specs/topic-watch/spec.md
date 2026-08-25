## MODIFIED Requirements

### Requirement: 关注标记实体模型

系统 SHALL 维护 `board_topic_watches` 表，持久化用户在某个版块下主动声明的关注。每行 SHALL 包含：`semantic_board_id`（所属板块）、`label`（关注文本——label 类为一句话关切描述，keyword 类为关键字表达式）、`type`（`label` / `keyword`）、`status`（`active` / `paused`）、`created_at`、`updated_at`。`status` SHALL 受 CHECK 约束为 `active` / `paused` 二态；`type` SHALL 受 CHECK 约束为 `label` / `keyword` 二态，默认 `label`。

关注标记 SHALL 与 `board_persistent_topics`（持久话题）是**两个互不相关的实体**：独立表、独立 ID 序列、不共享任何外键或归属逻辑。

历史已存在的 watch 行（迁移前）SHALL 默认 `type=label`，行为完全不变（仍走 AI 单信号）。

#### Scenario: 创建关注标记

- **WHEN** 用户在版块下创建关注，type=label，提交 label "美伊会不会真打起来"
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

### Requirement: 关注命中在日报时间线与详情中的呈现

TagsPage 的日报时间线 SHALL 保持原有日期顺序；每条日报记录下 SHALL 展示该期 **active** watch 的紧凑命中预告，最多两个 tag，余项显示 `+N`。tag SHALL 显示 watch label：keyword 类带 `#`，label 类带 `✦`；点击 tag SHALL 打开该期日报并定位到命中的 section。暂停或已删除 watch 的历史 hit SHALL NOT 出现在预告中。

日报详情 SHALL NOT 渲染独立、居中、窄宽的关注栏位或重复 section 正文。详情正文中 SHALL 在 **`.drm-content` 正文列内**、位于「关心的话题」之前，以全宽同级阅读分区依次呈现「追踪关键字」「追踪话题」：各分区只显示命中 section 的单行索引（图标 + watch label + section 标题），点击定位到原 section；keyword 与 label 依靠 `#` / `✦` 区分，不使用与日报主题脱节的大面积绿色。reason SHALL NOT 在索引中常驻展示。

无命中时对应详情分区 SHALL 整体隐藏；管理入口仍唯一位于 TagsPage tab 栏的「我在追踪 (N)」，不改变其职责。

#### Scenario: 时间线命中预告

- **GIVEN** 某期日报有 label 类关注「美伊博弈」命中 2 个 section，keyword 类关注「ASML」命中 1 个 section
- **WHEN** 用户查看 TagsPage 的日报时间线
- **THEN** 该期记录日期下 SHALL 显示最多两个紧凑 tag（`✦ 美伊博弈`、`# ASML`）；时间线日期顺序不变

#### Scenario: 详情优先索引与定位

- **GIVEN** 某期日报同时有 keyword 与 label 类 active watch 命中
- **WHEN** 用户打开该期日报
- **THEN** 「追踪关键字」「追踪话题」SHALL 位于「关心的话题」之前，分别显示单行索引；点击索引 SHALL 定位原 section，索引中不重复 section 正文或常驻 reason

#### Scenario: keyword 命中无 AI 理由

- **WHEN** keyword 类关注命中某 section
- **THEN** 该命中的 reason 字段 SHALL 为机械文本「含关键字『XX』」，SHALL NOT 调用 AI 生成自然语言理由

#### Scenario: 无命中隐藏

- **WHEN** 某期日报无任何 active 关注命中
- **THEN** 时间线记录下不显示命中 tag，详情的两个追踪分区均隐藏

### Requirement: 关注标记管理 API

系统 SHALL 提供关注标记的 CRUD API（针对单用户、单 board 场景）：

- `POST /api/semantic-boards/:id/topic-watches`：创建关注（body: `label` + 可选 `type`，type 缺省为 `label`）
- `GET /api/semantic-boards/:id/topic-watches`：列出该 board 全部关注（含 status、type）
- `PATCH /api/topic-watches/:id`：更新 label 或切换 status（active/paused）
- `DELETE /api/topic-watches/:id`：删除关注（连同其命中记录）

`paused` 状态的关注 SHALL NOT 参与当期日报的命中判定（label 与 keyword 两类皆然）。

label 类与 keyword 类在 CRUD 上 SHALL 一视同仁（同一组端点，type 仅影响命中判定方式）。

type=keyword 时，label SHALL 为可解析出至少一个有效词组的关键字表达式——空串、纯空白、纯分隔符（如 `"ASML|"`、`"  |  "`）SHALL 被拒绝（400）。

#### Scenario: 创建 keyword 关注

- **WHEN** POST body 含 label="ASML|镓锗 出口"、type=keyword
- **THEN** 系统 SHALL 创建 type=keyword 的关注

#### Scenario: type 缺省为 label

- **WHEN** POST body 仅含 label，无 type 字段
- **THEN** 系统 SHALL 创建 type=label 的关注（向后兼容）

#### Scenario: 无效关键字表达式被拒绝

- **WHEN** POST body 含 type=keyword、label="ASML|"（解析后不含任何有效词组；空串 / 纯空白 / 纯分隔符同）
- **THEN** 系统 SHALL 拒绝创建并返回 400，提示关键字表达式无效，SHALL NOT 写入该行

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

### Requirement: 关注管理面板（版块级中枢）

系统 SHALL 在版块工作台内容区的 tab 栏（TagsPage tags-content-tabs）右端提供关注管理入口「我在追踪 (N)」，版块选中后在各内容 tab（板块内容/话题总览/日报/文章/数据增强）下均常驻可见。管理面板（弹层或抽屉）SHALL 提供：

- **新建关注**：类型双选（label/keyword）对话框——label 态为一句话关切输入；keyword 态含语法提示（空格 AND / `|` OR）、实时解析预览（chips 展示解析结果，无效表达式红字提示并禁用提交）、回扫预期说明。提交 SHALL 走统一 createWatch API。
- **keyword 即时回扫反馈**：keyword 关注创建后，面板 SHALL 展示回扫结果反馈（已回扫近 14 天 + 命中数可点查看）。
- **关注列表管理**：列出该版块全部关注（label/keyword 类型标识），支持暂停/恢复与删除（删除级联清理命中）。

创建与管理的唯一入口 SHALL 为本面板；日报详情页的关注栏位 SHALL NOT 提供新建入口（栏头跳转见「关注标记日报顶部独立栏位」）。

#### Scenario: 入口常驻版块内容区

- **WHEN** 用户选中某版块，处于任一内容 tab（板块内容/话题总览/日报/文章/数据增强）
- **THEN** 版块日报面板头部 SHALL 显示「我在追踪 (N)」入口，N 为该版块关注总数（active 与 paused 皆计入）

#### Scenario: 从管理面板新建关键字关注

- **WHEN** 用户在管理面板切 type=keyword，输入 "ASML|镓锗 出口" 提交
- **THEN** 系统 SHALL 创建 type=keyword 关注，并展示即时回扫反馈（回扫近 14 天，命中 N 条可点查看）

#### Scenario: 管理面板内暂停或删除关注

- **GIVEN** 管理面板列出该版块全部关注（含类型标识）
- **WHEN** 用户暂停关注 #5 或删除关注 #5
- **THEN** 系统 SHALL 将 #5 置为 paused（下期起不参与判定），或删除 #5 及其全部命中记录
