## Why

当前标签合并建议队列完全依赖 embedding 余弦相似度（阈值 0.92）来判断标签对是否相似。但即使在高相似度区间，纯 embedding 仍无法区分关键差异模式：

- **数值不同**：`"沪深两市成交额突破1万亿元"` vs `"沪深两市成交额突破1.5万亿元"`（不同天的新闻）
- **同一实体不同事件**：`"凯文·沃什承诺对美联储进行大规模整顿"` vs `"凯文·沃什宣誓就任美联储主席"`
- **上下位关系**：`"伊朗伊斯兰革命卫队海军"` vs `"伊朗伊斯兰革命卫队"`

数据分析显示，pending 队列中约 20% 属于"数值不同但主题相同"，15% 属于"相关但独立的事件"——这些都不应该被建议合并。

## What Changes

- 在 `TagMatch` 和 `runFullScan` 中，对候选标签对增加**实体+数值提取**前置过滤
- 提取标签中的命名实体（人名、机构名、地名等）和数值（金额、百分比、数量等）
- 如果两个标签的提取数值集合不同 → 排除该候选对，不写入建议表
- 如果两个标签的实体集合无交集 → 排除该候选对
- 新增 Go 包 `tagging/entity` 实现实体提取逻辑（纯 Go 实现，不依赖 Python）

## Capabilities

### New Capabilities
- `tag-entity-extraction`: 标签文本的实体和数值提取能力，用于增强标签匹配判断

### Modified Capabilities
- `tag-merge-suggestions`: 增量和全量扫描路径增加实体+数值过滤，减少误报
- `tag-match-identity-protection`: TagMatch 的候选结果增加实体过滤步骤

## Impact

- `backend-go/internal/domain/tagging/`: 新增 entity 提取包，修改 `embedding.go`、`tag_merge_suggest.go`
- 不影响前端
- 不影响数据库 schema
- 向后兼容：现有 pending 建议不受影响，仅影响新增建议的质量
