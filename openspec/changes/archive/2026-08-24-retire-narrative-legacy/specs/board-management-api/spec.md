## MODIFIED Requirements

### Requirement: 板块 CRUD API
系统 SHALL 提供 SemanticBoard（semantic_labels label_type=board）的增删改查 API，包括列表、详情、手动创建、编辑、删除。API SHALL 使用 `/api/semantic-boards` 命名空间。

#### Scenario: 手动创建板块
- **WHEN** 用户通过 API 创建板块 "量子计算"
- **THEN** 系统 SHALL 创建 semantic_label（label_type="board", source="manual", protected=true），生成 embedding

#### Scenario: 列出板块
- **WHEN** 用户请求板块列表
- **THEN** 系统 SHALL 返回所有 label_type="board" 且 status="active" 的 semantic_labels，包含 ref_count 和 tag_count
