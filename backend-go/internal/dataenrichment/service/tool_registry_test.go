package service_test

import (
	"context"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
)

// TestRegistryTools_NoFinancialTools verifies the A-share financial direction
// was fully removed: the registered tool set exposes the internal navigation +
// web_search but none of list_etf_by_keyword / get_etf_quote / list_sectors.
func TestRegistryTools_NoFinancialTools(t *testing.T) {
	registry := service.NewRegistry(&nilFetcher{})
	tools := registry.Tools()

	for _, name := range []string{"list_boards", "list_lanes", "get_lane_detail", "web_search", "fetch_page"} {
		if tools[name] == nil {
			t.Errorf("expected registered tool %s", name)
		}
	}
	for _, name := range []string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"} {
		if tools[name] != nil {
			t.Errorf("removed financial tool %s must NOT be registered", name)
		}
	}
}

// TestToolUnknownRejection verifies that calling an unregistered tool name
// returns an error and lists the available tools (financial tools absent).
func TestToolUnknownRejection(t *testing.T) {
	registry := service.NewRegistry(&nilFetcher{})

	output, err := registry.Execute(context.Background(), "get_stock_price", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(output, "未知工具") {
		t.Fatalf("unknown tool should report 未知工具, got: %s", output)
	}
	// Available list must include an always-on tool and exclude financial tools.
	if !strings.Contains(output, "web_search") {
		t.Fatal("error should list an available tool (web_search)")
	}
	if strings.Contains(output, "list_etf_by_keyword") || strings.Contains(output, "get_etf_quote") {
		t.Fatalf("available list must NOT include removed financial tools, got: %s", output)
	}
}
