## ADDED Requirements

### Requirement: 辅助标签创建归一化查重键补强

系统 SHALL 在辅助标签创建入口（`ResolveAuxiliaryLabel`，当前唯一调用方为 LLM 提取入库的 `AttachAuxiliaryLabels`；系统无 keyword 入库、无手动创建 aux 路径）的现有 L1 匹配（slug + alias）之外，增加按归一化键（label 去除全部空白字符并转小写）对 active 辅助标签做精确查重；命中时 SHALL 复用既有标签（累计 ref_count），SHALL NOT 新建记录。该归一化键 SHALL 与一次性迁移所用归一化函数为同一实现，避免迁移后新标签再次产生变体。

#### Scenario: 文本变体命中既有标签

- **WHEN** 系统拟新建辅助标签 "SK 海力士"，而已有 active 辅助标签 "SK海力士"
- **THEN** 系统 SHALL 复用 "SK海力士"，不新建记录

#### Scenario: 无命中时正常新建

- **WHEN** 系统拟新建辅助标签 "量子计算"，归一化查重无命中
- **THEN** 系统 SHALL 正常创建新辅助标签

### Requirement: 辅助标签 embedding 近重复阈值可配

系统现有的 L2 近重复检查（候选 merge_embedding 与全量 active 辅助标签 merge_embedding 的 Go-side cosine 比对，命中转 alias）SHALL 将硬编码阈值（`auxiliaryLabelMergeThreshold`）提升为 ai_settings `auxiliary_label_dedupe_sim`（默认 0.95，复用现有 merge_embedding 列，不新增 embedding 计算）。embedding 计算或比对失败时 SHALL 降级为直接新建（不阻塞打标管线），并记录告警日志。

#### Scenario: 语义近重复转 alias

- **WHEN** 系统拟新建 "提示词工程"，已有 "Prompt 工程" 且 embedding 相似度 0.97
- **THEN** 系统 SHALL 将 "提示词工程" 写入 "Prompt 工程" 的 aliases，复用既有标签

#### Scenario: embedding 失败降级新建

- **WHEN** embedding 服务调用失败
- **THEN** 系统 SHALL 记录告警并直接新建标签，不阻断文章打标

### Requirement: 一次性文本变体重复合并迁移

系统 SHALL 提供一次性幂等迁移：按归一化键（与创建闸同一函数）对 active 辅助标签分组，同组多条的合并——ref_count 最大者（并列取 id 最小）为主标签；合并 SHALL 复用现有 `MergeAuxiliaryLabelAlias(sourceID, targetID)` 逻辑（从 label 入主 aliases、topic_tag_semantic_labels 与 board_composition 引用改指主、从 status=disabled、主 ref_count 重算为 DISTINCT topic_tag 引用数）。迁移 SHALL 输出每组主/从明细日志，SHALL NOT 自动处理仅 embedding 近似的语义对（仅列报告供人工）。迁移完成后 SHALL invalidate 版块数据缓存（board_composition 已变更）。

#### Scenario: 合并文本变体组

- **GIVEN** active 辅助标签 "SK海力士"(ref 24, id 小)、"SK 海力士"(ref 14)、"SK海力士"(ref 4)
- **WHEN** 执行迁移
- **THEN** 系统 SHALL 保留首条 "SK海力士" 为主标签，另两条 label 入其 aliases、引用改指主标签、status 置 disabled，主标签 ref_count 重算为合并后实际引用数

#### Scenario: 迁移幂等可重跑

- **WHEN** 迁移已执行过一次，再次执行
- **THEN** 系统 SHALL 检出无可合并分组，不产生任何变更

#### Scenario: 语义对仅报告不合并

- **WHEN** "哈梅内伊" 与 "阿里·哈梅内伊" embedding 近似但归一化键不同
- **THEN** 迁移 SHALL 仅在报告中列出该对，不改动数据
