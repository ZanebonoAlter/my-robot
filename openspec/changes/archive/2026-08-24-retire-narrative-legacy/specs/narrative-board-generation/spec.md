## REMOVED Requirements

### Requirement: SemanticBoard 派生每日 NarrativeBoard
**Reason**: NarrativeBoard 生成函数（CollectSemanticBoardNarrativeInputs / DeriveBoardRelations 等）已全部删除，narrative_boards 表零生产写入方；职责由 board_daily_reports（daily-report-system）承接。
**Migration**: 死数据随 DROP TABLE narrative_boards 清除，无降级路径；历史设计见归档 changes/2026-04-30-narrative-board-system。

### Requirement: 取消 abstract tree 热点板路径
**Reason**: 描述的是被删除的旧热点板路径的否定性约束，随生成系统整体下线失去意义。
**Migration**: 无——该约束针对的代码路径已不存在。

### Requirement: 冷启动无 SemanticBoard 时不生成 NarrativeBoard
**Reason**: NarrativeBoard 生成已不存在；等价语义（冷启动无 board 不报错）由 board-upgrade「冷启动允许无 SemanticBoard」与 daily-report-system 承接。
**Migration**: 见 board-upgrade spec 的冷启动 Requirement。

### Requirement: 多板块归属允许事件重复展示
**Reason**: 多 board 挂载行为的权威表述在 tag-to-board-matching「Tag 可属于多个 Board」，本条为叙事侧重复表述。
**Migration**: tag-to-board-matching spec 本 change 同步修正措辞。

### Requirement: NarrativeBoard 通过 semantic_board_id 续接
**Reason**: 跨日续接职责已由 board_daily_reports.prev_report_id 与 daily_report_section_relations 承接。
**Migration**: 见 daily-report-system「日报关联昨日报告」Scenario。

### Requirement: Board 叙事上下文来自 SemanticBoard
**Reason**: 生成链路已删除，无对应行为。
**Migration**: 无。
