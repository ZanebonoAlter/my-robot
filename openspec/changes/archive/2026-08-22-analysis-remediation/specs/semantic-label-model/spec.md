# semantic-label-model Delta

## ADDED Requirements

### Requirement: disabled 标签不保留向量

`semantic_labels` 中 status='disabled' 的行 MUST 将 embedding 与 merge_embedding 置为 NULL（行本体与 aliases 保留）；重新启用时由 llm_extract 重算向量。禁用流程的代码路径 MUST 同步置 NULL，防止新增 disabled 行继续积累向量。

#### Scenario: 禁用标签时向量置 NULL
- **WHEN** 通过 API 或流程将一个 semantic_label 置为 disabled
- **THEN** 该行的 embedding 与 merge_embedding 被置 NULL，向量存储立即释放

#### Scenario: 存量 disabled 行向量清理
- **WHEN** 执行一次性存量清理（分批 UPDATE）
- **THEN** 所有 status='disabled' 且向量非 NULL 的行向量被置 NULL，行数据与 aliases 不受影响

#### Scenario: 重新启用标签
- **WHEN** 一个 disabled 标签被重新启用
- **THEN** 其向量通过 llm_extract 流程重新生成，语义功能恢复
