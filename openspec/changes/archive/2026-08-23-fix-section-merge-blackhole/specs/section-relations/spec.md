# Delta Spec: section-relations

## MODIFIED Requirements

### Requirement: 同日 Section 两阶段合并
日报生成管线中、`SaveReport()` 落库之前，系统 SHALL 对同日 sections 执行两阶段合并以消除聚类过碎问题。合并整体受配置开关 `daily_report_section_merge_enabled`（存 `ai_settings`，默认 false）控制：开关关闭时系统 SHALL 跳过两阶段合并，sections 按上游 lane 管线原始分组原样落库。

**锚定边界（前置过滤）**
lane 管线的 keep/switch/new 归因裁决是系统记录，展示层合并不得跨越。所有合并候选对（Stage 1 确定性与 Stage 2 灰区）在建边前 SHALL 先过锚定边界校验，仅以下两类 pair 允许合并：

- 双方 `MatchedTopicID` 均非 NULL 且相等（同话题当日分组）；
- 双方 `MatchedTopicID` 均 NULL（同属新叙事/未锚定池）。

`MatchedTopicID` 不同、或 NULL 与非 NULL 混合的 pair SHALL 被拒绝：不进入确定性合并、不进入 LLM 仲裁、不参与传递闭包。边界过滤在建边前执行保证传递闭包的连通分量内锚定必然一致。

**Stage 1：确定性合并（embedding）**
系统 SHALL 计算所有通过锚定边界的同日 section pairs 的 embedding cosine distance。distance < 0.20 的 pairs SHALL 自动合并为一个 section。

合并规则：
- 保留 `article_count` 最大的 section 作为主 section
- 被合并 section 的 threads SHALL 迁移到主 section 下
- 主 section 的 `cluster_label` 不变
- `cluster_tag_ids` SHALL 合并两个 section 的 tag IDs
- `article_count`、`best_tier`、`avg_score` SHALL 重新计算（合并后的值）
- 被合并 section SHALL 从 sections 列表中移除

连通性：如果 A↔B 和 B↔C 都是合法边且距离均 < 0.20，则 A、B、C SHALL 合并为一个 section（使用传递闭包；边界过滤在建边前执行，闭包不会跨越锚定）。

**Stage 1 审计**
确定性合并的每个候选对（含被锚定边界拒绝的对）SHALL 记录审计日志：双方 `cluster_label`、`MatchedTopicID`、lane_tier、距离、合并或拒绝结果。与 Stage 2 灰区仲裁的 LLM 调用日志共同构成可回放审计面。

**Stage 2：LLM 仲裁（灰色地带）**
通过锚定边界且距离在 0.20 - 0.25 之间的 pairs SHALL 批量送 LLM 判断是否合并。

LLM 输入：每个 candidate pair 的 `(section_a_label, section_a_tag_labels[], section_b_label, section_b_tag_labels[])`。
LLM 输出：`merge_pairs: [[index_a, index_b], ...]` 列表。
LLM 判定为合并的 pairs SHALL 按 Stage 1 相同规则合并。

合并完成后，系统 SHALL 继续 relation 写入逻辑（基于合并后的 sections）。

#### Scenario: 合并开关关闭
- **WHEN** `daily_report_section_merge_enabled=false`（默认）且同日存在多个语义相近的 sections
- **THEN** 系统 SHALL 跳过两阶段合并，sections 按 lane 管线原始分组落库，不产生任何合并

#### Scenario: Stage 1 确定性合并（同话题）
- **WHEN** sections [A, B] 同属 topic 7（MatchedTopicID 均为 7），distance=0.15
- **THEN** 系统 SHALL 合并 A 和 B 为一个 section（保留 article_count 更大的），移除另一个

#### Scenario: 不同话题拒绝合并
- **WHEN** sections [A(topic 7), B(topic 12)] distance=0.11
- **THEN** 系统 SHALL 拒绝合并，A、B 各自独立落库，该 pair 不进入 LLM 仲裁

#### Scenario: 新叙事不被锚定 section 吸收
- **WHEN** sections [A(topic 7, l1_direct), B(MatchedTopicID=NULL, l3_new)] distance=0.14
- **THEN** 系统 SHALL 拒绝合并，B 作为独立新叙事 section 落库并在 SaveReport 时走 auto_new 创建 candidate topic

#### Scenario: 两个新叙事 section 可合并
- **WHEN** sections [A(NULL, l3_new), B(NULL, l3_new)] distance=0.18
- **THEN** 系统 SHALL 允许该 pair 进入正常两阶段合并流程

#### Scenario: 传递闭包不跨越锚定边界
- **WHEN** sections [A(topic 7), B(topic 7), C(topic 12)]，A↔B=0.15（合法边），B↔C=0.18（跨界被拒）
- **THEN** 系统 SHALL 仅合并 A、B，C 独立落库

#### Scenario: Stage 2 LLM 仲裁合并
- **WHEN** sections [A, B] 同属 topic 7，distance=0.21，LLM 判定为 merge=true
- **THEN** 系统 SHALL 合并 A 和 B

#### Scenario: Stage 2 LLM 仲裁拒绝合并
- **WHEN** sections [A, B] 同属 topic 7，distance=0.23，LLM 判定为 merge=false
- **THEN** 系统 SHALL 保留 A 和 B 为独立 section

#### Scenario: 无灰色地带 pairs
- **WHEN** 同日所有通过边界校验的 section pairs 距离均 < 0.20 或 > 0.25
- **THEN** 系统 SHALL 跳过 Stage 2，不调用 LLM

#### Scenario: 确定性合并审计日志
- **WHEN** Stage 1 处理候选对 [A(topic 7), B(topic 12)] distance=0.19 且被锚定边界拒绝
- **THEN** 系统 SHALL 记录含双方 label、MatchedTopicID、lane_tier、距离、拒绝原因的审计日志
