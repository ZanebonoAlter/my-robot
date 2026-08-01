package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
)

// mockFetcher records calls and returns canned responses.
type mockFetcher struct {
	responses map[string]string // url prefix → response body
	calls     [][]string        // each call: [url, header_str]
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		responses: make(map[string]string),
	}
}

func (m *mockFetcher) add(urlPrefix, body string) {
	m.responses[urlPrefix] = body
}

func (m *mockFetcher) Fetch(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	headerStr := ""
	for k, v := range headers {
		headerStr += fmt.Sprintf("%s:%s;", k, v)
	}
	m.calls = append(m.calls, []string{url, headerStr})
	for prefix, body := range m.responses {
		if strings.Contains(url, prefix) {
			return []byte(body), nil
		}
	}
	return []byte(`{}`), nil
}

func TestToolListETFByKeyword_HitsReturnAll(t *testing.T) {
	fetcher := newMockFetcher()
	// Mock ETF spot data response
	etfData := map[string]any{
		"data": map[string]any{
			"diff": []map[string]any{
				{"f12": "512480", "f14": "半导体ETF", "f2": 1.234, "f3": 2.5},
				{"f12": "159995", "f14": "芯片ETF", "f2": 0.987, "f3": 1.2},
				{"f12": "512880", "f14": "证券ETF", "f2": 0.567, "f3": -0.5},
			},
		},
	}
	body, _ := json.Marshal(etfData)
	fetcher.add("eastmoney.com/api/qt/clist", string(body))

	registry := service.NewRegistry(fetcher)

	output, err := registry.Execute(context.Background(), "list_etf_by_keyword", map[string]any{
		"keyword": "半导体",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	totalCount, ok := result["total_count"].(float64)
	if !ok || int(totalCount) != 1 {
		t.Fatalf("total_count = %v, want 1", result["total_count"])
	}
	etfs, _ := result["etfs"].([]any)
	if len(etfs) != 1 {
		t.Fatalf("etfs length = %d, want 1", len(etfs))
	}
}

func TestToolListETFByKeyword_ZeroHitsReturnsHint(t *testing.T) {
	fetcher := newMockFetcher()
	etfData := map[string]any{
		"data": map[string]any{
			"diff": []map[string]any{
				{"f12": "512480", "f14": "半导体ETF", "f2": 1.234, "f3": 2.5},
			},
		},
	}
	body, _ := json.Marshal(etfData)
	fetcher.add("eastmoney.com/api/qt/clist", string(body))

	registry := service.NewRegistry(fetcher)

	output, err := registry.Execute(context.Background(), "list_etf_by_keyword", map[string]any{
		"keyword": "光刻机",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(output, "hit_count") || !strings.Contains(output, "0") {
		t.Fatalf("zero-hits response should contain hit_count=0, got: %s", output)
	}
	if !strings.Contains(output, "hint") {
		t.Fatal("zero-hits response should contain hint")
	}
}

func TestToolGetETFQuote_Valid(t *testing.T) {
	fetcher := newMockFetcher()
	sinaData := `var hq_str_sz159995="芯片ETF,0.987,0.975,0.990,0.970,0.987,0.987,1000000,0.00,0.00,0.00,0.00";`
	fetcher.add("hq.sinajs.cn", sinaData)

	registry := service.NewRegistry(fetcher)

	output, err := registry.Execute(context.Background(), "get_etf_quote", map[string]any{
		"codes": []any{"159995"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v, raw: %s", err, output)
	}
	quotes, _ := result["quotes"].([]any)
	if len(quotes) != 1 {
		t.Fatalf("quotes length = %d, want 1", len(quotes))
	}
	q := quotes[0].(map[string]any)
	if q["code"] != "159995" {
		t.Fatalf("code = %v, want 159995", q["code"])
	}
	if q["name"] != "芯片ETF" {
		t.Fatalf("name = %v, want 芯片ETF", q["name"])
	}
}

func TestToolGetETFQuote_SinaRefererHeader(t *testing.T) {
	fetcher := newMockFetcher()
	sinaData := `var hq_str_sh512480="半导体,1.234,1.200,1.240,1.190,1.234,1.234,500000,0.00,0.00,0.00,0.00";`
	fetcher.add("hq.sinajs.cn", sinaData)

	registry := service.NewRegistry(fetcher)

	_, err := registry.Execute(context.Background(), "get_etf_quote", map[string]any{
		"codes": []any{"512480"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify Referer header was sent.
	foundReferer := false
	for _, call := range fetcher.calls {
		if strings.Contains(call[1], "Referer:https://finance.sina.com.cn") {
			foundReferer = true
			break
		}
	}
	if !foundReferer {
		t.Fatal("expected Sina Referer header in request")
	}
}

func TestToolGetETFQuote_EmptyCodesError(t *testing.T) {
	fetcher := newMockFetcher()
	registry := service.NewRegistry(fetcher)

	output, err := registry.Execute(context.Background(), "get_etf_quote", map[string]any{
		"codes": []any{},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "错误") && !strings.Contains(output, "error") {
		t.Fatalf("empty codes should return error, got: %s", output)
	}
}

func TestToolListSectors_Top30(t *testing.T) {
	fetcher := newMockFetcher()
	// Generate 50 mock sectors
	diffs := make([]map[string]any, 50)
	for i := 0; i < 50; i++ {
		diffs[i] = map[string]any{
			"f14": fmt.Sprintf("板块%d", i+1),
			"f2":  float64(i) * 10.0,
			"f3":  float64(i) - 25.0,
		}
	}
	body, _ := json.Marshal(map[string]any{
		"data": map[string]any{"diff": diffs},
	})
	fetcher.add("m:90+t2", string(body))

	registry := service.NewRegistry(fetcher)

	output, err := registry.Execute(context.Background(), "list_sectors", nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	sectors, _ := result["sectors"].([]any)
	if len(sectors) > 30 {
		t.Fatalf("sectors length = %d, want <= 30", len(sectors))
	}
	if int(result["sector_count"].(float64)) != len(sectors) {
		t.Fatal("sector_count should match actual sector count")
	}
}

func TestToolUnknownRejection(t *testing.T) {
	fetcher := newMockFetcher()
	registry := service.NewRegistry(fetcher)

	output, err := registry.Execute(context.Background(), "get_stock_price", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(output, "未知工具") {
		t.Fatalf("unknown tool should list available names, got: %s", output)
	}
	if !strings.Contains(output, "list_etf_by_keyword") {
		t.Fatal("error should list available tool names")
	}
}

// ── Regression: sync.Once failure cache ────────────────────────────────────

// errorThenSuccessFetcher fails on the first Fetch call, then succeeds.
type errorThenSuccessFetcher struct {
	callCount int
}

func (f *errorThenSuccessFetcher) Fetch(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	f.callCount++
	if f.callCount == 1 {
		return nil, fmt.Errorf("mock network error")
	}
	etfData := map[string]any{
		"data": map[string]any{
			"diff": []map[string]any{
				{"f12": "512480", "f14": "半导体ETF", "f2": 1.234, "f3": 2.5},
			},
		},
	}
	body, _ := json.Marshal(etfData)
	return body, nil
}

func TestToolListETFByKeyword_RetryAfterFetchFailure(t *testing.T) {
	fetcher := &errorThenSuccessFetcher{}
	registry := service.NewRegistry(fetcher)
	ctx := context.Background()

	// First call: fetch fails → should report the error.
	output1, err := registry.Execute(ctx, "list_etf_by_keyword", map[string]any{
		"keyword": "半导体",
	})
	if err != nil {
		t.Fatalf("execute should not error (returns JSON error): %v", err)
	}
	if !strings.Contains(output1, "加载 ETF 数据失败") {
		t.Fatalf("first call should report fetch failure, got: %s", output1)
	}

	// Second call: fetch succeeds → must NOT be permanently cached as failure.
	output2, err := registry.Execute(ctx, "list_etf_by_keyword", map[string]any{
		"keyword": "半导体",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output2, "total_count") {
		t.Fatalf("second call should succeed after retry, got: %s", output2)
	}
}
