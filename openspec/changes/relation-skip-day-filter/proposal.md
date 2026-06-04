## Why

`MatchAndSaveRelations` 对每个新 section 匹配同 board 下**所有非当日** section，导致跨多天的 section 之间产生大量低质量 relation（如 6/1→6/3 跳过 6/2 直接连线）。这些跳跃连线将 6/3 sections 的入度吹到 13-14，使 status 推导严重失真（`continuing` 被误标为 `split`，`merge` 淹没在噪声中）。实际数据验证：相邻天 6/1→6/2 的 5 条 relation 全是 dist=0.000（完美延续），而跨天 6/1→6/3 的 37 条 relation 平均 dist=0.266（噪声）。

## What Changes

- **修改** `MatchAndSaveRelations` 的匹配逻辑：对每个新 section 的 embedding 匹配结果按天分组，区分"相邻天匹配"和"跨天匹配"
- **相邻天匹配**（from 和 to 之间无其他天的 section）：distance < 0.35 → 进入竞争过滤
- **跨天匹配**（from 和 to 之间有中间天）：只有当 from section 在中间天无任何延续关系时才考虑，且 distance 阈值收紧到 0.25 → 进入竞争过滤
- **竞争过滤**：对每个新 section 的所有候选 relation，按 distance 排序，如果 best 与 2nd 差距 ≥ 0.03，只保留 best；否则保留 best ± 0.03 范围内的所有候选（真正的 split/merge）
- 已有 relation 数据需要回刷（清除旧 relation，重新按新规则生成）

## Capabilities

### New Capabilities

_(无新 capability)_

### Modified Capabilities

- `section-relations`: 关系写入逻辑增加跨天过滤规则——相邻天 distance < 0.35 进入竞争过滤；跨天匹配需满足"from section 在中间天无延续"且 distance < 0.25 才进入竞争过滤；竞争过滤对每个 section 的候选保留 best 或 best ± 0.03 内的多个候选（split/merge）

## Impact

- **后端**：`repository.go` 的 `MatchAndSaveRelations` 函数改写 SQL 查询逻辑，增加中间天检查 + 竞争过滤；新增 `competitiveFilter` 纯函数
- **后端**：新增或复用 `BackfillRelations(boardID)` 函数用于回刷已有数据
- **数据库**：无需 schema 变更，只需清理并重建 `daily_report_section_relations` 表数据
- **前端**：`BoardThreadBrowser` 和 `SectionLifecyclePanel` 的 relation 连线根据 distance 显示不同粗细/透明度（强关系粗实线、弱关系细淡线），hover 时 tooltip 显示具体距离值
