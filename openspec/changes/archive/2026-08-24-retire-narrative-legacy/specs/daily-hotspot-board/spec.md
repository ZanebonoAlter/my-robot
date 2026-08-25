## REMOVED Requirements

### Requirement: Large abstract tree becomes daily hotspot board
**Reason**: hotspot NarrativeBoard 生成代码已删除，`narrative_board_hotspot_threshold` 仅剩历史迁移 seed 键残留（postgres_migrations.go:393），无运行时消费方。
**Migration**: 死数据随 DROP TABLE narrative_boards 清除；热点发现需求由日报聚类（daily-report-system）承接。

### Requirement: Hotspot board cross-day continuation
**Reason**: 生成代码已不存在。
**Migration**: 无。

### Requirement: Hotspot board narratives continue across days
**Reason**: 生成代码已不存在。
**Migration**: 无。
