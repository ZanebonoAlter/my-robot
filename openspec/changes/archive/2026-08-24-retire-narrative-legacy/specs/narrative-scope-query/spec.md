## REMOVED Requirements

### Requirement: 分类版块列表数据源
**Reason**: 依赖 narrative_summaries 的 scope 查询，消费端点与前端均已删除，零生产写入方。
**Migration**: 死数据随 DROP TABLE 清除，无降级路径。

### Requirement: GetScopes 支持多日范围
**Reason**: 同上——scope 查询 API 已无消费方。
**Migration**: 无。

### Requirement: 切换到分类模式时加载分类列表
**Reason**: 前端分类模式切换 UI 已不存在。
**Migration**: 无。
