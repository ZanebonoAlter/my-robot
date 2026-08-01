package service

import (
	"context"
	"errors"
	"testing"
)

// mockCrawler 是可编程的 Crawler mock，用于测试降级链调度。
type mockCrawler struct {
	result    *ScrapeResult
	err       error
	called    bool
	callCount int
}

func (m *mockCrawler) ScrapePage(_ context.Context, _ string) (*ScrapeResult, error) {
	m.called = true
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// TestFallbackCrawler_PrimaryUsableSkipsFallback 验证 readability 返回合格正文时，
// fallback（firecrawl）完全不被调用。这是 SSR 文章绕开树莓派的核心场景。
func TestFallbackCrawler_PrimaryUsableSkipsFallback(t *testing.T) {
	primary := &mockCrawler{result: &ScrapeResult{
		Markdown: repeatString("正文内容。", 200), // 1000 字合格正文
		Source:   "readability",
	}}
	fallback := &mockCrawler{result: &ScrapeResult{Markdown: "firecrawl", Source: "firecrawl"}}

	fc := NewFallbackCrawler(primary, fallback)
	res, err := fc.ScrapePage(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("ScrapePage failed: %v", err)
	}
	if res.Source != "readability" {
		t.Errorf("Source = %q, want readability (primary should win)", res.Source)
	}
	if fallback.called {
		t.Error("fallback was called but primary returned usable content")
	}
}

// TestFallbackCrawler_PrimaryUnusableCallsFallback 验证 readability 返回不合格正文
// （空壳/太短）时，自动降级调用 fallback（firecrawl）。这是 SPA 文章走 Firecrawl 的场景。
func TestFallbackCrawler_PrimaryUnusableCallsFallback(t *testing.T) {
	primary := &mockCrawler{result: &ScrapeResult{
		Markdown: "太短了", // 远低于 500 字阈值
		Source:   "readability",
	}}
	fallback := &mockCrawler{result: &ScrapeResult{
		Markdown: repeatString("firecrawl 渲染后的完整正文。", 200),
		Source:   "firecrawl",
	}}

	fc := NewFallbackCrawler(primary, fallback)
	res, err := fc.ScrapePage(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("ScrapePage failed: %v", err)
	}
	if res.Source != "firecrawl" {
		t.Errorf("Source = %q, want firecrawl (should have fallen back)", res.Source)
	}
	if !fallback.called {
		t.Error("fallback was not called but primary was unusable")
	}
}

// TestFallbackCrawler_BothFailReturnsFallbackError 验证 readability 不合格且
// firecrawl 也失败（树莓派挂掉）时，返回 fallback 的错误。SSR 文章不受此影响。
func TestFallbackCrawler_BothFailReturnsFallbackError(t *testing.T) {
	primary := &mockCrawler{result: &ScrapeResult{Markdown: "太短了", Source: "readability"}}
	firecrawlErr := errors.New("firecrawl service unreachable")
	fallback := &mockCrawler{err: firecrawlErr}

	fc := NewFallbackCrawler(primary, fallback)
	_, err := fc.ScrapePage(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error when both crawlers fail, got nil")
	}
	if !errors.Is(err, firecrawlErr) {
		t.Errorf("err = %v, want to wrap firecrawlErr", err)
	}
}

// TestFallbackCrawler_PrimaryErrorCallsFallback 验证 primary 本身报错
// （网络错误等）时也降级到 fallback。
func TestFallbackCrawler_PrimaryErrorCallsFallback(t *testing.T) {
	primary := &mockCrawler{err: errors.New("readability network error")}
	fallback := &mockCrawler{result: &ScrapeResult{
		Markdown: repeatString("firecrawl 正文。", 200),
		Source:   "firecrawl",
	}}

	fc := NewFallbackCrawler(primary, fallback)
	res, err := fc.ScrapePage(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("ScrapePage failed: %v", err)
	}
	if res.Source != "firecrawl" {
		t.Errorf("Source = %q, want firecrawl (should fall back on primary error)", res.Source)
	}
}
