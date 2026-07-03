# Tasks: watch 双轨化（keyword）+ 即时匹配 + 快捷关注

> 垂直切片，每切片独立可交付、可验证。推荐顺序：A 后端双轨判定（核心纯逻辑先行）→ B keyword 即时匹配 → C 前端类型切换 + 快捷入口。尾部遵循开发执行规范 §11 归档门禁。
>
> ⚠️ 前置：topic-watchlist-observability 已归档（topic-watch 主 spec + board_topic_watches 表已存在）。

## 1. 后端：实体 type 字段 + 双轨命中判定（A · topic-watch）

- [ ] 1.1 版本化迁移：`board_topic_watches.type` 列（CHECK label/keyword，默认 label），幂等。验收：testcontainer 反复执行无错，历史行 type=label
- [ ] 1.2 模型 `BoardTopicWatch.Type` 字段 + 常量 `WatchTypeLabel`/`WatchTypeKeyword`。验收：模型对齐迁移
- [ ] 1.3 纯函数 `parseKeywordExpr(expr string) [][]string`：先拆 `|`（OR 组），再拆空格（AND 组）。验收：SQLite 单测覆盖（单词 / 多词AND / 多词OR / 混用 `ASML|镓锗 出口` / 空串降级）
- [ ] 1.4 纯函数 `matchKeywordSections(expr string, sections []SectionText) []KeywordHit`：拼接 threads title+summary，大小写不敏感，按 parseKeywordExpr 判定，返回命中 section + 命中词。验收：SQLite 单测覆盖（AND 全含命中 / AND 缺一不命中 / OR 任一命中 / 大小写 / 无 threads 降级）
- [ ] 1.5 `EvaluateWatchHits` 分叉：label 类收集走 AI 批量（现有逻辑不变）；keyword 类调 matchKeywordSections；两类命中合并 upsert 写表。验收：SQLite 单测覆盖分叉分支
- [ ] 1.6 零副作用集成测试（testcontainer）：断言①keyword 命中不改 section.persistent_topic_id ②keyword 命中不推进 topic 生命周期 ③两类命中复合唯一索引去重。验收：3 个集成测试 PASS
- [ ] 1.7 `CreateWatch(boardID, label, watchType)` 签名扩展（默认 label 保持向后兼容）；调用方（handler + 即时匹配）适配。验收：codegraph impact CreateWatch 无 HIGH 风险
- [ ] 1.8 handler `createTopicWatch` body 解析 type（缺省 label）。验收：handler 测试覆盖 type 传入/缺省

## 2. 后端：keyword 即时匹配（B · topic-watch，依赖 1）

- [ ] 2.1 `MatchKeywordInstant(boardID, watchID, sinceDays=14)`：拉近 14 天 section + threads 文本，调 matchKeywordSections，upsert 写 hits（OnConflict DoNothing）。验收：SQLite 单测覆盖
- [ ] 2.2 `CreateWatch` 当 type=keyword 时，建表后同步触发 MatchKeywordInstant；失败 log.Warnf 吞（关注仍建成功）。验收：单测覆盖即时触发 + 失败降级
- [ ] 2.3 集成测试：即时匹配命中历史 section + 与日报匹配幂等去重（OnConflict DoNothing）。验收：testcontainer PASS
- [ ] 2.4 label 类不触发即时匹配的断言。验收：单测覆盖

## 3. 前端：类型切换 + 两类展示（C · topic-watch，依赖 1）

- [ ] 3.1 `topicWatches.ts`：`createWatch` 加 type 参数；`TopicWatch` 接口加 type 字段；normalizer 适配。验收：API 封装测试 PASS
- [ ] 3.2 新建关注 `AppDialog` 加「类型」选择（关注话题 label / 关注关键字 keyword），label/关键字输入区随类型切换提示文案。验收：组件测试断言类型切换
- [ ] 3.3 `DailyReportWatchBar` 两类命中展示：keyword 分组 reason 显示「含关键字『XX』」+ 标签图标微区分；label 分组保持 AI 理由斜体。全语义 token。验收：组件测试断言两类 reason 文案 + 视觉区分 class
- [ ] 3.4 keyword 多词输入提示（空格=AND、|=OR）内联说明。验收：组件测试断言提示存在

## 4. 前端：内容流快捷关注入口（D · topic-watch，依赖 3）

- [ ] 4.1 section 详情旁「＋关注」入口：点击打开新建对话框，label 预填 cluster_label，type 默认 label。验收：组件测试断言预填
- [ ] 4.2 话题详情（生命线 / 泳道节点详情）旁「＋关注」入口：label 预填 topic.label。验收：组件测试断言预填
- [ ] 4.3 快捷入口复用 createWatch（与顶部栏同端点），切 keyword 提交触发即时匹配。验收：测试断言同一 API 调用
- [ ] 4.4 入口禁 `window.*`，复用 AppDialog/AppButton/AppInput。验收：grep 无 window.* 调用

## 5. 架构体检（§7 强制，每个子任务后）

- [ ] 5.1 `codegraph impact`：`CreateWatch`/`EvaluateWatchHits`/`matchKeywordSections` 三处波及面无 HIGH/CRITICAL 忽略
- [ ] 5.2 新增/改 handler grep 路由注册二次确认（codegraph 追不到 group.POST）。验收：端点路由注册确认
- [ ] 5.3 传导链守卫：keyword 命中是只读叠加，重跑 topic-watchlist-observability 的零副作用集成测试，确认归属/生命周期未被波及。验收：PASS
- [ ] 5.4 分层合规：keyword 匹配纯函数在 `internal/topicgraph/`（或 service），前端入口在 `features/tags/components/`，不引入循环依赖

## 6. 数据兼容性（§10）

- [ ] 6.1 迁移幂等：type 列 + CHECK 在 testcontainer 反复执行无错
- [ ] 6.2 历史 watch type 默认 label 不报错；行为不变（仍走 AI）
- [ ] 6.3 JSON 响应 type 为新增可选字段（默认 label），向后兼容
- [ ] 6.4 回滚路径：DROP type 列可逆；keyword 判定/即时匹配逻辑可独立 revert（label 类不受影响）

## 7. 文档（§12.4 里程碑收尾统一更新）

> 以下 reference 更新在**里程碑收尾时**统一做。触及 flow 的，archive 后按 §12.2 补「变更溯源」链接。

- [ ] 7.1 `docs/reference/api/`：createWatch 补 type 参数；说明 label/keyword 两类
- [ ] 7.2 `docs/reference/database/`：board_topic_watches 补 type 列
- [ ] 7.3 `docs/reference/flow/daily-report.md`：EvaluateWatchHits 补"分叉：label 走 AI / keyword 走文本" + keyword 即时匹配（建关注时触发，非生成流程）

## 8. 测试（§11.2）

> 归档前重跑，确认零失败。后端命令须走 cmd.exe；前端 typecheck/build/test 须 cmd，lint 可 WSL。

- [ ] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/topicgraph/handler -short"` → PASS
- [ ] T.2 testcontainer 集成：迁移幂等 + EvaluateWatchHits 双轨分叉 + keyword 即时匹配去重 + 零副作用 → PASS
- [ ] T.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全过（含 parseKeywordExpr/matchKeywordSections + 类型切换对话框 + 两类命中展示 + 快捷入口预填）
- [ ] T.4 `grep -rnE "window.(alert|prompt|confirm)" front/app` → 零命中

## 9. 验证（§11.2，归档前实测）

- [ ] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [ ] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/topicgraph/..."` → VET_OK
- [ ] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/..."` → 0 issues
- [ ] V.4 `cd front && pnpm lint` → 0 error（lint WSL 可跑）
- [ ] V.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [ ] V.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [ ] V.7 `bash scripts/check-standards.sh` → A-D 段零失败（E 段归档后校验）
- [ ] V.8 浏览器视觉验收（cmd 起后端 + 前端）：① 新建关注可切 label/keyword ② keyword 多词（空格AND/|OR）命中正确 ③ 建完 keyword 立刻看到历史命中（即时匹配）④ 顶部栏两类命中视觉区分（keyword「含关键字」+ 图标）⑤ section/话题详情「＋关注」预填 label 一键建
