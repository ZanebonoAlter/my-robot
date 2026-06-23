## MODIFIED Requirements

### Requirement: 增量记录合并建议

`findOrCreateTag` 创建新 tag 时，如果 `TagMatch` 返回 candidates，SHALL 将候选对写入 `tag_merge_suggestions`。在写入之前，SHALL 对每个候选对调用 `ShouldConsiderMerge` 进行实体+数值过滤。

#### Scenario: 新标签有相似候选且通过实体过滤
- **WHEN** `findOrCreateTag` 的 `TagMatch` 返回 `candidates`
- **AND** `ShouldConsiderMerge(newLabel, candidateLabel)` 返回 `true`
- **AND** 新 tag 创建成功（获得 new_tag_id）
- **THEN** 对该 candidate 写入一条 `tag_merge_suggestion`（status=pending, source=incremental）
- **AND** 以 `(new_tag_id, candidate_tag_id)` 为唯一键，已存在则 skip

#### Scenario: 新标签有相似候选但被实体过滤排除
- **WHEN** `findOrCreateTag` 的 `TagMatch` 返回 `candidates`
- **AND** `ShouldConsiderMerge(newLabel, candidateLabel)` 返回 `false`
- **THEN** 不写入该 candidate 的 suggestion
- **AND** 记录 debug 日志：`"entity filter rejected: labelA vs labelB reason=numeric_mismatch"`

#### Scenario: 新标签无相似候选
- **WHEN** `TagMatch` 返回 `no_match` 或 `exact`
- **THEN** 不写入 suggestion

### Requirement: 异步全量扫描

用户可手动触发全量扫描，遍历所有 active tag，每个 tag 调用 `FindSimilarTags` 查找相似对，结果写入 `tag_merge_suggestions`。在写入之前，SHALL 对每个候选对调用 `ShouldConsiderMerge` 进行实体+数值过滤。

#### Scenario: 触发全量扫描
- **WHEN** 收到 `POST /merge-preview/scan` 请求
- **THEN** 后端在 goroutine 中启动扫描，立即返回 202
- **AND** 遍历所有 active tag，每个 tag 调用 `FindSimilarTags(tag, category, 10, 'semantic')`
- **AND** 对每个相似度 ≥ LowSimilarity 的结果，调用 `ShouldConsiderMerge`
- **AND** 仅将通过过滤的结果写入 suggestion（source=full_scan）
- **AND** 被过滤掉的计入 `filtered_count`，在 SSE 进度中报告
- **AND** 已存在的对 skip，不重复写入

#### Scenario: SSE 进度增加过滤统计
- **WHEN** 全量扫描推送 SSE 进度
- **THEN** 进度消息增加 `filtered` 字段，表示被实体过滤排除的候选对数量
```json
{
  "status": "scanning",
  "total": 590,
  "scanned": 342,
  "new_suggestions": 23,
  "filtered": 15
}
```
