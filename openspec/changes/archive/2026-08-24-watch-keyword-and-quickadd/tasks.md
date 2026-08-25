# Tasks: watch 双轨化（keyword）+ 即时匹配 + 版块级管理面板

> 垂直切片，每切片独立可交付、可验证。推荐顺序：A 后端双轨判定（核心纯逻辑先行）→ B keyword 即时匹配 → C 前端管理面板 + 两类展示。
>
> **基座现状（2026-08-24 核对）**：topic-watchlist-observability 已归档（2026-07-23），`board_topic_watches` 表 / `EvaluateWatchHits` / 前端 `DailyReportWatchBar` 均在线上代码，可直接开工。`topic-watch` 主 spec 基线已补回 `openspec/specs/topic-watch/`。
>
> **入口决策（2026-08-24 用户确认）**：创建/管理唯一入口 = 版块工作台 tab 栏（TagsPage `tags-content-tabs`）右端「我在追踪 (N)」chip（五 tab 常驻，日报与话题总览是平级 tab）→ 管理面板；日报详情栏只读 + 栏头跳转；**不做**内容流快捷入口（与文章页标签关注撞心智）。
>
> **测试设计锚 `test-cases.md`**（change 目录内）：故事 S1-S4 为主链路，变体走查 / 继承与调整 / 白盒附加都在该文档，本文件的验收措辞与其对齐。

## 1. 后端：实体 type 字段 + 双轨命中判定（A · topic-watch · 故事 S1/S4）

- [x] 1.1 版本化迁移：`board_topic_watches.type` 列（CHECK label/keyword，默认 label），幂等。验收：`backend-go/internal/platform/database/watch_type_column_test.go`（新增，testcontainer）断言反复执行无错 + 历史行 type=label
- [x] 1.2 模型 `BoardTopicWatch.Type` 字段 + 常量 `WatchTypeLabel`/`WatchTypeKeyword`。验收：模型列定义与迁移 SQL 对照一致（CHECK/默认值逐项 grep 核对）
- [x] 1.3 纯函数 `parseKeywordExpr(expr string) [][]string`：先拆 `|`（OR 组），再拆空格（AND 组）。验收：`backend-go/internal/topicgraph/service/keyword_match_unit_test.go`（新增，无 DB）覆盖 test-cases.md 变体表 V1-V8：空串 / 纯空白（含全角、tab）/ 纯分隔符（单个/连续/开头/结尾）/ 单词 / 多词AND / 多词OR / 混用 `ASML|镓锗 出口` / 大小写
- [x] 1.4 纯函数 `matchKeywordSections(expr string, sections []SectionText) []KeywordHit`：拼接 threads title+summary，大小写不敏感，按 parseKeywordExpr 判定，返回命中 section + 命中词。验收：`keyword_match_unit_test.go` 覆盖：AND 全含命中 / AND 缺一不命中 / OR 任一命中 / 大小写等价 / section 无 threads 降级（不命中）
- [x] 1.5 `EvaluateWatchHits` 分叉：label 类收集走 AI 批量（现有逻辑不变）；keyword 类调 matchKeywordSections；两类命中合并 upsert 写表。验收：`daily_report_watch_test.go` 覆盖分叉两分支，keyword 分支断言 chat 调用次数=0（不调 AI）
- [x] 1.6 零副作用集成测试（testcontainer）：断言①keyword 命中不改 section.persistent_topic_id ②keyword 命中不推进 topic 生命周期（consecutive_hits 不变）③两类命中复合唯一索引去重。验收：`daily_report_watch_integration_test.go` 3 个用例 PASS
- [x] 1.7 `CreateWatch(boardID, label, watchType)` 签名扩展（默认 label 保持向后兼容）；调用方（handler + 即时匹配）适配。验收：`codegraph impact CreateWatch` 波及面无 HIGH/CRITICAL 被忽略
- [x] 1.8 handler `createTopicWatch` body 解析 type（缺省 label）+ keyword 表达式有效性校验（解析后无有效词组 → 400）。验收：`topic_watch_handler_test.go` 覆盖 type 传入 / 缺省 / 无效表达式拒绝（`"ASML|"`、纯空白）

## 2. 后端：keyword 即时匹配（B · topic-watch · 故事 S2，依赖 1）

- [x] 2.1 `MatchKeywordInstant(boardID, watchID, sinceDays=14)`：拉近 14 天 section + threads 文本，调 matchKeywordSections，upsert 写 hits（OnConflict DoNothing），返回命中数供前端反馈。验收：`daily_report_watch_test.go` 覆盖窗口边界（第 14 天当天含、第 15 天不含）
- [x] 2.2 `CreateWatch` 当 type=keyword 时，建表后同步触发 MatchKeywordInstant；失败 log.Warnf 吞（关注仍建成功）。验收：`daily_report_watch_test.go` 覆盖即时触发 + 匹配报错时 watch 行仍存在
- [x] 2.3 集成测试（testcontainer）：即时匹配命中历史 section + 与日报匹配幂等去重（同 (watch_id, section_id, report_id) 只一行）。验收：`daily_report_watch_integration_test.go` PASS
- [x] 2.4 label 类不触发即时匹配的断言（CreateWatch type=label 时 MatchKeywordInstant 零调用）。验收：`daily_report_watch_test.go` PASS

## 3. 前端：管理面板 + 类型切换对话框 + 两类展示（C · topic-watch · 故事 S1/S3，依赖 1）

- [x] 3.1 `topicWatches.ts`：`createWatch` 加 type 参数；`TopicWatch` 接口加 type 字段；normalizer 适配。验收：`front/app/api/topicWatches.test.ts` PASS（type 透传 + 缺省不传）
- [x] 3.2 抽取新建关注对话框为独立组件 `TopicWatchCreateDialog.vue`（现状内嵌于 `DailyReportWatchBar.vue`）：类型双选（关注话题 label / 关注关键字 keyword）；keyword 态含语法提示（空格=AND、|=OR）+ 实时解析预览（chips 化表达式，无效红字提示并禁用提交）。验收：`TopicWatchCreateDialog.test.ts` 断言类型切换 + 解析预览 chips + 无效表达式禁提交 + 空 label 误输入反馈（提示可见、已输内容保留）
- [x] 3.3 新组件 `WatchManagePanel.vue`（含入口 chip）：入口「我在追踪 (N)」挂 TagsPage `tags-content-tabs` 右端（五 tab 常驻，日报/话题总览是平级 tab）；面板内列出该版块全部关注（label/keyword 类型标识、active/paused 状态）、暂停/恢复、删除（二次确认）、新建入口（开 `TopicWatchCreateDialog`）、keyword 建后即时回扫反馈（命中数可点查看）。验收：`WatchManagePanel.test.ts` 断言列表渲染 + 暂停/删除调用 API + 回扫反馈展示；`TagsPage.test.ts`（新增）断言 chip 在 tab 栏右端挂载 + N 计数（active+paused）
- [x] 3.4 `DailyReportWatchBar` 只读化：两类命中展示（keyword reason=「含关键字『XX』」+ 标签图标微区分；label AI 理由斜体，全语义 token）；栏头改为「你在追踪 · N 个关注」跳转管理面板；**移除栏内新建按钮与内嵌对话框**。验收：`DailyReportWatchBar.test.ts` 断言两类 reason 文案 + 区分 class + 栏头跳转事件 + 栏内无新建入口
- [x] 3.5 本 change 的 watch 组件禁 `window.*`，复用 AppDialog/AppButton/AppInput。验收：`grep -rnE "window.(alert|prompt|confirm)" front/app/features/tags/components/topic-watch front/app/features/tags/components/daily-report/DailyReportWatchBar.vue` 零命中（`front/app/composables/useGlobalSettings.ts` 的既有原生确认框不属本 change，不扩大范围修复）

## 3A. 日报追踪展示重构（用户确认：时间线预告 + 详情优先索引）

- [x] 3A.1 日报列表 API：`ListReports` 以一次批量 JOIN 回填每期 `active_watch_summaries[]`（watch_id / label / type）；同 watch 命中多个 section 只出现一次，paused/deleted watch 不返回，保持 period_date DESC。验收：repository 测试覆盖批量回填 / 去重 / paused 排除。
- [x] 3A.2 命中详情读取只返回 active watch，并返回 watch label/type，供时间线 tag 与详情索引共用；删除/暂停后重新读取不得返回陈旧 hit。验收：handler/repository 测试 PASS。
- [x] 3A.3 `BoardDailyReportTimeline`：保持日期顺序，每条日报记录下显示至多两个紧凑 `# keyword` / `✦ topic` tag，余项 `+N`；点击 tag 打开对应日报并定位 section；不得逐报告 N+1 请求。验收：`BoardDailyReportTimeline.test.ts` 断言摘要渲染、+N、单次列表请求、定位事件。
- [x] 3A.4 以 `DailyReportWatchIndex` 替换独立 `DailyReportWatchBar`：在 `.drm-content` 内、与「关心的话题」同一正文列且位于其之前，依次插入全宽「追踪关键字」「追踪话题」分区；仅渲染可点击单行索引，定位真实 section，命中为空则隐藏，不复制正文或常驻 reason，不用大面积绿色。验收：`DailyReportWatchIndex.test.ts` + `BoardDailyReportTimeline.test.ts` 覆盖分类、定位锚点、无 reason/正文和同列 DOM 层级。
- [x] 3A.5 `TagsPage` 的「我在追踪 (N)」入口位置和行为不变；图标/文字禁止换行，补固定 gap 与 padding 呼吸感。验收：`TagsPage.test.ts` 断言 nowrap class/样式契约。
- [x] 3A.6 删除已废弃的 `DailyReportWatchBar` 与其测试/分组辅助代码，更新 imports 与 Scenario 映射；不得保留死组件或双重展示。验收：`rg "DailyReportWatchBar|topicWatchGrouping" front/app` 零命中。

## 4. 架构体检（§7 强制，每个子任务后）

- [x] 4.1 `codegraph impact`：`CreateWatch`/`EvaluateWatchHits`/`matchKeywordSections` 三处波及面无 HIGH/CRITICAL 忽略
- [x] 4.2 新增/改 handler grep 路由注册二次确认（codegraph 追不到 group.POST）。验收：`grep -n "topic-watches" backend-go/internal/topicgraph/handler/topic_watch_handler.go` 路由注册确认
- [x] 4.3 传导链守卫：keyword 命中是只读叠加，重跑零副作用集成测试（1.6），确认归属/生命周期未被波及。验收：PASS
- [x] 4.4 分层合规：keyword 匹配纯函数在 `internal/topicgraph/service`，前端面板/对话框在 `features/tags/components/`（入口挂 TagsPage，不依赖 BoardDailyReportTimeline 内部遗留的 showThreadBrowser 切换），不引入循环依赖

## 5. 数据兼容性（§10）

- [x] 5.1 迁移幂等：type 列 + CHECK 在 testcontainer 反复执行无错
- [x] 5.2 历史 watch type 默认 label 不报错；行为不变（仍走 AI）
- [x] 5.3 JSON 响应 type 为新增可选字段（默认 label），向后兼容
- [x] 5.4 回滚路径：DROP type 列可逆；keyword 判定/即时匹配逻辑可独立 revert（label 类不受影响）

## 6. 测试（§11.2）

> 后端命令须走 cmd.exe；前端 typecheck/build/test 须 cmd，lint 可 WSL。归档前重跑，确认零失败。

- [x] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/... ./internal/platform/database -short"` → PASS（纯函数 + 服务层 + handler）
- [x] T.2 testcontainer 集成：`go test ./internal/topicgraph/service -run Integration` + `go test ./internal/platform/database -run WatchType`（全量非 -short，分两命令避免 `-run` 串包歧义）→ 迁移幂等 + 双轨分叉 + 即时匹配去重 + 零副作用 PASS
- [x] T.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → PASS（keyword 表达式解析 + 类型切换对话框 + 管理面板 + 两类命中展示）
- [x] T.4 `grep -rnE "window.(alert|prompt|confirm)" front/app/features/tags/components/topic-watch front/app/features/tags/components/daily-report/DailyReportWatchBar.vue` → 零命中（全局 `useGlobalSettings.ts` 的既有原生确认框不属本 change）
- [x] T.5 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/repository ./internal/topicgraph/handler -short"` → active 摘要、暂停过滤、API 契约 PASS
- [x] T.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit -- BoardDailyReportTimeline DailyReportWatchIndex DailyReportTopicSection TagsPage"` → 时间线预告、详情定位、入口 nowrap PASS

## 7. 文档（§12.4 里程碑收尾统一更新）

<!-- doc-impact: api database flow -->
<!-- doc-impact-excuse: standard=命中 docs/reference/standard/shared/test-design.md 属并行 test-case-design-standard change；本 change 未修改 standard 文档 -->

- [x] 7.1 `docs/reference/api/`：createWatch 补 type 参数；label/keyword 两类说明；keyword 表达式语法（空格 AND / `|` OR）与 400 校验
- [x] 7.2 `docs/reference/database/`：board_topic_watches 补 type 列
- [x] 7.3 `docs/reference/flow/daily-report.md`：EvaluateWatchHits 补「分叉：label 走 AI / keyword 走文本」+ keyword 即时匹配（建关注时触发，非生成流程）+ 关注管理入口（TagsPage tab 栏，版块级；日报栏只读化）。触及 flow，archive 后按 §12.2 补「变更溯源」链接
- [x] 7.4 更新 `docs/reference/api/semantic-boards.md` 与 `docs/reference/flow/daily-report.md`：日报列表 active watch 摘要、时间线预告与详情优先索引规则；删除“独立日报栏”描述。

## 8. 验证（§11.2，归档前实测）

- [x] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [x] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/topicgraph/..."` → VET_OK
- [x] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/..."` → 0 issues
- [x] V.4 `cd front && pnpm lint` → 0 error（lint WSL 可跑）
- [x] V.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [x] V.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [x] V.7 `bash scripts/check-standards.sh` → A-D 段零失败（E 段归档后校验；F 段仅 `test-case-design-standard` 并发 change 未过，本 change doc-impact 已通过）
- [x] V.8 交互验收（ui-verify skill + §5.3 工具分流，opencli 驱动真实 Chrome）：① 五个 tab 下「我在追踪 (N)」chip 均可见、N 计数正确 ② 面板内新建可切 label/keyword、解析预览正确 ③ keyword 多词（空格AND/|OR）命中正确 ④ 建完 keyword 立刻看到回扫反馈与历史命中 ⑤ 日报栏两类命中视觉区分（keyword「含关键字」+ 图标；视觉差异截图派 Luna 子代理）⑥ 日报栏头跳管理面板、栏内无新建。留痕：opencli 会话记录 + 截图存 change 目录
  - 2026-08-24 Luna（opencli session `v8`）第一轮实测：①②③④⑥ PASS，keyword 回扫命中 3 条后清理。
- [x] V.9 交互/视觉验收：时间线不改日期顺序、记录下命中 tag 可见且可定位；详情「追踪关键字 / 追踪话题」位于「关心的话题」之前、无独立窄栏/绿色大卡；管理 chip 不换行。留痕：opencli + Luna 截图存 `evidence/`。
  - 2026-08-24 用户人工验收：索引移入 `.drm-content` 正文列、与「关心的话题」同级后通过；无需重复 Luna 复核。
  - 2026-08-24 Luna 补测（用户授权临时创建 label/keyword 并重建日报）：board 1974 的 2026-08-24 日报生成完成；同一 WatchBar 同时显示 label 的斜体 AI 理由和 keyword 的「含关键字『霍尔木兹海峡、封锁』」+ # 绿色机械样式。临时 watch `id=2/3` 已删除，页面与 IPv6 API 均为 0 条。证据：`evidence/v8-daily-watchbar-dual.png`、`evidence/v8-dual-cleanup.png`。

### Scenario 映射表（scenario-trace.sh 归档对账，标题须与 delta specs 逐字一致）

| Scenario | 测试文件 |
| --- | --- |
| 创建关注标记 | backend-go/internal/topicgraph/handler/topic_watch_handler_test.go |
| 创建 keyword 类关注 | backend-go/internal/topicgraph/handler/topic_watch_handler_test.go |
| 状态约束 | backend-go/internal/topicgraph/repository/topic_watch_repository_test.go |
| 类型约束 | backend-go/internal/topicgraph/repository/topic_watch_repository_test.go |
| 历史 watch 默认类型 | backend-go/internal/platform/database/watch_type_column_test.go |
| 时间线命中预告 | front/app/features/tags/components/BoardDailyReportTimeline.test.ts |
| 详情优先索引与定位 | front/app/features/tags/components/daily-report/DailyReportWatchIndex.test.ts |
| keyword 命中无 AI 理由 | front/app/features/tags/components/daily-report/DailyReportWatchIndex.test.ts |
| 无命中隐藏 | front/app/features/tags/components/daily-report/DailyReportWatchIndex.test.ts |
| 创建 keyword 关注 | backend-go/internal/topicgraph/handler/topic_watch_handler_test.go |
| type 缺省为 label | backend-go/internal/topicgraph/handler/topic_watch_handler_test.go |
| 无效关键字表达式被拒绝 | backend-go/internal/topicgraph/handler/topic_watch_handler_test.go |
| 暂停的关注不参与判定 | backend-go/internal/topicgraph/service/daily_report_watch_test.go |
| 删除关注级联清理命中 | backend-go/internal/platform/database/watch_hit_fk_cascade_test.go |
| label 类走 AI 单信号 | backend-go/internal/topicgraph/service/daily_report_watch_test.go |
| keyword 类走文本匹配 | backend-go/internal/topicgraph/service/daily_report_watch_test.go |
| keyword 多词 AND 逻辑 | backend-go/internal/topicgraph/service/keyword_match_unit_test.go |
| keyword 多词 OR 逻辑 | backend-go/internal/topicgraph/service/keyword_match_unit_test.go |
| keyword 大小写不敏感 | backend-go/internal/topicgraph/service/keyword_match_unit_test.go |
| 两类命中合并写表 | backend-go/internal/topicgraph/service/daily_report_watch_integration_test.go |
| 建关注后立即命中历史 section | backend-go/internal/topicgraph/service/daily_report_watch_integration_test.go |
| 即时与日报匹配幂等去重 | backend-go/internal/topicgraph/service/daily_report_watch_integration_test.go |
| 即时匹配失败不阻断建关注 | backend-go/internal/topicgraph/service/daily_report_watch_test.go |
| label 类不即时匹配 | backend-go/internal/topicgraph/service/daily_report_watch_test.go |
| 入口常驻版块内容区 | front/app/features/tags/components/TagsPage.test.ts |
| 从管理面板新建关键字关注 | front/app/features/tags/components/topic-watch/WatchManagePanel.test.ts |
| 管理面板内暂停或删除关注 | front/app/features/tags/components/topic-watch/WatchManagePanel.test.ts |
