## 1. 后端：MatchAndSaveRelations 改写

> **SUPERSEDED:** `MatchAndSaveRelations`, `shouldWriteRelation`, `competitiveFilter` 已被匈牙利二分图匹配（`RebuildBoardRelations`）取代。详见 `docs/plans/2026-06-06-bipartite-relation-matching.md`。

- [x] 1.1 改写 `MatchAndSaveRelations` 函数：对每个新 section 的 embedding 匹配结果按天分组，区分相邻天匹配和跨天匹配
- [x] 1.2 实现相邻天匹配逻辑：from 和 to 之间无其他已完成报告天 → distance < 0.35 直接写入
- [x] 1.3 实现跨天匹配逻辑：from 和 to 之间有中间天 → 检查 from_section 是否在中间天有延续关系 + distance < 0.25 才写入
- [x] 1.4 中间天延续检查使用内存邻接表：一次性加载 board 下已有 relation 构建 from→[]to 映射，避免逐条 SQL 查询
- [x] 1.5 重写 `BackfillSectionEmbeddings` Phase 2：将原有的 `LIMIT 1` 最近邻逻辑替换为按 board 调用 `BackfillRelations`（见 2.x）

## 2. 后端：BackfillRelations 回刷函数

- [x] 2.1 新增 `BackfillRelations(boardID)` 函数：删除指定 board 涉及的所有 relation 记录
- [x] 2.2 按日期从早到晚遍历 board 下所有 section，逐个执行带过滤的写入逻辑（中间天判断基于已写入的 relation，使用内存邻接表）
- [x] 2.3 新增 `BackfillAllRelations()` 函数：查询所有有 embedding section 的 board，逐个调用 `BackfillRelations`
- [x] 2.4 （可选）新增 API 端点或 CLI 命令触发回刷

## 3. 自动化测试

- [x] 3.1 为 `MatchAndSaveRelations` 新逻辑编写表级测试（真实 DB 事务回滚模式），覆盖：相邻天直接写入、跨天无延续写入、跨天有延续过滤、跨天距离超阈值过滤
- [x] 3.2 测试日期不连续场景（6/1→6/3 无 6/2 报告时视为相邻天）
- [x] 3.3 测试多匹配组合场景（split：一个 section 同时匹配相邻天多个旧 section）
- [x] 3.4 测试 `BackfillRelations` 全量重建正确性

## 4. 竞争过滤（Competitive Matching）

- [x] 4.1 新增 `matchCandidate` 结构体和 `competitiveFilter(candidates []matchCandidate) []matchCandidate` 纯函数
- [x] 4.2 在 `MatchAndSaveRelations` 中，对 `shouldWriteRelation` 过滤后的候选调用 `competitiveFilter`，只写入过滤后的结果
- [x] 4.3 在 `BackfillRelations` 中同样加入 `competitiveFilter` 调用
- [x] 4.4 为 `competitiveFilter` 编写单元测试：gap ≥ 0.03 只保留 best、gap < 0.03 保留多候选、单候选直通、空候选返回空

## 5. 验证（回刷）

- [x] 5.1 对 board 2853（伊朗局势）重新回刷，验证 section 状态分布变化：merge 占比下降，continuing/split 占比上升
- [x] 5.2 对 board 3639（AI 技术）回刷，验证稠密版块的匹配收敛效果
- [x] 5.3 验证 `DeriveSectionStatuses` 的 `ending` 覆盖逻辑在新 relation 拓扑下无异常
- [x] 5.4 编译通过、受影响包测试通过

## 6. 前端可视化

- [x] 6.1 `BoardThreadBrowser` 连线根据 distance 显示粗细/透明度（<0.15 粗, <0.25 中, >=0.25 细淡）
- [x] 6.2 `SectionLifecyclePanel` 同上
- [x] 6.3 hover 连线时 tooltip 显示距离值（"距离: 0.158"）

## 7. 方向修正（已完成的前置工作）

- [x] 6.1 改写 `MatchAndSaveRelations`：相邻天 < 0.35 进入候选、跨天需无中间延续 + < 0.25 进入候选
- [x] 6.2 新增 `BackfillRelations(boardID)` + `BackfillAllRelations()` 回刷函数
- [x] 6.3 重写 `BackfillSectionEmbeddings` Phase 2
- [x] 6.4 新增 `POST /api/daily-reports/backfill-relations` 端点
- [x] 6.5 修复匹配方向 bug（`!=` → `<` 确保只匹配更早日期）
- [x] 6.6 `shouldWriteRelation` 纯逻辑单元测试（7 个场景全部通过）
