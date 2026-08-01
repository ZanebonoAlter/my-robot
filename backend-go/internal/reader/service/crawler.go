package service

import "context"

// Crawler 是文章正文抓取的中立接口。
// readability（进程内纯 Go）与 firecrawl（外部 JS 渲染服务）都实现它，
// 调用方（job / handler）只依赖本接口，不感知具体爬虫的实现细节与专有 JSON 结构。
type Crawler interface {
	ScrapePage(ctx context.Context, url string) (*ScrapeResult, error)
}

// ScrapeResult 是中立抓取结果，不绑死任何爬虫的响应结构。
// 下游统一读取 Markdown 写入 article.FirecrawlContent、读 OGImage 回填封面图。
type ScrapeResult struct {
	Markdown string // 清洗后的正文 Markdown（写入 article.FirecrawlContent）
	HTML     string // 原始 HTML（可选，供未来扩展）
	Title    string // 文章标题
	OGImage  string // 封面图（OG/Twitter，补 article.ImageURL 用）
	Source   string // 来源标记："readability" | "firecrawl"
}
