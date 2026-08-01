package service

import (
	"context"
	"strings"
	"testing"
)

// TestBuildAgentAllowedTools_AlwaysIncludesExplorationPlusConditionalFinancial
// verifies the financial-degradation contract. buildAgentAllowedTools always
// layers exploration entry points + web_search on top of the board-configured
// (source-typed) tools. Financial ETF tools only appear when the board config
// supplies them (i.e. board source_types include etf_quote, mapped upstream by
// ToolsForSourceType). Non-financial topics get zero financial tool names, so
// buildToolsDesc never exposes them to the agent.
func TestBuildAgentAllowedTools_AlwaysIncludesExplorationPlusConditionalFinancial(t *testing.T) {
	// buildAgentAllowedTools does not read receiver state, so a zero-value
	// OrchestratorService suffices (keeps the test free of DI noise).
	orch := &OrchestratorService{}

	// Non-financial board: cfg.AllowedTools carries no financial tools.
	nonFin := orch.buildAgentAllowedTools(nil)
	for _, name := range []string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"} {
		if containsStr(nonFin, name) {
			t.Errorf("non-financial allowedTools must NOT include %s, got %v", name, nonFin)
		}
	}
	for _, name := range explorationToolNames {
		if !containsStr(nonFin, name) {
			t.Errorf("allowedTools must always include %s, got %v", name, nonFin)
		}
	}

	// Financial board: cfg.AllowedTools carries the ETF tools.
	finTools := []string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"}
	fin := orch.buildAgentAllowedTools(finTools)
	for _, name := range finTools {
		if !containsStr(fin, name) {
			t.Errorf("financial allowedTools must include %s, got %v", name, fin)
		}
	}
	for _, name := range explorationToolNames {
		if !containsStr(fin, name) {
			t.Errorf("allowedTools must always include %s even on financial board, got %v", name, fin)
		}
	}

	// Dedup: feeding exploration names via configuredTools must not duplicate.
	dup := orch.buildAgentAllowedTools([]string{"web_search", "list_boards"})
	if countOccurrences(dup, "web_search") != 1 || countOccurrences(dup, "list_boards") != 1 {
		t.Errorf("allowedTools must dedup, got %v", dup)
	}
}

// TestBuildToolsDesc_HidesFinancialToolsForNonFinancialBoard ties the
// allowedTools contract to what the agent actually sees in its system prompt.
func TestBuildToolsDesc_HidesFinancialToolsForNonFinancialBoard(t *testing.T) {
	registry := NewRegistry(&nilFetcherHTTP{}) // all tools registered
	orch := &OrchestratorService{}

	// Financial board prompt exposes both financial + exploration tools.
	finDesc := buildToolsDesc(registry, orch.buildAgentAllowedTools(
		[]string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"}))
	for _, name := range []string{"list_etf_by_keyword", "get_lane_detail", "web_search"} {
		if !strings.Contains(finDesc, "**"+name+"**") {
			t.Errorf("financial board prompt should expose %s", name)
		}
	}

	// Non-financial board prompt exposes exploration + web_search only.
	nonFinDesc := buildToolsDesc(registry, orch.buildAgentAllowedTools(nil))
	for _, name := range []string{"list_boards", "list_lanes", "get_lane_detail", "web_search"} {
		if !strings.Contains(nonFinDesc, "**"+name+"**") {
			t.Errorf("non-financial board prompt should expose exploration tool %s", name)
		}
	}
	if strings.Contains(nonFinDesc, "list_etf_by_keyword") {
		t.Error("non-financial board prompt must NOT expose financial tools")
	}
	if strings.Contains(nonFinDesc, "get_etf_quote") {
		t.Error("non-financial board prompt must NOT expose get_etf_quote")
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
	return []byte(`{"data":{"diff":[]}}`), nil
}
