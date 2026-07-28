package service

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"syntopica-backend/internal/platform/tracing"
)

// FallbackCrawler 实现双源降级链：先尝试 primary（readability，进程内），
// 仅当 primary 结果不合格或报错时才降级调用 fallback（firecrawl）。
//
// 设计意图：SSR 文章（博客园、博客类）在进程内就被 primary 搞定，不碰外部服务；
// SPA 文章（36氪、掘金）primary 提取到空壳，自动降级 fallback 渲染 JS。
type FallbackCrawler struct {
	primary  Crawler // readability，进程内纯 Go
	fallback Crawler // firecrawl，外部 JS 渲染服务
}

// NewFallbackCrawler 构造降级链。primary 通常是 ReadabilityCrawler，
// fallback 通常是 FirecrawlService。
func NewFallbackCrawler(primary, fallback Crawler) *FallbackCrawler {
	return &FallbackCrawler{primary: primary, fallback: fallback}
}

// ScrapePage 先跑 primary；若结果合格直接返回，否则降级 fallback。
// primary 报错不中断，而是降级 fallback（SSR 站偶发网络错误也能救回来）。
// fallback 报错则原样返回（文章进入重试队列，符合现有 firecrawl_status 机制）。
func (f *FallbackCrawler) ScrapePage(ctx context.Context, url string) (*ScrapeResult, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "FallbackCrawler.ScrapePage")
	defer span.End()
	primaryRes, err := f.primary.ScrapePage(ctx, url)
	if err == nil && isUsableArticle(primaryRes.Markdown) {
		return primaryRes, nil
	}

	// primary 不合格或报错 → 降级 fallback
	fallbackRes, fbErr := f.fallback.ScrapePage(ctx, url)
	if fbErr != nil {
		// fallback 也失败：包装错误，保留原 fallback 错误信息供重试队列使用
		if err != nil {
			return nil, fmt.Errorf("primary failed (%v) and fallback failed (%w)", err, fbErr)
		}
		return nil, fmt.Errorf("primary unusable and fallback failed (%w)", fbErr)
	}
	return fallbackRes, nil
}
