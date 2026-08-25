package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
)

// stubPageFetcher is a test double for PageFetcher.
type stubPageFetcher struct {
	title    string
	mainText string
	err      error
	url      string
	calls    int
}

func (s *stubPageFetcher) FetchPage(_ context.Context, url string) (string, string, error) {
	s.calls++
	s.url = url
	if s.err != nil {
		return "", "", s.err
	}
	return s.title, s.mainText, nil
}

func TestFetchPage_ToolReturnsContent(t *testing.T) {
	pf := &stubPageFetcher{title: "石油危机回顾", mainText: "1973 年第四次中东战争..."}
	registry := service.NewRegistry(&nilFetcher{}, service.WithPageFetcher(pf))

	output, err := registry.Execute(context.Background(), "fetch_page", map[string]any{
		"url": "https://example.com/oil",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if pf.calls != 1 || pf.url != "https://example.com/oil" {
		t.Fatalf("fetcher not called once with url, got calls=%d url=%q", pf.calls, pf.url)
	}
	if !strings.Contains(output, "石油危机回顾") || !strings.Contains(output, "main_text") || !strings.Contains(output, "1973") {
		t.Fatalf("expected title + main_text in output, got: %s", output)
	}
}

func TestFetchPage_NotConfiguredDegrades(t *testing.T) {
	// Registry without WithPageFetcher → fetch_page degrades to error JSON.
	registry := service.NewRegistry(&nilFetcher{})
	output, err := registry.Execute(context.Background(), "fetch_page", map[string]any{"url": "https://x"})
	if err != nil {
		t.Fatalf("fetch_page not-configured must degrade to JSON, got go error: %v", err)
	}
	if !strings.Contains(output, "fetch_page 未配置") {
		t.Fatalf("expected not-configured degradation, got: %s", output)
	}
}

func TestFetchPage_EmptyURLValidationError(t *testing.T) {
	pf := &stubPageFetcher{title: "t", mainText: "m"}
	registry := service.NewRegistry(&nilFetcher{}, service.WithPageFetcher(pf))

	output, err := registry.Execute(context.Background(), "fetch_page", map[string]any{"url": ""})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "参数错误") {
		t.Fatalf("expected param error for empty url, got: %s", output)
	}
	if pf.calls != 0 {
		t.Fatalf("fetcher should not be called on empty url, got %d calls", pf.calls)
	}
}

func TestFetchPage_ScrapeFailureDegradesToJSONNoGoError(t *testing.T) {
	pf := &stubPageFetcher{err: errors.New("timeout")}
	registry := service.NewRegistry(&nilFetcher{}, service.WithPageFetcher(pf))

	output, err := registry.Execute(context.Background(), "fetch_page", map[string]any{"url": "https://x"})
	if err != nil {
		t.Fatalf("fetch_page scrape failure must NOT return a go error (agent loop continues), got: %v", err)
	}
	if !strings.Contains(output, "fetch_page 失败") {
		t.Fatalf("expected degraded error JSON, got: %s", output)
	}
}
