package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"syntopica-backend/internal/platform/httpclient"
)

// HTTPFetcher abstracts HTTP GET requests for testability.
type HTTPFetcher interface {
	Fetch(ctx context.Context, url string, headers map[string]string) ([]byte, error)
}

// DefaultHTTPFetcher is the production HTTP client with 5s timeout.
type DefaultHTTPFetcher struct {
	client *http.Client
}

// NewDefaultHTTPFetcher creates a fetcher with 5s timeout.
func NewDefaultHTTPFetcher() *DefaultHTTPFetcher {
	return &DefaultHTTPFetcher{
		client: httpclient.New(httpclient.WithTimeout(5 * time.Second)),
	}
}

// Fetch performs an HTTP GET with custom headers.
func (f *DefaultHTTPFetcher) Fetch(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}

// ── Tool definition ─────────────────────────────────────────────────────────

// Tool represents a callable data source tool.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Execute     func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds all registered data source tools.
type Registry struct {
	tools   map[string]*Tool
	fetcher HTTPFetcher

	etfCacheMu     sync.Mutex
	etfCache       []map[string]any
	etfCacheLoaded bool

	// Exploration tools (阶段2a-ii). Each is optional; when nil the tool returns
	// a graceful error JSON. webSearcher defaults to NoopWebSearcher so the
	// web_search tool degrades cleanly until a real backend is configured.
	webSearcher        WebSearcher
	boardLister        BoardLister
	laneLister         LaneLister
	laneDetailRenderer LaneDetailRenderer
}

// RegistryOption configures optional exploration dependencies on a Registry.
// Financial ETF tools only need the HTTP fetcher and are always registered.
type RegistryOption func(*Registry)

// WithWebSearcher injects a web search backend (default NoopWebSearcher).
func WithWebSearcher(ws WebSearcher) RegistryOption {
	return func(r *Registry) { r.webSearcher = ws }
}

// WithBoardLister injects the board-panorama data source for list_boards.
func WithBoardLister(bl BoardLister) RegistryOption {
	return func(r *Registry) { r.boardLister = bl }
}

// WithLaneLister injects the persistent-topic data source for list_lanes.
func WithLaneLister(ll LaneLister) RegistryOption {
	return func(r *Registry) { r.laneLister = ll }
}

// WithLaneDetailRenderer injects the lifeline renderer for get_lane_detail.
func WithLaneDetailRenderer(ldr LaneDetailRenderer) RegistryOption {
	return func(r *Registry) { r.laneDetailRenderer = ldr }
}

// NewRegistry creates a tool registry with the given HTTP fetcher and any
// optional exploration dependencies. Existing callers that pass only the
// fetcher keep working (exploration tools degrade to error JSON).
func NewRegistry(fetcher HTTPFetcher, opts ...RegistryOption) *Registry {
	r := &Registry{
		tools:       make(map[string]*Tool),
		fetcher:     fetcher,
		webSearcher: NoopWebSearcher{},
	}
	for _, opt := range opts {
		opt(r)
	}
	r.register()
	return r
}

// Tools returns the registered tools map (read-only).
func (r *Registry) Tools() map[string]*Tool {
	return r.tools
}

// Execute runs a tool by name with the given arguments.
// Returns JSON string on success or error JSON on failure.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	tool := r.tools[name]
	if tool == nil {
		names := make([]string, 0, len(r.tools))
		for k := range r.tools {
			names = append(names, k)
		}
		errJSON, _ := json.Marshal(map[string]any{
			"error":     fmt.Sprintf("未知工具: %s", name),
			"available": names,
		})
		return string(errJSON), fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, args)
}

func (r *Registry) register() {
	r.tools["list_etf_by_keyword"] = &Tool{
		Name:        "list_etf_by_keyword",
		Description: "按名称关键词筛选全市场 ETF，返回命中的 ETF 代码和名称清单。命中0时换宽泛词重试。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"keyword": map[string]any{"type": "string"}},
			"required":   []string{"keyword"},
		},
		Execute: r.executeListETFByKeyword,
	}

	r.tools["get_etf_quote"] = &Tool{
		Name:        "get_etf_quote",
		Description: "获取指定 ETF 代码列表的实时行情（最新价、涨跌幅）。需先通过 list_etf_by_keyword 拿到代码。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"codes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
			"required":   []string{"codes"},
		},
		Execute: r.executeGetETFQuote,
	}

	r.tools["list_sectors"] = &Tool{
		Name:        "list_sectors",
		Description: "列出 A 股全部行业板块及其当日涨跌幅。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: r.executeListSectors,
	}

	// ── Exploration entry points + web_search (阶段2a-ii) ──────────────────
	// These are always registered; EnrichTopic always allows them for the agent
	// so it has multi-level navigation + a web fallback regardless of board
	// source_type. The financial tools above are conditionally exposed via
	// allowedTools (board_config ToolsForSourceType).

	r.tools["list_boards"] = &Tool{
		Name:        "list_boards",
		Description: "列出全部语义版块(全景),返回 [{id,name,active_lanes 活跃泳道数}]。无参。用于从顶层选择版块。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: r.executeListBoards,
	}

	r.tools["list_lanes"] = &Tool{
		Name:        "list_lanes",
		Description: "列出某版块下的全部持久话题泳道,返回 [{lane_id,label,status,hit_count,consecutive_hits}]。先用 list_boards 拿到 board_id。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"board_id": map[string]any{"type": "integer"}},
			"required":   []string{"board_id"},
		},
		Execute: r.executeListLanes,
	}

	r.tools["get_lane_detail"] = &Tool{
		Name:        "get_lane_detail",
		Description: "查看某泳道(持久话题)的近期演进详情(复用 lifeline 渲染)。先用 list_lanes 拿到 lane_id。window_days 默认 14。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"lane_id":     map[string]any{"type": "integer"},
				"window_days": map[string]any{"type": "integer"},
			},
			"required": []string{"lane_id"},
		},
		Execute: r.executeGetLaneDetail,
	}

	r.tools["web_search"] = &Tool{
		Name:        "web_search",
		Description: "网络搜索(外部知识补充),返回 [{title,url,snippet}]。当内部数据源不足时用。若未配置会返回错误,你可基于已有数据继续或跳过。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"query": map[string]any{"type": "string"}},
			"required":   []string{"query"},
		},
		Execute: r.executeWebSearch,
	}
}

// ── Tool: list_etf_by_keyword ────────────────────────────────────────────────

func (r *Registry) executeListETFByKeyword(ctx context.Context, args map[string]any) (string, error) {
	keyword, ok := args["keyword"].(string)
	if !ok || strings.TrimSpace(keyword) == "" {
		return jsonError("参数错误: keyword 必须为非空字符串"), nil
	}
	keyword = strings.TrimSpace(keyword)

	// Lazy-load ETF cache with retry-on-failure (mutex + loaded flag).
	// Unlike sync.Once, a failed load does not get permanently cached.
	r.etfCacheMu.Lock()
	if !r.etfCacheLoaded {
		cache, err := r.loadETFSpot(ctx)
		if err != nil {
			r.etfCacheMu.Unlock()
			return jsonError("加载 ETF 数据失败: " + err.Error()), nil
		}
		r.etfCache = cache
		r.etfCacheLoaded = true
	}
	cache := r.etfCache
	r.etfCacheMu.Unlock()

	hits := make([]map[string]any, 0)
	for _, etf := range cache {
		name, _ := etf["名称"].(string)
		if strings.Contains(name, keyword) {
			slim := map[string]any{
				"代码":  etf["代码"],
				"名称":  etf["名称"],
				"涨跌幅": etf["涨跌幅"],
			}
			hits = append(hits, slim)
		}
	}

	if len(hits) == 0 {
		b, _ := json.Marshal(map[string]any{
			"hit_count": 0,
			"hint":      fmt.Sprintf("没有名称含'%s'的 ETF,建议换更宽泛的产业关键词重试", keyword),
		})
		return string(b), nil
	}

	b, _ := json.Marshal(map[string]any{
		"total_count": len(hits),
		"etfs":        hits,
		"note":        "已返回全部命中,无需重查",
	})
	return string(b), nil
}

// loadETFSpot fetches the full ETF spot list from Eastmoney (akshare equivalent).
// This is called only once via sync.Once.
func (r *Registry) loadETFSpot(ctx context.Context) ([]map[string]any, error) {
	// Eastmoney fund ETF spot API. Returns all ETFs with basic info.
	url := "http://push2.eastmoney.com/api/qt/clist/get"
	params := "?pn=1&pz=10000&po=1&np=1&fltt=2&invt=2&fid=f3&fs=b:MK0021,b:MK0022,b:MK0023,b:MK0024&fields=f2,f3,f4,f12,f14"
	fullURL := url + params
	body, err := r.fetcher.Fetch(ctx, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch ETF list: %w", err)
	}

	var result struct {
		Data struct {
			Diff []map[string]any `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse ETF list: %w", err)
	}

	etfs := make([]map[string]any, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		code, _ := item["f12"].(string)
		name, _ := item["f14"].(string)
		price, _ := item["f2"].(float64)
		chgPct, _ := item["f3"].(float64)
		if code == "" || name == "" {
			continue
		}
		etfs = append(etfs, map[string]any{
			"代码":  code,
			"名称":  name,
			"最新价": price,
			"涨跌幅": chgPct,
		})
	}
	return etfs, nil
}

// ── Tool: get_etf_quote ─────────────────────────────────────────────────────

func (r *Registry) executeGetETFQuote(ctx context.Context, args map[string]any) (string, error) {
	codesRaw, ok := args["codes"]
	if !ok {
		return jsonError("参数错误: codes 必须提供"), nil
	}

	codes, ok := toStringSlice(codesRaw)
	if !ok {
		// Try []any
		if codeList, ok2 := codesRaw.([]any); ok2 {
			codes = make([]string, 0, len(codeList))
			for _, c := range codeList {
				codes = append(codes, fmt.Sprint(c))
			}
			ok = true
		}
	}
	if !ok || len(codes) == 0 {
		return jsonError("参数错误: codes 必须为非空数组"), nil
	}

	prefixed := make([]string, 0, len(codes))
	for _, c := range codes {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		prefix := "sh"
		if !strings.HasPrefix(c, "5") && !strings.HasPrefix(c, "6") && !strings.HasPrefix(c, "9") {
			prefix = "sz"
		}
		prefixed = append(prefixed, fmt.Sprintf("%s%s", prefix, c))
	}
	if len(prefixed) == 0 {
		return jsonError("无有效代码"), nil
	}

	url := "http://hq.sinajs.cn/list=" + strings.Join(prefixed, ",")
	headers := map[string]string{
		"Referer": "https://finance.sina.com.cn",
	}
	body, err := r.fetcher.Fetch(ctx, url, headers)
	if err != nil {
		return jsonError("请求失败: " + err.Error()), nil
	}

	results := make([]map[string]any, 0)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}
		codePart := parts[0]
		// Extract code from var name like "var hq_str_sh512480"
		if idx := strings.LastIndex(codePart, "_"); idx >= 0 {
			codePart = codePart[idx+1:]
		}
		if idx := strings.Index(codePart, "."); idx >= 0 {
			codePart = codePart[:idx]
		}
		// Strip sh/sz prefix to return clean code.
		if len(codePart) > 2 && (strings.HasPrefix(codePart, "sh") || strings.HasPrefix(codePart, "sz")) {
			codePart = codePart[2:]
		}
		content := parts[1]
		content = strings.Trim(content, "\"")
		fields := strings.Split(content, ",")
		if len(fields) < 4 {
			continue
		}
		name := fields[0]
		price := fields[1]
		prevClose := fields[2]
		chgPct := 0.0
		p, err1 := parseFloat(price)
		pc, err2 := parseFloat(prevClose)
		if err1 == nil && err2 == nil && pc != 0 {
			chgPct = (p - pc) / pc * 100
		}
		results = append(results, map[string]any{
			"code":    codePart,
			"name":    name,
			"price":   price,
			"chg_pct": chgPct,
		})
	}

	b, _ := json.Marshal(map[string]any{"quotes": results})
	return string(b), nil
}

// ── Tool: list_sectors ──────────────────────────────────────────────────────

func (r *Registry) executeListSectors(ctx context.Context, _ map[string]any) (string, error) {
	url := "http://push2.eastmoney.com/api/qt/clist/get"
	params := "?pn=1&pz=100&po=1&np=1&fltt=2&invt=2&fid=f3&fs=m:90+t2&fields=f2,f3,f14"
	fullURL := url + params
	body, err := r.fetcher.Fetch(ctx, fullURL, nil)
	if err != nil {
		return jsonError("获取板块失败: " + err.Error()), nil
	}

	var result struct {
		Data struct {
			Diff []map[string]any `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return jsonError("解析板块数据失败: " + err.Error()), nil
	}

	sectors := make([]map[string]any, 0)
	for i, item := range result.Data.Diff {
		if i >= 30 {
			break
		}
		name, _ := item["f14"].(string)
		price, _ := item["f2"].(float64)
		chgPct, _ := item["f3"].(float64)
		sectors = append(sectors, map[string]any{
			"板块名称": name,
			"最新价":  price,
			"涨跌幅":  chgPct,
		})
	}

	b, _ := json.Marshal(map[string]any{
		"sector_count": len(sectors),
		"sectors":      sectors,
	})
	return string(b), nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func jsonError(msg string) string {
	b, _ := json.Marshal(map[string]any{"error": msg})
	return string(b)
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func toStringSlice(v any) ([]string, bool) {
	if s, ok := v.([]string); ok {
		return s, true
	}
	return nil, false
}

// toUint coerces a JSON-decoded numeric arg to uint. Handles both float64
// (the default json.Unmarshal type for numbers) and json.Number.
func toUint(v any) (uint, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 || n != float64(uint(n)) {
			return 0, false
		}
		return uint(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil || i < 0 {
			return 0, false
		}
		return uint(i), true
	}
	return 0, false
}

// jsonMarshal is a thin wrapper over json.Marshal that never returns an error
// for the simple map[string]any payloads the tools build (it panics only on
// truly impossible inputs, which the static shapes here preclude).
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
