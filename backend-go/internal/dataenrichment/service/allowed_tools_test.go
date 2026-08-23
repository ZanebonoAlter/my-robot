package service

import (
	"context"
	"strings"
	"testing"
)

// Allowed-tools contract tests (internal, package service).
//
// After the A-share financial direction was removed, buildAgentAllowedTools has
// no per-source_type conditional tools: the exploration set (list_boards /
// list_lanes / get_lane_detail / web_search, + fetch_page once wired) is always
// layered on top of whatever cfg.AllowedTools carries, and ToolsForSourceType
// always returns nil so cfg.AllowedTools is effectively always empty in
// production. These tests pin that contract and the "unknown tool rejected"
// behaviour.

// TestBuildAgentAllowedTools_ExplorationAlwaysOnPlusConfigured verifies that
// buildAgentAllowedTools always layers the exploration entry points on top of
// the board-configured tools, and dedups.
func TestBuildAgentAllowedTools_ExplorationAlwaysOnPlusConfigured(t *testing.T) {
	// buildAgentAllowedTools does not read receiver state, so a zero-value
	// OrchestratorService suffices (keeps the test free of DI noise).
	orch := &OrchestratorService{}

	// No configured tools → exploration set only.
	base := orch.buildAgentAllowedTools(nil)
	for _, name := range explorationToolNames {
		if !containsStr(base, name) {
			t.Errorf("allowedTools must always include exploration tool %s, got %v", name, base)
		}
	}
	// No financial tools are registered anymore, so the allowed list must not
	// contain any (they cannot be added since they no longer exist).
	for _, name := range []string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"} {
		if containsStr(base, name) {
			t.Errorf("allowedTools must NOT include removed financial tool %s, got %v", name, base)
		}
	}

	// Configured (e.g. future source-typed) tools are layered on top.
	withCfg := orch.buildAgentAllowedTools([]string{"some_future_source_tool"})
	if !containsStr(withCfg, "some_future_source_tool") {
		t.Errorf("configured tool should be layered in, got %v", withCfg)
	}
	for _, name := range explorationToolNames {
		if !containsStr(withCfg, name) {
			t.Errorf("allowedTools must still include exploration tool %s, got %v", name, withCfg)
		}
	}

	// Dedup: feeding exploration names via configuredTools must not duplicate.
	dup := orch.buildAgentAllowedTools([]string{"web_search", "list_boards"})
	if countOccurrences(dup, "web_search") != 1 || countOccurrences(dup, "list_boards") != 1 {
		t.Errorf("allowedTools must dedup, got %v", dup)
	}
}

// TestBuildToolsDesc_ExposesOnlyRegisteredTools ties the allowedTools contract
// to what the agent actually sees in its system prompt: exploration + web_search
// are advertised, and removed financial tool names never appear.
func TestBuildToolsDesc_ExposesOnlyRegisteredTools(t *testing.T) {
	registry := NewRegistry(&nilFetcherHTTP{}) // exploration + web_search registered
	orch := &OrchestratorService{}

	desc := buildToolsDesc(registry, orch.buildAgentAllowedTools(nil))
	for _, name := range []string{"list_boards", "list_lanes", "get_lane_detail", "web_search"} {
		if !strings.Contains(desc, "**"+name+"**") {
			t.Errorf("prompt should expose exploration/web tool %s", name)
		}
	}
	for _, name := range []string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"} {
		if strings.Contains(desc, "**"+name+"**") {
			t.Errorf("prompt must NOT expose removed financial tool %s", name)
		}
	}
}

func containsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func countOccurrences(haystack []string, needle string) int {
	n := 0
	for _, s := range haystack {
		if s == needle {
			n++
		}
	}
	return n
}

// nilFetcherHTTP is a minimal HTTPFetcher for internal tests (no real HTTP).
type nilFetcherHTTP struct{}

func (nilFetcherHTTP) Fetch(_ context.Context, _ string, _ map[string]string) ([]byte, error) {
	return []byte(`{}`), nil
}
