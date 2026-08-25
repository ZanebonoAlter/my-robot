## REMOVED Requirements

### Requirement: 生成结束后清理空 board
**Reason**: 清理对象是 narrative_boards/narrative_summaries（GenerateAndSaveForCategory / GenerateAndSave / GenerateNarrativesForBoard / cleanEmptyBoards 已全部不存在于代码库），无从清理。
**Migration**: DROP TABLE 后空 board 概念随之消失。

### Requirement: 全局生成后清理空 board
**Reason**: 同上。
**Migration**: 无。
