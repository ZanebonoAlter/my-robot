# tag-embedding-management Delta

## ADDED Requirements

### Requirement: tag 删除时 embedding 级联清理（DB FK 兜底）

系统 SHALL 保证 `topic_tag_embeddings` 中不存在指向已删除 `topic_tags` 的孤儿行：数据库层 MUST 建立外键约束（topic_tag_id → topic_tags.id）并声明 `ON DELETE CASCADE`，与 GORM 模型的 `OnDelete:CASCADE` 声明保持一致。

#### Scenario: 删除 tag 触发向量级联删除
- **WHEN** 通过任何路径删除一条 topic_tag 记录
- **THEN** 该 tag 关联的所有 topic_tag_embeddings 行被数据库自动删除，无孤儿残留

#### Scenario: 外键约束生效
- **WHEN** 向 topic_tag_embeddings 插入一条 topic_tag_id 指向不存在 tag 的记录
- **THEN** 数据库拒绝该写入（FK 违反）

### Requirement: 存量孤儿 embedding 一次性清理

提供一次性维护脚本，分批（单批 ≤5 万行）删除 `topic_tag_embeddings` 中 `topic_tag_id` 无对应 `topic_tags.id` 的行，执行前 MUST 备份相关表、执行后 MUST 复核孤儿计数为 0，然后才允许执行加 FK 迁移。

#### Scenario: 清理后加 FK
- **WHEN** 存量清理完成且孤儿计数复核为 0
- **THEN** FK 迁移执行成功，后续删除自动级联
