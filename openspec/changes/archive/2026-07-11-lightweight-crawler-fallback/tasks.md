# Tasks — lightweight-crawler-fallback

> 执行纪律：遵循 `docs/reference/开发执行规范.md` §2 TDD（先红后绿）、§4 后端门禁、§10 数据兼容性。
> 主线程只调度，实现派发子线程（§0.6）。

## 1. 依赖准备

- [x] 1.1 在 `backend-go/` 执行 `go get github.com/go-shiori/go-readability@latest github.com/JohannesKaufmann/html-to-markdown@latest`，`go mod tidy`
- [x] 1.2 确认 `go build ./...` 通过（传递依赖 x/net、goquery 版本升级无破坏）

## 2. Crawler 接口与中立 ScrapeResult（TDD）

- [x] 2.1 写测试 `crawler_test.go`：定义 `Crawler` 接口契约测试——给定一个实现 `ScrapePage(ctx,url)→(*ScrapeResult,error)` 的 stub，验证可赋值给 `Crawler` 接口；验证 `ScrapeResult` 字段（Markdown/Title/OGImage/Source）可读写
- [x] 2.2 实现 `crawler.go`：`Crawler` 接口 + `ScrapeResult` 结构（Markdown/HTML/Title/OGImage/Source），使测试转绿
- [x] 2.3 重构确认：`go vet ./internal/reader/service` 通过

## 3. 正文质量校验 isUsableArticle（TDD）

- [x] 3.1 写测试 `readability_crawler_test.go` 中的 `isUsableArticle` 用例：
  - 5814字+0垃圾词 → true（博客园合格）
  - 160字+3垃圾词 → false（36氪页脚，太短）
  - 650字+3垃圾词 → false（假成功，垃圾词过量）
  - 650字+1垃圾词 → true（边界合格）
- [x] 3.2 实现 `readability_crawler.go` 中的 `isUsableArticle(md string) bool` + 垃圾特征词清单常量，使测试转绿
- [x] 3.3 垃圾词清单覆盖实测词：备案/ICP/版权/Copyright/©/举报电话/All Rights/京ICP/登录去登录

## 4. ReadabilityCrawler 实现（TDD）

- [x] 4.1 写测试：`ReadabilityCrawler.ScrapePage` 对本地 HTML 片段（`FromReader` 适配）提取正文，返回 `ScrapeResult{Source:"readability"}`——用 httptest.Server 起本地 SSR 页面，断言 Markdown 非空且 Source 正确
- [x] 4.2 实现 `ReadabilityCrawler`：`FromURL` → readability.Article.Content（HTML）→ html-to-markdown 转换 → 填充 `ScrapeResult`；设置桌面 UA header
- [x] 4.3 确认 `go test ./internal/reader/service` 通过

## 5. FirecrawlService 适配 Crawler 接口（重构）

- [x] 5.1 修改 `firecrawl_service.go`：`ScrapePage` 返回值改为 `*ScrapeResult`（内部仍调 Firecrawl API，做 `ScrapeResponse.Data → ScrapeResult` 格式转换）；保留旧 `ScrapeResponse` 结构供内部解析用
- [x] 5.2 确认 `firecrawl_service.go` 满足 `Crawler` 接口（可赋值给 `var _ Crawler = (*FirecrawlService)(nil)`）
- [x] 5.3 更新 `firecrawl_service_test.go`：断言改为 `result.Markdown` / `result.OGImage`，不再读 `result.Data.Markdown`
- [x] 5.4 确认 `go test ./internal/reader/service` 通过

## 6. FallbackCrawler 降级链（TDD）

- [x] 6.1 写测试 `fallback_crawler_test.go`：
  - readability 合格 → 返回 readability 结果，firecrawl mock 未被调用
  - readability 不合格 → 调用 firecrawl mock，返回 firecrawl 结果
  - readability 不合格 + firecrawl 失败 → 返回 firecrawl 错误
- [x] 6.2 实现 `fallback_crawler.go`：`FallbackCrawler{primary: Crawler, fallback: Crawler}`，`ScrapePage` 先 primary，`isUsableArticle` 判定，不合格才 fallback
- [x] 6.3 确认 `go test ./internal/reader/service` 通过

## 7. 接入调度 job_firecrawl.go

- [x] 7.1 修改 `job_firecrawl.go`：构造 `FallbackCrawler{primary: ReadabilityCrawler, fallback: FirecrawlService}`，替换 `firecrawlService.ScrapePage` 调用；结果读取从 `result.Data.Markdown` 改为 `result.Markdown`，封面图从 `result.Data.Metadata` 改为 `result.OGImage`
- [x] 7.2 确认 `go build ./internal/admin/scheduler` 通过
- [x] 7.3 补充回归测试：job_firecrawl 对 readability 成功路径不调 firecrawl（如可测）

## 8. 接入手动 handler firecrawl_handler.go

- [x] 8.1 修改 `firecrawl_handler.go` 的 `CrawlArticle`：走同一 `FallbackCrawler`，结果读取改中立字段
- [x] 8.2 确认 `go build ./internal/reader/handler` 通过

## 9. wire 导出

- [x] 9.1 修改 `backend-go/internal/reader/wire.go`：导出 `Crawler` 接口类型别名 + `NewFallbackCrawler` 构造函数（供 admin wire 消费）
- [x] 9.2 确认 `go build ./...` 通过

## 10. 后端门禁

- [x] 10.1 `cd backend-go && golangci-lint run ./internal/reader/...` → 0 错误
- [x] 10.2 `cd backend-go && golangci-lint run ./internal/admin/...` → 0 错误
- [x] 10.3 `cd backend-go && go vet ./...` → 0 错误
- [x] 10.4 `cd backend-go && go test ./internal/reader/service ./internal/admin/scheduler` → PASS
- [x] 10.5 `cd backend-go && go build ./...` → 成功

## 11. 测试

本次 change 影响的测试包：

- [x] `backend-go/internal/reader/service`（crawler 接口、readability、firecrawl 适配、fallback）
- [x] `backend-go/internal/admin/scheduler`（job_firecrawl 接入 fallback）

测试命令：

```bash
cd backend-go
go test ./internal/reader/service
go test ./internal/admin/scheduler
```

## 12. 文档

- [x] 12.1 更新 `docs/reference/flow/content-enrichment.md`：补 readability 主力 → Firecrawl 兜底的降级链描述 + 变更溯源表（archive 后补链接，见 §12.2 规范）
- [x] 12.2 更新 `docs/reference/architecture/runtime.md`：Firecrawl scheduler 的定位从"唯一爬虫"改为"SPA 兜底"
- [x] 12.3 更新 `docs/reference/configuration.md`：说明 Firecrawl 现为可选，readability 进程内默认启用
- [x] 12.4 归档后在受影响 flow 文档补「变更溯源」链接

## 13. 验证

归档前重跑以下命令，每条必须零失败：

- [x] `cd backend-go && go vet ./...` → 0 错误
- [x] `cd backend-go && golangci-lint run ./...` → 0 错误
- [x] `cd backend-go && go test ./internal/reader/service` → PASS
- [x] `cd backend-go && go test ./internal/admin/scheduler` → PASS
- [x] `cd backend-go && go build ./...` → 成功
- [x] `grep -rn "result.Data.Markdown\|result.Data.Metadata" backend-go/internal/admin/scheduler backend-go/internal/reader/handler` → 零命中（调用方不再读 Firecrawl 专有结构）
- [x] `grep -rn "NewFallbackCrawler" backend-go/internal/admin/scheduler/job_firecrawl.go backend-go/internal/reader/handler/firecrawl_handler.go` → 有命中（降级链已接入；NewFirecrawlService 作为 fallback 参数保留是符合设计的）

## 部署后影响（强制汇报）

- **(a) 用户可见行为变化**：SSR 站文章（博客园、博客类）正文抓取不再依赖树莓派 Firecrawl，速度更快、更稳；SPA 站文章行为不变（仍走 Firecrawl）。
- **(b) 需要手动操作**：无。readability 是进程内的，重启后端即生效。
- **(c) 旧数据如何降级**：无影响——`article.FirecrawlContent` 字段语义不变，`firecrawl_status` 状态机不变，历史数据无需处理。
