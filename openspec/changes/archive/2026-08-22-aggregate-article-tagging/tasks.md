## 1. 数据模型与迁移

- [x] 1.1 `models/article.go` 的 Article 结构体加 `ContentForm string` 字段（gorm tag 可空 varchar），确认 `Omit`/`Select` 列清单不需要连带调整（grep `ai_content_summary` 出现的投影点）
- [x] 1.2 新增数据库迁移：`articles` 加 `content_form varchar(20)` 可空列；跑迁移验证 `\d articles` 列存在且旧行为空值
- [x] 1.3 后端全量编译 + 既有 article 相关包测试（`go build ./...` + `go test ./internal/models/...`）

## 2. 摘要产出形态标记

- [x] 2.1 定位 `content_completion_service.go` 的 system prompt 来源（`s.aiService.GetSystemPrompt("zh")`），在 prompt 中加入形态判定要求与 `<!-- form: mono|aggregate -->` 首行输出格式说明（含聚合/单主题判定标准：栏目间主题异构 vs 单一主题多章节）
- [x] 2.2 实现 `parseContentFormMark(summary string) (form string, cleaned string)`：正则匹配首行 HTML 注释，匹配成功返回标记值与剥离后正文，匹配失败返回空标记与原文；处理首行前后空白
- [x] 2.3 `summarizeContent` 成功路径接入：AIContentSummary 存剥离后正文，article.ContentForm 存标记值；fallback 路径（aiService.SummarizeArticle）同样接入解析
- [x] 2.4 单元测试：parseContentFormMark 覆盖 mono/aggregate/无标记/注释在中间不在首行/空串；summarize 存储路径 mock 验证列值
- [x] 2.5 `go test ./internal/reader/...` + 手动触发一篇阮一峰新文章摘要，DB 验证 content_form='aggregate' 且摘要无注释残留

## 3. 切片器（纯代码）

- [x] 3.1 新增切片器函数（tagmanagement/service/core，如 `section_splitter.go`）：输入 markdown 摘要，按 `## ` 切栏目；跳过标题匹配"导读"的片；<300 runes 片向后合并；>8 片时超长栏目按 `### ` 细分；仍 >8 从尾部合并压回 8；每片保留栏目标题作上下文头
- [x] 3.2 切片器单测：用真实周刊摘要样本（5 栏目→4 片）、无 `##` 标题（返回空→调用方回落 mono）、全短栏目、>8 栏目、单栏目超长含 `###` 各写一 case
- [x] 3.3 `go test ./internal/tagmanagement/...`

## 4. 融合 prompt 与逐片提取

- [x] 4.1 新增融合提取 prompt + JSON schema（合并现有 event/person 与 keyword 两个 prompt 的规则与正反例，加"这是文章的一个栏目片"上下文说明，每片上限 4 个标签）；解析复用 `parseRawTagObjects`/`parseAuxiliaryLabels`
- [x] 4.2 实现逐片提取函数：每片 1 次 `router.Chat`（Capability 沿用 CapabilityTopicTaging，operation 用新值如 `tag_extraction_section`），重试 3 次（对齐现有 maxRetries），单片失败记 warning 跳过
- [x] 4.3 融合路径单测：mock router 返回合法/非法 JSON/超量标签，验证解析、上限截断、失败跳过

## 5. 聚合编排与入库

- [x] 5.1 `article_tagger.go` 加 `tagAggregateArticle`：切片→逐片提取→跨片 Slugify 去重（保留首栏目出现者）→文章级上限 15→score 分层（首个正文栏目 0.9/中间 0.7/尾部 0.5）→复用 findOrCreateTag/aux labels/createArticleTopicTagLink/event embedding enqueue 链路
- [x] 5.2 `tagArticle` 入口按 `article.ContentForm` 分流：`aggregate` 走 5.1，其余走原路径；切片器返回空片时回落原路径
- [x] 5.3 mono 路径参数调整：`maxSummaryRunesForTagging` 2000→4000，`maxArticleTags` 5→6
- [x] 5.4 编排单测：mock 分流两路径、去重保留首栏目、score 分层、单片失败其余片入库；更新受影响的既有测试（原 2000/5 断言）
- [x] 5.5 `go test ./internal/tagmanagement/...` 全绿

## 6. 端到端验证

- [x] 6.1 本地起服务，等/触发一篇阮一峰新文章走完 firecrawl→摘要→打标全链路，DB 验证：content_form='aggregate'、标签 10-15 个、score 呈 0.9/0.7/0.5 分层、无捏造合并标签、event 标签已 enqueue embedding
- [x] 6.2 对比验证：找一篇单主题新文章（博客园普通文章），验证走 mono 路径、content_form='mono'、标签 ≤6
- [x] 6.3 存量回归：挑一篇 content_form 为空的存量文章手动 RetagArticle，确认走 mono 路径不报错

## 7. 门禁与文档

- [x] 7.1 后端门禁：`golangci-lint run ./...` + `go vet ./...` + `go build ./...` + `go test ./internal/tagmanagement/... ./internal/reader/...`
- [x] 7.2 `doc-impact.sh verify` + `check-standards.sh`，按输出补 `docs/reference/flow/` 相应流程文档（打标链路新增分流分支、摘要链路新增形态标记）
- [x] 7.3 完工汇报：含部署后影响（新文章自动分流、聚合 feed 标签覆盖变全、存量不变）、用户需执行的操作（无）、旧数据降级行为（空 content_form 走 mono）

## 文档

<!-- doc-impact: flow database -->
<!-- doc-impact-excuse: api=ai_handler/discovery_handler 等改动来自并行进行中的其他 change，本 change 未碰 handler/api 层; architecture=runtime.go/pause.go 等改动来自并行 change，本 change 不涉架构; configuration=config.yaml 改动来自并行 change，本 change 零配置新增 -->
