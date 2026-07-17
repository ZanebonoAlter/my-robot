## MODIFIED Requirements

### Requirement: LLM 判断升级/跳过

系统 SHALL 在触发建议生成（手动或 scheduler）时，将每个簇（≥2 个辅助标签）的辅助标签列表 + co-tag 事件 + 候选版块 shortlist 及其证据发送给 LLM，由 LLM 判断：create_new（升级为新 board）、merge_into_existing（并入 shortlist 内某个已有版块）或 skip（暂不处理）。LLM 输出的 merge_into_existing.target_board_id SHALL 校验必须属于该簇的候选版块 shortlist，非法值降级为 skip。单标签簇 SHALL NOT 进入 LLM 判断（走观察池，见新增需求）。

#### Scenario: LLM 判断创建新 board

- **WHEN** 簇 [新能源, 光伏, 储能] 与所有候选版块语义均不吻合，LLM 判断应升级
- **THEN** 系统 SHALL 返回 create_new 建议，包含 board 名称、描述和候选辅助标签

#### Scenario: LLM 判断并入已有版块

- **WHEN** 簇 [DeepSeek, Agent] 的 shortlist 含版块「生成式 AI 与大模型厂商」，且该版块泳道近期内容与簇内标签语义吻合
- **THEN** LLM SHALL 返回 merge_into_existing 建议，target_board_id 指向该版块

#### Scenario: merge 目标必须在 shortlist 内

- **WHEN** LLM 返回 merge_into_existing 且 target_board_id 不在该簇 shortlist 中
- **THEN** 系统 SHALL 将该决策降级为 skip，不产出建议

#### Scenario: LLM 判断跳过

- **WHEN** 簇内辅助标签过于分散，LLM 判断不足以形成板块也不属于任何候选版块
- **THEN** 系统 SHALL 返回 skip 决策，该簇不产出建议

## ADDED Requirements

### Requirement: 双签名候选版块 shortlist

系统 SHALL 为每个簇计算候选版块 shortlist：composition 签名（簇内 aux embedding 对各版块 composition aux embedding 的 min-distance）取 top-2，泳道签名（对各版块 active topic 近 30 天 section embedding 的 min-distance）取 top-2，去重后至多 4 个候选版块。版块无 active topic section 时，该版块仅参与 composition 签名。shortlist SHALL 附带各签名的距离与 margin 供置信度判定与 prompt 展示。

#### Scenario: 双签名 shortlist 生成

- **WHEN** 簇 [DeepSeek, Agent] 参与建议生成
- **THEN** 系统 SHALL 输出至多 4 个候选版块，含 composition top-2 与泳道 top-2 及其距离

#### Scenario: 无泳道内容的版块降级

- **WHEN** 某版块近 30 天无 active topic 的 section
- **THEN** 该版块 SHALL 仅通过 composition 签名参与 shortlist

### Requirement: 高置信 merge 免 LLM

当簇的 composition 签名与泳道签名 top-1 指向同一版块，且**两个签名各自的 margin（各签名 top-1 与 top-2 距离差）均** ≥ `semantic_board_upgrade_merge_confidence_margin`（默认 0.05，ai_settings 可配）时，系统 SHALL 直接产出 merge_into_existing 建议并标记 confidence=high，不再调用 LLM；其余簇（签名分歧，或任一签名 margin 不足）SHALL 走 LLM 裁决并标记 confidence=llm。

#### Scenario: 双签名一致且 margin 达标

- **WHEN** 簇 [OpenAI Codex, Codex CLI] 双签名 top-1 均为「生成式 AI 与大模型厂商」且 margin=0.077
- **THEN** 系统 SHALL 直接产出 confidence=high 的 merge 建议，不调用 LLM

#### Scenario: 签名分歧走 LLM

- **WHEN** 簇的 composition top-1 与泳道 top-1 指向不同版块
- **THEN** 系统 SHALL 将该簇提交 LLM 裁决，产出 confidence=llm 的建议或 skip

### Requirement: 候选版块泳道内容证据注入

系统 SHALL 在 LLM prompt 中为每个候选版块注入其 active topic 的近期内容摘要（近 30 天 section 标题，每版块至多 5 条），使 LLM 依据版块近期实际叙事判断 merge，而非仅凭版块名称与 aux 标签字面。拉取失败时 SHALL 降级为仅注入版块名称与描述，不阻断建议生成。

#### Scenario: prompt 注入泳道内容

- **WHEN** 候选版块「生成式 AI 与大模型厂商」有 active topic 近 30 天 section
- **THEN** LLM prompt SHALL 包含该版块名称、描述及至多 5 条近期 section 标题

#### Scenario: 泳道内容拉取失败降级

- **WHEN** 泳道内容查询失败
- **THEN** 系统 SHALL 仅以版块名称与描述注入，继续生成流程

### Requirement: 单标签簇观察池

聚类后仅含 1 个辅助标签的簇 SHALL NOT 进入 LLM，系统 SHALL 为其写入 decision=watch 的 pending 建议；当该标签在后续生成轮次中与其他候选成簇（≥2）时，系统 SHALL 正常参与裁决并自动关闭对应 watch 建议。watch 建议 SHALL NOT 出现在默认建议列表。

#### Scenario: 单标签簇入观察池

- **WHEN** 某轮生成中标签 "Fable 5" 自成一簇
- **THEN** 系统 SHALL 写入 decision=watch 的 pending 建议，不调用 LLM

#### Scenario: 观察池标签成簇后关闭

- **WHEN** 下一轮生成中 "Fable 5" 与 "Fable" 成簇并产出正式建议
- **THEN** 系统 SHALL 将原 watch 建议置为 confirmed 并从观察池移除
