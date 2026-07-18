# Tasks

> 算法主线依赖持久化与去重闸先行（防新 dup、防即算即弃）。TDD：每节先写失败测试再实现。

## 1. 数据迁移（schema + 一次性 dup 合并）

- [x] 1.1 新增迁移：创建 `board_upgrade_suggestions` 表（字段见 design.md D3；`pending` 状态哈希唯一约束：mode+decision+target_board_id+sorted_aux_ids）+ `suggestion_dismissals` 冷却记录（或表内 status=dismissed 兼任，实现时二选一并在 design 补注）
- [x] 1.2 新增一次性幂等迁移：aux 文本变体合并（归一化键分组，键与创建闸同一函数 → 主=ref_count 最大 → 复用 MergeAuxiliaryLabelAlias 合并 → 明细日志；语义对仅列报告；迁移末尾 packageBoardCache.InvalidateBoardData()）。验收：testcontainer 重跑两次无副作用
  - **§1.2 留痕**：迁移层因不依赖 service/embedder，独立实现 `mergeOneAuxLabelDup`（postgres_migrations.go）。经与 `service.MergeAuxiliaryLabelAlias`（auxiliary_label_service.go）逐行对比：aliases 去重合并、topic_tag_semantic_labels 改指（ON CONFLICT DO NOTHING）、source status=disabled、target ref_count 重算为 topic_tag 引用数——四步等价；且迁移层额外处理 board_composition 改指（service 方法未覆盖，迁移补齐）。归一化键抽取为无依赖 leaf 包 `textutil.NormalizeLabelKey`（core 反向依赖 database，致 database 不能 import core，故提取到 leaf 包满足 spec「同一实现」）。testcontainer 验证见 `internal/platform/database/dup_merge_test.go`（迁移幂等重跑 + dup 合并正确性）。
- [ ] 1.3 迁移演练：对生产库 `pg_dump` 相关表 → 本地跑迁移 → 核验 184 组收敛、SK海力士类 ref_count 合并正确、board_composition 无残留从标签引用

## 2. aux 创建去重闸（spec: auxiliary-label）

- [x] 2.1 失败测试：归一化查重键（"SK 海力士" 命中 "SK海力士" 复用不新建；无命中新建）→ 在 ResolveAuxiliaryLabel 现有 L1(slug+alias) 旁增加"去空白+lower"归一化键匹配（唯一创建入口，无需另覆盖 keyword/手动路径）
- [x] 2.2 失败测试：L2 阈值可配（默认 0.95 命中转 alias；改 ai_settings 值生效；embedding 失败降级新建 + 告警）→ 将 const auxiliaryLabelMergeThreshold 提升为 ai_settings `auxiliary_label_dedupe_sim`（复用现有 merge_embedding 列）
- [x] 2.3 计时日志：闸门耗时 >50ms 告警

## 3. 建议持久化与生命周期（spec: board-upgrade-suggestions）

- [x] 3.1 失败测试：建议落库（非 skip 写入 pending；skip 不写）→ 实现 repository + model
- [x] 3.2 失败测试：生成幂等（同哈希 pending 已存在则跳过）→ 实现唯一约束 + ON CONFLICT DO NOTHING
- [x] 3.3 失败测试：dismissed 冷却（冷却期内同哈希不再生成；期满允许重生）→ 实现冷却判定（`semantic_board_upgrade_suggestion_dismiss_cooldown_days` 默认 14 天）
- [x] 3.4 失败测试：confirm 联动（upgrade-execute 事务内置 confirmed，失败回滚状态不变）→ 实现事务内状态推进
- [x] 3.5 失败测试：watch 建议成簇自动关闭 → 实现成簇时关闭对应 watch

> **§3 留痕（实现澄清）**：
>
> - repository 放置：`internal/tagmanagement/repository/board_upgrade_suggestion.go`（独立文件 `BoardUpgradeSuggestionRepository`，与 `tag_job_queue.go` 同级惯例）。service 编排入口在同包 `service/board/board_upgrade_suggestion_persist.go`。
> - `CloseWatchSuggestions` 返回 `(int64, error)` 而非 tasks 原述的 `error`——计数供阶段 3 成簇逻辑记日志（“关闭 N 条 watch”），调用方可忽略返回值，向前兼容。
> - `skip 不落库` 的语义在 `GenerateAndPersist` 编排层实现（skip 决策在循环内 `continue`）；repository `InsertPending` 本身不感知决策，被调用即落库。
> - `CloseWatchSuggestions` 的 jsonb 包含查询用 `auxiliary_label_ids @> jsonb_build_array(?::int)`（逐 id OR），参数需显式 `::int` 类型转换，否则 PG 报 `could not determine data type of parameter`（42P18）。
> - **预存在失败（非本阶段引入）**：board 包 9 个既有测试（Backfill×4 / Matching×2 / Cluster×2 / CoTag×1）因早期迁移 `postgres_migrations.go:270` 已 seed `semantic_board_upgrade_cotag_hard_limit` 等 ai_settings key，而这些测试用 `db.Create` 插同 key 撞 `uni_ai_settings_key` 唯一约束。已验证：移除全部本阶段改动后原始代码同样失败，失败集完全一致。本阶段 6 个新测试全绿，新增 0 回归。修复属另一笔账（改这些测试为 FirstOrCreate/OnConflict），不在 §3 范围。

## 4. 扩展决策空间与双签名（spec: board-upgrade）

- [ ] 4.1 失败测试：discover_new 决策空间含 merge_into_existing；target_board_id 不在 shortlist 降级 skip → 改 `filterSemanticBoardUpgradeSuggestions` + `buildSemanticBoardUpgradePrompt`(user prompt) + `airouterSemanticBoardUpgradeLLM` system prompt schema（三处目前都禁 merge，system prompt schema 硬编码 create_new|skip 不随 mode 变，需改）
- [ ] 4.2 失败测试：双签名 shortlist（composition top-2 + 泳道 top-2 去重 ≤4；无泳道版块降级）→ 实现泳道签名（active topic 近 30 天 section embedding min-distance）
- [ ] 4.3 失败测试：高置信免 LLM（双签名 top-1 一致且**两签名各自** margin≥0.05 → confidence=high 不调 LLM；分歧或任一 margin 不足 → LLM）→ 实现 margin 闸门（`semantic_board_upgrade_merge_confidence_margin` 可配）
- [ ] 4.4 失败测试：prompt 注入候选版块泳道近期内容（至多 5 条 section 标题；拉取失败降级名称+描述）→ 实现证据注入
- [ ] 4.5 失败测试：单标签簇不进 LLM 写 watch → 实现观察池分流

## 5. API 与调度

- [x] 5.1 `GET /api/semantic-boards/upgrade-suggestions`（status/decision 过滤，默认 pending 非 watch，high 优先排序）
- [x] 5.2 `POST /api/semantic-boards/upgrade-suggestions/:id/dismiss`
- [x] 5.3 `POST /api/semantic-boards/upgrade-suggestions/generate` 同步生成入表返回新增数（替代旧 `POST upgrade-suggest`，旧路由保留兼容期）；`upgrade-execute` 请求体加 `suggestion_id` 联动 confirm
- [x] 5.4 注册 `job_board_upgrade_suggest`（默认每日 06:30 固定时间点、松耦合不保证紧随日报、仅 discover_new、失败仅记日志）+ 单测 job 接线
- [x] 5.5 watch 建议 GC：创建满 `semantic_board_upgrade_watch_gc_days`（默认 30 天）未成簇自动 dismissed

## 6. 前端建议面板

- [x] 6.1 建议列表改读 `upgrade-suggestions`（决策过滤：全部/merge/create_new；置信度标识）
- [x] 6.2 merge 建议卡：目标版块 + 证据展示（泳道 section 标题 + co-tag 事件）+ dismiss 按钮
- [x] 6.3 保留手动"合并到..."下拉（现状）与 expand_existing 入口，不动
- [x] 6.4 `front/app/api/` 新接口封装（/upgrade-suggestions 列表、/generate、/:id/dismiss；upgrade-execute 带 suggestion_id）+ 单测

## 7. Review 与数据兼容性

- [ ] 7.1 人工聚焦 review（§0.6 步骤4）：精读 raw SQL（shortlist/迁移/冷却查询）参数化、事务边界（confirm 联动）、迁移幂等
- [ ] 7.2 grep 不变量：`grep -rn 'merge_into_existing' backend-go/internal/tagmanagement/` 确认 discover_new 路径 merge 不再被 filter 丢弃；`grep -rn 'board_upgrade_suggestions' backend-go/` 确认仅 repository 写表
- [ ] 7.3 迁移前后对照：受影响版块 composition 数量变化入迁移日志；日报匹配回归（跑一次 rebuild 匹配对比前后 hit 分布无异常）

## 8. 测试（§11.2）

> 归档前重跑，零失败。后端 Go 命令走 cmd.exe；前端 typecheck/build/test 走 cmd.exe，lint 可 WSL。

- [ ] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/tagmanagement/... -short"` → PASS
- [ ] T.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/admin/scheduler -short"` → PASS
- [ ] T.3 testcontainer 集成：迁移幂等（2 次重跑）+ dup 合并正确性 + 建议生命周期（幂等/冷却/confirm 联动/watch 关闭）→ PASS
- [ ] T.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → PASS（含建议面板新用例）

## 9. 文档

> archive 前完成；触及 flow 的，archive 后按 §12.2 补「变更溯源」链接。

- [ ] 9.1 `docs/reference/flow/semantic-board.md`：升级建议节重写（持久化生命周期 + 双签名 + 高置信免 LLM + 观察池 + scheduler job）
- [ ] 9.2 `docs/reference/api/`：补 upgrade-suggestions 查询/dismiss 端点 + upgrade-suggest 语义变更
- [ ] 9.3 `docs/reference/database/`：补 board_upgrade_suggestions 表 + dup 合并迁移记录
- [ ] 9.4 `docs/reference/flow/scheduler.md`：补 job_board_upgrade_suggest

## 10. 验证（§11.2，归档前实测）

- [ ] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [ ] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/tagmanagement/... ./internal/admin/scheduler"` → VET_OK
- [ ] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/tagmanagement/... ./internal/admin/scheduler"` → 0 issues
- [ ] V.4 `cd front && pnpm lint` → 0 error
- [ ] V.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [ ] V.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [ ] V.7 `bash scripts/check-standards.sh` → A-D 段零失败
- [ ] V.8 重跑 `verification/upgrade_verify.sql` V3a → 文本变体重复组数从 184 降为 0（迁移生效）
- [ ] V.9 功能验收（cmd 起后端+前端）：① scheduler 或手动触发后建议入表可见 ② DeepSeek/Agent 类簇产出 merge 建议且目标为「生成式 AI 与大模型厂商」③ dismiss 后冷却期内不再出现 ④ 单标签簇只在观察池可见
