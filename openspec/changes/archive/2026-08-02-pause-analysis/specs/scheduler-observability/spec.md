## ADDED Requirements

### Requirement: 调度器状态体现分析暂停语义
当 analysis_paused 为 true 时，受影响的分析类调度器在 GET /api/schedulers 返回中 SHALL 体现"暂停"状态语义（如 status 含 paused 标记或暂停说明），使暂停态在调度器可观测面板端到端可见。

#### Scenario: 暂停时调度器状态可见
- **WHEN** analysis_paused 为 true 且调用 GET /api/schedulers
- **THEN** content_completion 等受影响调度器的状态条目体现暂停语义（如显示 paused 标记）

#### Scenario: 恢复后调度器状态复原
- **WHEN** analysis_paused 切回 false
- **THEN** 受影响调度器状态恢复正常 idle/running 语义
