## MODIFIED Requirements

### Requirement: 冷启动允许无 SemanticBoard
系统 SHALL 允许冷启动阶段没有任何 SemanticBoard。无 SemanticBoard 时，tag SHALL 仍然提取和积累辅助标签；日报生成 SHALL 自然产出空结果且不报错，直到用户确认创建第一批 SemanticBoard 并回填。

#### Scenario: 冷启动无 board
- **WHEN** 系统尚无 label_type="board" 的 semantic_labels
- **THEN** tag 提取 SHALL 正常写入辅助标签，board 匹配 SHALL 返回无归属且不报错

#### Scenario: 冷启动初始化建议
- **WHEN** 辅助标签池累计到升级阈值且用户手动触发升级建议
- **THEN** 系统 SHALL 基于当前辅助标签池生成第一批 SemanticBoard 建议
