# Spec: preference-profile

## ADDED Requirements

### Requirement: 行为加权的版块偏好向量

系统 SHALL 以 `reading_behaviors` 为权重来源、`article_topic_tags × topic_tag_board_labels × topic_tag_embeddings` 为向量来源，按 SemanticBoard 聚合生成偏好向量（每版块一行，`board_id=NULL` 表示全局桶）。权重 MUST 按行为类型分级（收藏 > 深读 > 普通打开）并施加时间衰减。未挂载任何版块的标签 MUST 计入全局桶。重算 MUST 为全量重建且幂等，重复执行结果一致；重算 MUST NOT 调用 LLM 或 embedding 接口（纯向量算术）。

#### Scenario: 有行为数据时重算产出版块向量

- **GIVEN** 用户近 30 天对挂载到「AI 前沿」版块的标签所属文章有阅读和收藏行为
- **WHEN** 偏好重算执行
- **THEN** `preference_vectors` 表存在该版块 `source=behavior` 的行，向量为行为加权标签向量质心
- **AND** 该行 `tag_weights` 记录贡献 top 标签及权重

#### Scenario: 重算幂等

- **GIVEN** 行为数据未发生变化
- **WHEN** 连续执行两次偏好重算
- **THEN** 两次产出的向量与权重一致，且不产生重复行

#### Scenario: 信号不足退全局桶

- **GIVEN** 某版块下用户互动过的不同标签数少于最小阈值
- **WHEN** 偏好重算执行
- **THEN** 该版块不产出偏好向量
- **AND** 相关标签权重计入全局桶

### Requirement: 偏好重算调度与手动触发

系统 SHALL 提供 `preference_profile_update` scheduler job 按可配间隔（默认 3600 秒）定期重算偏好向量，并 SHALL 提供手动触发端点 `POST /api/preference-profile/recompute` 走同一重算路径。重算失败 MUST 仅记日志，不阻塞同轮其它 scheduler job。

#### Scenario: 手动触发与定时触发等效

- **WHEN** 用户调用 `POST /api/preference-profile/recompute`
- **THEN** 系统执行与定时 job 相同的重算逻辑
- **AND** 返回本次重算的版块数与标签数摘要

### Requirement: 问答种子偏好

系统 SHALL 在用户通过发现页问答表达兴趣时，将兴趣文本 embedding 与板块向量匹配：相似度达到可配阈值（默认 0.5）写入对应版块，否则写入全局桶，以 `source=seed` 落 `preference_vectors`。**同一版块多次问答 SHALL 加权合并累积种子**（保 `UNIQUE(board_id, source)` 单行）：`new_vec = normalize(α×incoming + (1−α)×existing)`，α 默认 0.4（可配），`tag_weights` 同步合并 top 列表。行为重算 MUST NOT 覆盖 `source=seed` 行。

#### Scenario: 冷启动问答产出种子偏好

- **GIVEN** 用户无任何阅读行为数据
- **WHEN** 用户在发现页提问「我想看 AI 芯片相关的中文资讯」
- **THEN** 系统将该兴趣文本的向量写入匹配版块（或全局桶）的 `source=seed` 偏好行
- **AND** 后续推荐刷新可基于该种子产出推荐

#### Scenario: 行为重算不清除种子

- **GIVEN** 存在 `source=seed` 的偏好行
- **WHEN** 行为偏好重算执行
- **THEN** 种子行保持不变

#### Scenario: 同版块多次问答累积而非覆盖

- **GIVEN** 用户已在「AI 前沿」版块有 `source=seed` 种子（来自「AI 芯片」提问）
- **WHEN** 用户再次提问「量子计算」，仍匹配到「AI 前沿」版块
- **THEN** 该版块 seed 行向量更新为两兴趣的加权合并（α×量子 + (1−α)×芯片）
- **AND** `tag_weights` 反映两批标签的合并 top 列表
- **AND** 不新增 seed 行（仍单行）

### Requirement: 兴趣画像读取

系统 SHALL 提供 `GET /api/preference-profile` 返回各版块偏好画像：版块名、贡献 top 标签及权重、向量来源（behavior/seed）、最后计算时间。无偏好数据时 MUST 返回空列表而非错误。

#### Scenario: 画像按版块组织

- **WHEN** 前端请求 `GET /api/preference-profile`
- **THEN** 响应按版块分组返回 top 标签与权重
- **AND** 全局桶作为独立条目返回

### Requirement: 旧偏好功能废弃

系统 SHALL 移除旧 `user_preferences` 表、`preference_update` scheduler job、`/api/user-preferences/*` 端点及对应前端偏好面板与死代码。`reading_behaviors` 采集与 `/api/reading-behavior/*` 端点 MUST 保留。

#### Scenario: 旧端点移除

- **WHEN** 请求 `GET /api/user-preferences` 或 `POST /api/user-preferences/update`
- **THEN** 后端返回 404（路由不存在）

#### Scenario: 行为采集保留

- **WHEN** 用户打开、滚动、收藏文章
- **THEN** 行为仍批量上报至 `POST /api/reading-behavior/track-batch` 并落 `reading_behaviors` 表
