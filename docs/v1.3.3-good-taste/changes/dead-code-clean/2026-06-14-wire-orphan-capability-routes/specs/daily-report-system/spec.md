## ADDED Requirements

### Requirement: 日报 LLM 调用路由绑定
日报生成的所有 LLM 调用（事件标签语义聚类、要闻 highlights 生成、叙事线程 narrative 生成）SHALL 通过 `digest_polish` capability 加载路由与 provider，SHALL NOT 复用 `topic_tagging` 路由。这使得日报可独立配置 provider、并发上限与温度，不再与标签提取共享配额。

#### Scenario: 语义聚类调用使用 digest_polish
- **WHEN** `daily_report_cluster` 对去重后的事件标签执行 LLM 语义分组
- **THEN** LLM 调用 SHALL 使用 `digest_polish` capability

#### Scenario: 要闻与叙事调用使用 digest_polish
- **WHEN** `daily_report_llm` 生成要闻（highlights）或叙事线程（narrative）
- **THEN** LLM 调用 SHALL 使用 `digest_polish` capability

#### Scenario: 日报独立配置 provider
- **WHEN** 用户在能力路由面板为 `digest_polish` 配置了与 `topic_tagging` 不同的 provider
- **THEN** 日报生成 SHALL 使用 `digest_polish` 配置的 provider，标签提取 SHALL NOT 受影响
