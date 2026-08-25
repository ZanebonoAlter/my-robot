## MODIFIED Requirements

### Requirement: Tag 可属于多个 Board
系统 SHALL 允许一个 tag 同时属于多个 SemanticBoard。所有满足匹配规则的 board SHALL 按匹配分从高到低排序，默认最多保留 3 个。同一 event tag 及其文章 SHALL 允许出现在多个版块的日报产物中。每条 `topic_tag_board_labels` 记录 SHALL 包含 `downgraded` 标记。

#### Scenario: 多视角挂载含降级标记
- **WHEN** tag "霍尔木兹海峡" 同时满足 board "地缘政治"（命中率 75%，downgraded=false）和 board "能源安全"（max_sim 0.82，N=1 降级，downgraded=true）
- **THEN** tag SHALL 同时挂载到两个 board，各自的 downgraded 标记独立记录

#### Scenario: 超过归属上限时截断
- **WHEN** tag "AI芯片出口管制" 匹配到 5 个 SemanticBoard，semantic_board_match_max_boards=3
- **THEN** 系统 SHALL 仅保留匹配分最高的 3 个 topic_tag_board_labels 记录
