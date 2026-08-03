## 1. 数据源层：web_search 真接通 + fetch_page

- [ ] 1.1 新增博查 HTTP client `BochaWebSearcher`（实现 `WebSearcher` 接口，调 `api.bochaai.com`，返回 `[]WebSearchResult`）于 `service/web_search.go`
- [ ] 1.2 配置注入：`configs/config.yaml` 增博查段 + 环境变量 `BOCHA_API_KEY` 读取（照 airouter provider pattern，单独 `search_config.go` 或并入 board_config）
- [ ] 1.3 `wire.go`：`BOCHA_API_KEY` 非空时注入 `BochaWebSearcher`，空时回退 `NoopWebSearcher`（条件注入，保留优雅降级）
- [ ] 1.4 单测 `web_search_test.go`：mock HTTP 测博查解析 + key 缺失回退 Noop 分支
- [ ] 1.5 新增 `fetch_page` 工具：`tool_registry.go` 注册，复用 `internal/reader/service/readability_crawler`，返回 `{title,url,main_text}`（超长截断）
- [ ] 1.6 注入 reader crawler 依赖到 Registry（新增 `WithPageFetcher` option 或类似），wire 连线
- [ ] 1.7 单测 fetch_page：mock crawler 测正文返回 + 超时/反爬失败返回错误 JSON 不阻断

## 2. 后端 prompt + schema 重写（深度层 + 去硬编码）

- [ ] 2.1 `orchestrator.go`：新增 `Depth` 结构体（`system_reframe`/`mechanism_layers`/`historical_analogy`/`regime_shift`/`boundary`/`evidence_chain`）+ 4 个 Analysis 结构体挂 `Depth` 字段
- [ ] 2.2 `parseAnalyzeOutput` 扩展：解析 depth 块；非 sparse 形态 depth 必填校验（缺/`boundary` 空则视为解析失败重试一次）；sparse 禁 depth
- [ ] 2.3 新增 `structural` 形态：`isValidForm` + `FormStructural` 常量 + `StructuralAnalysis` 结构体（结构演化叙述 + depth）+ 解析分支
- [ ] 2.4 重写 `interpretPrompt`：「结构化分析编辑」，提炼领域自适应研究方向（历史机制/关键数据/可比案例），删除"A 股 ETF 方向"硬编码
- [ ] 2.5 重写 `agentLoopSystemPrompt`：「研究助理/事实核查员」，工具集改为 web_search + fetch_page + 内部导航
- [ ] 2.6 重写 `analyzePrompt`：强制产出 depth 块 + 显式反过度解读（`boundary` 非空）+ 新增 structural 形态产出分支
- [ ] 2.7 重写 `lensProposePrompt`：视角示例改为结构/系统题（"X 为何反复发生""X 背后底层结构"）
- [ ] 2.8 单测 `orchestrator_test.go`：各形态（含 structural）depth 解析、boundary 非空、sparse 不产 depth、evidence_chain 含 web/page 类带 url

## 3. 金融方向一刀切删除 + 注册表/配置重做

- [ ] 3.1 `tool_registry.go` register()：删除 `list_etf_by_keyword`/`get_etf_quote`/`list_sectors` 三个工具注册
- [ ] 3.2 删除 `executeListETFByKeyword`/`executeGetETFQuote`/`executeListSectors`/`loadETFSpot` 及 ETF cache 字段（etfCacheMu/etfCache/etfCacheLoaded）
- [ ] 3.3 `repository/models.go`：删 `SourceTypeETFQuote`/`SourceTypeExchangeRate`/`SourceTypeGDELTEvent` 常量 + `validSourceTypes` 清空（保留 SourceType 类型 + CHECK 机制为扩展点）
- [ ] 3.4 `board_config.go`：`ToolsForSourceType` 重做——默认 always-on 集 = 内部导航 + web_search + fetch_page，不再有 per-source_type 条件工具
- [ ] 3.5 清理 `buildAgentAllowedTools`/`explorationToolNames` 等对金融工具的引用（`orchestrator.go:1103` explorationToolNames 加 fetch_page）
- [ ] 3.6 单测更新：`tool_registry_test.go`/`web_search_test.go` 等去掉 ETF 断言，新增 fetch_page 断言

## 4. 前端深度层渲染

- [ ] 4.1 `app/api/boardEnrichment.ts`：新增 `Depth`/`MechanismLayer`/`HistoricalAnalogy`/`EvidenceChainItem` 类型；analysis 接口加可选 `depth`
- [ ] 4.2 `CausalAnalysisReport.vue`：新增深度层渲染区块（系统重定位 / 多层机制 / 历史类比 / 范式转折 / 边界限定 / 证据链可点击 URL）
- [ ] 4.3 旧结果（无 depth 字段）降级渲染不崩；引用标记 📰新闻 / 🌐网页 / 📄正文
- [ ] 4.4 `BoardEnrichmentPanel.vue`：按需微调（深度层区块布局）

## 5. 测试

<!-- 本 change 影响包：internal/dataenrichment/{service,repository,handler}；前端 boardEnrichment 相关。golden case 需人工验收（深度质量不可自动判定） -->

- [ ] 5.1 后端单测（仅影响包）：`cd backend-go && go test ./internal/dataenrichment/...`
- [ ] 5.2 golden case 人工验收：用「内部看美国」真实话题（如人民币国际化）触发一次增强，人工核对深度层是否含系统重定位/多层机制/边界/可核查证据链
- [ ] 5.3 前端单测（Windows cmd）：深度层渲染 + 降级分支
- [ ] 5.4 review_judge 适配验证：用新 schema 跑一次认知对比，确认新旧深度层见解对比仍成立

## 6. 文档

<!-- doc-impact: data-enrichment-flow(数据增强分析主线从"A股产业点评"改为"结构化深度剖析";新增 web 源/深度层/structural 形态;移除金融方向与旧走向预测残留) + api(enrichment 产物 schema 加 depth 块、source_type 枚举精简) + configuration(博查 BOCHA_API_KEY) + database-docs(source_type 枚举精简、旧 etf source 行清理说明)。apply 启动以 doc-impact.sh suggest 实际预勾选为准；若 suggest 命中 flow/standard/api/configuration/database 则补对应文档更新 -->

- [ ] 6.1 apply 启动跑 `doc-impact.sh suggest` 预勾选 + `doc-impact.sh context` 注入 flow 业务约束
- [ ] 6.2 按 suggest 命中同步 `docs/reference/flow/`（数据增强 flow 变更溯源）、`docs/reference/api/`（enrichment schema）、`docs/reference/configuration.md`（博查 key）、`docs/reference/database/`（source_type 精简）
- [ ] 6.3 提供"部署后影响 + 需要的操作"汇报：用户配博查 key、旧 etf source 行清理 SQL、旧结果无 depth 降级行为

## 7. 验证

- [ ] 7.1 `cd backend-go && golangci-lint run ./...` → 期望：无 error/warning
- [ ] 7.2 `cd backend-go && go vet ./...` → 期望：无诊断
- [ ] 7.3 `cd backend-go && go build ./...` → 期望：编译通过
- [ ] 7.4 `cd backend-go && go test ./internal/dataenrichment/...` → 期望：全部 PASS
- [ ] 7.5 `cd front && pnpm lint` → 期望：无 error（WSL 可跑）
- [ ] 7.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 期望：无类型错误
- [ ] 7.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 期望：构建成功
- [ ] 7.8 归档前 `doc-impact.sh verify` + `check-standards.sh` 对账通过（开发执行规范 §11.4 / §0.6）
