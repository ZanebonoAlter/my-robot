package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
)

// ── WebSearcher mocks ──────────────────────────────────────────────────────

type mockWebSearcher struct {
	results []service.WebSearchResult
	err     error
	query   string
	calls   int
}

func (m *mockWebSearcher) Search(_ context.Context, query string) ([]service.WebSearchResult, error) {
	m.calls++
	m.query = query
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func TestWebSearch_NoopReturnsError(t *testing.T) {
	// NoopWebSearcher must surface an error so the agent can self-degrade.
	var ws service.WebSearcher = service.NoopWebSearcher{}
	results, err := ws.Search(context.Background(), "anything")
	if err == nil {
		t.Fatal("NoopWebSearcher.Search should return an error")
	}
	if !errors.Is(err, errors.New("web_search not configured")) && err.Error() != "web_search not configured" {
		t.Fatalf("unexpected error text: %v", err)
	}
	if results != nil {
		t.Fatalf("NoopWebSearcher should return nil results, got %v", results)
	}
}

func TestWebSearch_ToolDelegates(t *testing.T) {
	ws := &mockWebSearcher{
		results: []service.WebSearchResult{
			{Title: "半导体涨价", URL: "https://example.com/a", Snippet: "全球缺芯加剧"},
		},
	}
	registry := service.NewRegistry(&nilFetcher{}, service.WithWebSearcher(ws))

	output, err := registry.Execute(context.Background(), "web_search", map[string]any{
		"query": "半导体 涨价",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if ws.calls != 1 {
		t.Fatalf("expected 1 webSearcher call, got %d", ws.calls)
	}
	if ws.query != "半导体 涨价" {
		t.Fatalf("query forwarded = %q, want %q", ws.query, "半导体 涨价")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v (raw: %s)", err, output)
	}
	if hitCount, _ := result["hit_count"].(float64); int(hitCount) != 1 {
		t.Fatalf("hit_count = %v, want 1", result["hit_count"])
	}
	results, _ := result["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results length = %d, want 1", len(results))
	}
	first := results[0].(map[string]any)
	if first["url"] != "https://example.com/a" {
		t.Fatalf("url = %v", first["url"])
	}
}

func TestWebSearch_BackendError_DegradesToJSON(t *testing.T) {
	ws := &mockWebSearcher{err: errors.New("boom")}
	registry := service.NewRegistry(&nilFetcher{}, service.WithWebSearcher(ws))

	output, err := registry.Execute(context.Background(), "web_search", map[string]any{"query": "x"})
	if err != nil {
		t.Fatalf("web_search backend error must degrade to JSON, got go error: %v", err)
	}
	if !strings.Contains(output, "web_search 失败") {
		t.Fatalf("expected degraded error JSON, got: %s", output)
	}
}

func TestWebSearch_EmptyQuery_ValidationError(t *testing.T) {
	registry := service.NewRegistry(&nilFetcher{}, service.WithWebSearcher(service.NoopWebSearcher{}))
	output, err := registry.Execute(context.Background(), "web_search", map[string]any{"query": ""})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "参数错误") {
		t.Fatalf("expected param error, got: %s", output)
	}
}

func TestWebSearch_DefaultNoopWhenNotInjected(t *testing.T) {
	// NewRegistry without WithWebSearcher defaults to NoopWebSearcher.
	registry := service.NewRegistry(&nilFetcher{})
	output, err := registry.Execute(context.Background(), "web_search", map[string]any{"query": "x"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "web_search not configured") && !strings.Contains(output, "未配置") {
		t.Fatalf("expected not-configured degradation, got: %s", output)
	}
}
