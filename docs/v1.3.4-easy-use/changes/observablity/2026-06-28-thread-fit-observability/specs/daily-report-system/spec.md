## ADDED Requirements

### Requirement: Thread↔section 标题贴合度计算

日报生成管线 SHALL 为每个 thread 计算其标题 embedding 与所属 section 标题 embedding 的余弦距离，作为"thread 是否忠于本 section 叙事主题"的贴合度信号（区别于 System 1 tag↔板块、System 2 section↔话题，本信号是 System 3 thread↔section）。

贴合度 SHALL 在 Step6（section 装配、section 标题 embedding 就绪）之后、Step7（MergeSimilarSections）之前同步计算，使 thread↔section 配对在合并前固定。批量 embed thread 标题 SHALL 复用 section 标题 embedding 的同一 provider（`airouter.Embed`，`CapabilityEmbedding`）。

信号设计原则：贴合度量"thread 是否忠于它所在 section 的标题（LLM 在 ClusterTags 给该组做的叙事宣言）"，而非"thread 彼此紧凑"。该定义对 section 叙事广度自适应——紧凑型 section（标题聚焦）严判、伞形 section（标题包容）宽判，不预设"组须 embedding 紧凑"（否则会误杀「XR 硬件爆发：从消费级新品到全球军援」这类子事件在 embedding 空间天然分散但叙事合理的伞形话题）。

#### Scenario: 贴合 thread 算出小距离
- **WHEN** 某 section 标题为「OpenAI 发布管控与治理动荡」，其下 thread 标题为「政府强制要求 OpenAI 放缓 GPT-5.6 发布节奏」
- **THEN** 该 thread 的 `fit_distance` SHALL 为较小值（贴合）

#### Scenario: 跑题 thread 算出大距离
- **WHEN** 某 section 标题为「OpenAI 发布管控与治理动荡」，其下 thread 标题为「华腾百巨头争夺机器人控制器核心生态控制权」（LLM 在 ClusterTags 误把机器人 tag 归入 OpenAI 簇）
- **THEN** 该 thread 的 `fit_distance` SHALL 显著大于贴合 thread（离群），足以触发前端降级阈值

#### Scenario: thread 配对在 Step7 合并前固定
- **WHEN** 日报生成进入 Step7 MergeSimilarSections
- **THEN** 所有 thread 的 `fit_distance` SHALL 已计算并落库（对应 Step6 时的 thread↔section 配对）

#### Scenario: 复用 section 标题 embedding 的 provider
- **WHEN** 批量 embed thread 标题
- **THEN** 系统 SHALL 使用与 section 标题 embedding 相同的 `airouter.Embed` 调用（同 `CapabilityEmbedding`），不引入新 embedding 依赖

### Requirement: Thread 贴合度软降级展示

日报 section 的 thread 列表渲染时，`fit_distance` 超过降级阈值（导出常量 `THREAD_FIT_DEMOTE_THRESHOLD`，候选默认 0.20，实现期按现网分布标定）的 thread SHALL **软降级**：灰 token 显示 + 默认折叠收起 + 离群标记，**保留信息不删除**（展开可读全量内容）。降级阈值之下或无 `fit_distance` 的 thread SHALL 按正常 thread 渲染。

当一个 section 内存在 ≥1 条离群 thread 时，section 底部 SHALL 显示提示行"另有 N 条可能跑题的线索"，点击可展开查看。贴合度数值与中文标签（"贴合"/"可能跑题"）SHALL 仅在 thread hover/展开时呈现，不进 thread 标题正文——沿用 observability 系列展示分层哲学（正文极轻、分数进探究区）。

软降级是**有信号才触发**：历史 thread（无 `fit_distance`）SHALL 默认按正常 thread 渲染，不降级、不报错。本 requirement 不自动剔除或重组 thread（保信息，由用户甄别）。

#### Scenario: 离群 thread 软降级
- **GIVEN** 某 thread `fit_distance=0.28`（超 `THREAD_FIT_DEMOTE_THRESHOLD=0.20`）
- **THEN** 该 thread SHALL 灰 token 显示、默认折叠收起、附离群标记，且不删除其标题/摘要内容

#### Scenario: 贴合 thread 正常渲染
- **GIVEN** 某 thread `fit_distance=0.08`（低于阈值）
- **THEN** 该 thread SHALL 按正常 thread 样式渲染，无降级、无折叠

#### Scenario: 阈值边界值
- **WHEN** 某 thread `fit_distance` 恰为 0.20
- **THEN** 该 thread SHALL 归入正常档（阈值之上才降级，0.20 本身不降级）

#### Scenario: 离群 thread 提示行
- **GIVEN** 某 section 有 3 条 thread，其中 2 条 `fit_distance` 超阈值
- **THEN** section 底部 SHALL 显示提示行"另有 2 条可能跑题的线索"
- **AND** 点击该提示行 SHALL 展开显示这 2 条离群 thread 的完整内容

#### Scenario: 贴合度数值仅进探究区
- **WHEN** 渲染离群或贴合 thread
- **THEN** thread 标题正文 SHALL 不出现任何距离数值或百分比
- **AND** 贴合度数值与中文标签 SHALL 仅在 thread hover/展开时呈现

#### Scenario: 历史 thread 不降级
- **GIVEN** 某 thread 缺失 `fit_distance`（历史 thread 或 embedding 失败）
- **THEN** 该 thread SHALL 按正常 thread 渲染，不降级、不报错、不折叠
