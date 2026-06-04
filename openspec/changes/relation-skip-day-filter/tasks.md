## 1. 后端：MatchAndSaveRelations 改写

- [ ] 1.1 改写 `MatchAndSaveRelations` 函数：对每个新 section 的 embedding 匹配结果按天分组，区分相邻天匹配和跨天匹配
- [ ] 1.2 实现相邻天匹配逻辑：from 和 to 之间无其他已完成报告天 → distance < 0.35 直接写入
- [ ] 1.3 实现跨天匹配逻辑：from 和 to 之间有中间天 → 检查 from_section 是否在中间天有延续关系 + distance < 0.25 才写入
- [ ] 1.4 中间天延续检查使用内存邻接表：一次性加载 board 下已有 relation 构建 from→[]to 映射，避免逐条 SQL 查询
- [ ] 1.5 重写 `BackfillSectionEmbeddings` Phase 2：将原有的 `LIMIT 1` 最近邻逻辑替换为按 board 调用 `BackfillRelations`（见 2.x）

## 2. 后端：BackfillRelations 回刷函数

- [ ] 2.1 新增 `BackfillRelations(boardID)` 函数：删除指定 board 涉及的所有 relation 记录
- [ ] 2.2 按日期从早到晚遍历 board 下所有 section，逐个执行带过滤的写入逻辑（中间天判断基于已写入的 relation，使用内存邻接表）
- [ ] 2.3 新增 `BackfillAllRelations()` 函数：查询所有有 embedding section 的 board，逐个调用 `BackfillRelations`
- [ ] 2.4 （可选）新增 API 端点或 CLI 命令触发回刷

## 3. 自动化测试

- [ ] 3.1 为 `MatchAndSaveRelations` 新逻辑编写表级测试（真实 DB 事务回滚模式），覆盖：相邻天直接写入、跨天无延续写入、跨天有延续过滤、跨天距离超阈值过滤
- [ ] 3.2 测试日期不连续场景（6/1→6/3 无 6/2 报告时视为相邻天）
- [ ] 3.3 测试多匹配组合场景（split：一个 section 同时匹配相邻天多个旧 section）
- [ ] 3.4 测试 `BackfillRelations` 全量重建正确性

## 4. 验证

- [ ] 4.1 对 board 2853 执行回刷，验证 6/3 section 入度从 13-14 降到 5-6
- [ ] 4.2 验证 690→932 (dist=0.094) 真隔天续上关系被保留
- [ ] 4.3 验证 692→932 (有 6/2 延续) 等冗余噪声 relation 被过滤
- [ ] 4.4 验证回刷后 `DeriveSectionStatuses` 的 `ending` 覆盖逻辑在新 relation 拓扑下无异常
- [ ] 4.5 后端编译通过、lint 通过、受影响包测试通过
