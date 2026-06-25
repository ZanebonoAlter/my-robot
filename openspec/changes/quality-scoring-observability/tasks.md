# Tasks: 日报质量排序可观测

> 垂直切片，每切片独立可交付、可验证。按数据血缘顺序：①后端血缘下沉（治 🔴 事实1）→ ②MatchTier 语义+截断修复 → ③API 暴露 → ④前端工具上移+探究区明细 → ⑤正文 tier 徽章。尾部遵循开发执行规范 §11 归档门禁。

## 1. 后端数据血缘下沉（A · daily-report-system）

- [x] 1.1 版本化迁移 `20260625_*`：`daily_report_sections` 加 `quality_breakdown JSONB NULL`（无外键约束，兼容历史 NULL）。验收：迁移在 testcontainer 幂等执行、可 DROP COLUMN 回滚
- [x] 1.2 `daily_report_models.go`：`TagInput` 加 `Downgraded bool` 字段；`DailyReportSection` 加 `QualityBreakdown JSON \`gorm:"type:jsonb"\` 字段。验收：AutoMigrate/迁移增列、结构体字段就位
- [x] 1.3 `daily_report_orchestrator.go` collectBoardTags：SELECT 补 `topic_tag_board_labels.downgraded`（两处来源：主路径 row 扫描 + fallback 路径 boardMatch），填入 `TagInput.Downgraded`。验收：降级 tag 的 Downgraded=true 进管线（治 🔴 事实1）
- [x] 1.4 `daily_report_orchestrator.go` section 组装处（约 :164）：从当前作用域 `tags` 切片按 cluster.TagIDs 填充 `quality_breakdown`（结构 `[{tag_id,label,match_reason,score,downgraded}]`）。验收：新 section 持久化含完整明细
- [x] 1.5 `daily_report_merge.go` MergeSimilarSections：合并后按合并后的 tagIDSet 重算 `quality_breakdown`（与重算 avg_score 同处、同规则）。验收：合并 section 的明细=各成员明细并集，avg_score 与之一致

## 2. 后端 MatchTier 语义与截断修复（B · daily-report-system）

- [x] 2.1 `daily_report_orchestrator.go` filterTagsByQuality 截断排序：`MatchTier(kept[i].MatchReason, false)` 改为 `MatchTier(kept[i].MatchReason, kept[i].Downgraded)`（修硬编码，依赖 1.3 的 Downgraded）。验收：降级 max_sim 正确落到 tier=3
- [ ] 2.2 编写 MatchTier 语义表单测（内存 SQLite 纯逻辑）：覆盖 direct_hit→0 / hit_rate→1 / max_sim非降级→2 / max_sim降级→3 / weighted→3 五分支。验收：五分支返回值符合 design D3 映射表
- [ ] 2.3 编写 best_tier 聚合单测：组内多 tier 取最优（min）。验收：{0,2,3} → best_tier=0

## 3. 后端 API 暴露（C · daily-report-system）

- [ ] 3.1 section / timeline / lifeline 接口的 section 表示序列化 `quality_breakdown`（JSON 字段，可为 null）。验收：前端能取到字段（数组或 null）
- [ ] 3.2 历史 section（quality_breakdown=NULL）接口不报错、返回 null。验收：历史行兼容

## 4. 前端工具函数上移 + 探究区 tag 级明细（C · 复用 match-score-visualization 语义）

- [ ] 4.1 `matchReasonColor` / `matchInfoLabel` 从 tags feature 现位置上移到共享 utils（保留原 TagsPage 调用点 import 同步改）。验收：codegraph impact 确认调用面无遗漏、原 TagsPage 行为不变
- [ ] 4.2 日报 section 探究区（hover/详情）展示 `quality_breakdown`：每条 tag chip 用 match_reason 色（复用四色系）+ score 文字 + 降级表现（50% 不透明 + "↓"）。颜色 MUST 由主题 token 派生、跟随双主题。验收：探究区明细可读、降级标记清晰、双主题正确
- [ ] 4.3 历史 section（quality_breakdown=null）探究区降级文案"无质量明细"。验收：不报错、有占位
- [ ] 4.4 normalizer 层 snake_case→camelCase（`camelizeKeys()`），组件内只用 camelCase；数字 tag_id 在 API 边界转字符串。验收：命名转换合规

## 5. 前端正文 Tier 徽章（C · 不破沉浸）

- [ ] 5.1 新增 tier 徽章组件：best_tier 0/1/2 实心点（绿/蓝/橙 token）、3 空心点（灰 token），**无任何分数/百分比/匹配方式文字**。颜色由主题语义 token 派生、跟随 editorial/dark 双主题。验收：四态可区分、双主题清晰、零文字
- [ ] 5.2 徽章接 best_tier（独立冻结字段，不依赖 quality_breakdown）；历史 section（quality_breakdown=null 但 best_tier 存在）正常展示徽章。验收：历史 section 徽章正常
- [ ] 5.3 浏览器视觉验收（cmd，桌面 1280px）：徽章不破坏正文沉浸、与既有 section 布局不冲突。验收：人工核验（如发现打扰，design 后期加默认关闭开关）
- [ ] 5.4 复用项目组件库（若徽章需交互用 AppButton/AppDialog，禁原生 button 样式类、禁 window.*）。验收：零原生弹窗

## 6. 架构体检（§7 强制，每个子任务后）

- [ ] 6.1 `codegraph impact filterTagsByQuality` / `codegraph impact MatchTier`：波及面无 HIGH/CRITICAL 被忽略（MatchTier 调用变更重点核）
- [ ] 6.2 `codegraph affected daily_report_orchestrator.go` / `daily_report_merge.go` / `daily_report_models.go`：受影响测试范围符合预期
- [ ] 6.3 `matchReasonColor`/`matchInfoLabel` 上移后 `codegraph impact` 确认原调用面 import 全部更新
- [ ] 6.4 分层合规：改动全在 `internal/topicgraph/`（后端）+ tags feature（前端），无循环依赖

## 7. 测试（§4.2 双层）

- [ ] 7.1 后端纯单元（内存 SQLite `glebarez/sqlite` mode=memory，参考 `feed_service_test.go`）：quality_breakdown JSON 组装、MatchTier 五分支、best_tier 聚合、filterTagsByQuality 截断排序（含真实 downgraded）
- [ ] 7.2 后端集成（testcontainer pgvector `testutil.SetupTestDB`）：quality_breakdown 列迁移幂等、新日报写入明细、合并重算、历史行 NULL 兼容、API 序列化字段
- [ ] 7.3 前端单测（Vitest + happy-dom）：tier 徽章四态样式映射、探究区明细渲染（含降级标记）、历史 null 降级文案、matchReasonColor/matchInfoLabel 复用正确

## 8. 数据兼容性（§10）

- [ ] 8.1 历史数据兼容：quality_breakdown=NULL 的历史 section 接口不报错、前端降级显示
- [ ] 8.2 迁移幂等：重复执行 quality_breakdown 列迁移无副作用
- [ ] 8.3 GORM 字段变更向后兼容：quality_breakdown 为可空新增字段，JSON 响应格式不破坏既有消费方
- [ ] 8.4 回滚路径明确：DROP COLUMN quality_breakdown 可逆

## 9. 文档流转（§12，里程碑收尾）

> `docs/reference/` 在里程碑收尾统一更新，不在本 change 内逐条改活文档。本节列出待更新清单：

- [ ] 9.1（里程碑收尾）`docs/reference/database/`：补充 `daily_report_sections.quality_breakdown` 列说明
- [ ] 9.2（里程碑收尾）`docs/reference/api/`：补充 section 表示的 `quality_breakdown` 字段
- [ ] 9.3（里程碑收尾）`docs/reference/architecture/map.md`：MatchTier 质量等级映射登记到质量排序链路索引

## 10. 归档门禁（§11）

- [ ] 10.1 后端门禁：`cd backend-go && golangci-lint run ./... && go vet ./... && go test ./internal/topicgraph/... && go build ./...`（测试只跑影响包）
- [ ] 10.2 前端门禁：`cd front && pnpm lint`（WSL）+ `cmd.exe /C "cd /d D:\\project\\Syntopica\\front && pnpm exec nuxi typecheck"` + `pnpm test:unit` + `pnpm build`（typecheck/build 经 Windows cmd）
- [ ] 10.3 `openspec validate quality-scoring-observability` 通过
- [ ] 10.4 issue `docs/issues/01-quality-sort-blackbox.md` 状态更新（归档时标记 resolved）
