package service

import (
	"context"
	"encoding/json"
	"errors"
)

// WebSearchResult is a single hit from a web search.
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearcher abstracts a web search backend (Tavily/Serper/etc.).
// The agent loop calls it via the web_search tool. Implementations are injected
// at wiring time; when none is configured, NoopWebSearcher is used and the tool
// returns a graceful error JSON so the agent can self-degrade.
//
// NOTE(阶段2a-ii): no real search API is wired yet — users must configure a key
// and inject a real implementation. The default is NoopWebSearcher.
type WebSearcher interface {
	Search(ctx context.Context, query string) ([]WebSearchResult, error)
}

// NoopWebSearcher is the default WebSearcher used when no search backend is
// configured. It always returns an error, signalling the agent to degrade.
type NoopWebSearcher struct{}

// Search implements WebSearcher by returning a "not configured" error.
func (NoopWebSearcher) Search(_ context.Context, _ string) ([]WebSearchResult, error) {
	return nil, errors.New("web_search not configured")
}

// ── Tool: web_search ────────────────────────────────────────────────────────

// executeWebSearch delegates to the registry's WebSearcher and returns JSON.
// On any failure (unconfigured/backend error) it returns an error JSON string
// and a nil Go error, matching the registry's graceful-degradation convention
// for single-tool failures (the agent loop already handles these via its three
// defenses: dedup / no-truncation / thinking-off).
func (r *Registry) executeWebSearch(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return jsonError("参数错误: query 必须为非空字符串"), nil
	}
	if r.webSearcher == nil {
		return jsonError("web_search 未配置"), nil
	}
	results, err := r.webSearcher.Search(ctx, query)
	if err != nil {
		return jsonError("web_search 失败: " + err.Error()), nil
	}
	if len(results) == 0 {
		b, _ := json.Marshal(map[string]any{
			"query":     query,
			"hit_count": 0,
			"hint":      "没有命中网页，可换更具体或更宽泛的关键词重试",
			"results":   []WebSearchResult{},
		})
		return string(b), nil
	}
	b, _ := json.Marshal(map[string]any{
		"query":     query,
		"hit_count": len(results),
		"results":   results,
	})
	return string(b), nil
}
