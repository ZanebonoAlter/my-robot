# tagging-domain Delta — fix-quality-audit-p0

## ADDED Requirements

### Requirement: topic_tags.feed_count 周期对账
系统 SHALL 在 TagQualityScoreJob（周期调度）中对 `topic_tags.feed_count` 执行全量对账：将 feed_count 重算为 `COUNT(DISTINCT articles.feed_id)`（经 `article_topic_tags` 关联）。对账失败 SHALL 记录 warning 日志且不中断 job 其余步骤，与既有 auxiliary ref_count 对账的容错模式一致。

#### Scenario: 打标漂移后被对账修正
- **WHEN** 新文章打标使某 tag 实际 distinct feed 引用数为 5，而 `topic_tags.feed_count` 仍为旧值 3
- **THEN** 下一次 TagQualityScoreJob 运行后该 tag 的 `feed_count` 为 5

#### Scenario: 无引用标签对账为零
- **WHEN** 某 tag 的所有 article 关联被清除（如 hard merge 后残留）
- **THEN** 对账后该 tag 的 `feed_count` 为 0

#### Scenario: 对账失败不中断 job
- **WHEN** feed_count 对账 SQL 执行失败
- **THEN** job 记录 warning 日志并继续执行后续步骤（quality score 计算等），job 不返回失败
