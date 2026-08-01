package service

import (
	"context"
	"testing"
)

// stubCrawler 是测试用的 Crawler 实现，验证接口可被任意实现满足。
type stubCrawler struct {
	md     string
	title  string
	image  string
	source string
}

func (s *stubCrawler) ScrapePage(_ context.Context, _ string) (*ScrapeResult, error) {
	return &ScrapeResult{
		Markdown: s.md,
		Title:    s.title,
		OGImage:  s.image,
		Source:   s.source,
	}, nil
}

// TestCrawler_InterfaceAcceptsAnyImplementation 验证任何实现了
// ScrapePage(ctx,url)→(*ScrapeResult,error) 的类型都能赋值给 Crawler 接口。
func TestCrawler_InterfaceAcceptsAnyImplementation(t *testing.T) {
	var c Crawler = &stubCrawler{md: "body", title: "T", image: "img.png", source: "stub"}

	res, err := c.ScrapePage(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("ScrapePage failed: %v", err)
	}
	if res.Markdown != "body" {
		t.Errorf("Markdown = %q, want %q", res.Markdown, "body")
	}
	if res.Title != "T" {
		t.Errorf("Title = %q, want %q", res.Title, "T")
	}
	if res.OGImage != "img.png" {
		t.Errorf("OGImage = %q, want %q", res.OGImage, "img.png")
	}
	if res.Source != "stub" {
		t.Errorf("Source = %q, want %q", res.Source, "stub")
	}
}
