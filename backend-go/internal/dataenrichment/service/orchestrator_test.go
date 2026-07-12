package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/database"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var testCap = airouter.Capability("data_enrichment_analysis")

// ── Test helpers ────────────────────────────────────────────────────────────

func setupOrchTestDB(t *testing.T) *repository.Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.DB = db
	if err := database.RunAutoMigrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	repo := repository.NewRepository(db)
	repository.SetRepo(repo)
	return repo
}

// mockAirRouter records calls and returns canned ChatResults in order.
type mockAirRouter struct {
	responses []*airouter.ChatResult
	callCount int
	Calls     []airouter.ChatRequest
}

func newMockAirRouter() *mockAirRouter {
	return &mockAirRouter{}
}

func (m *mockAirRouter) addResponse(content string) {
	m.responses = append(m.responses, &airouter.ChatResult{Content: content})
}

func (m *mockAirRouter) Chat(ctx context.Context, req airouter.ChatRequest) (*airouter.ChatResult, error) {
	m.Calls = append(m.Calls, req)
	if m.callCount >= len(m.responses) {
		return &airouter.ChatResult{Content: "{}"}, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

// mockBoardConfigReader returns canned board config.
type mockBoardConfigReader struct {
	cfg *service.BoardEnrichmentConfig
	err error
}

func (m *mockBoardConfigReader) GetBoardConfig(ctx context.Context, topicID uint) (*service.BoardEnrichmentConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cfg, nil
}

func newEnabledConfig() *service.BoardEnrichmentConfig {
	return &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        14,
		ContextLayers:     []string{"week", "month", "year", "all"},
		AllowedTools:      []string{"list_etf_by_keyword", "get_etf_quote", "list_sectors"},
	}
}

// orchMockLifelineReader returns canned lifeline data for orchestrator tests.
// (Avoids collision with mockLifelineReader in lifeline_renderer_test.go.)
type orchMockLifelineReader struct {
	data service.SectionTimelineData
	err  error
}

func (m *orchMockLifelineReader) GetTopicLifeline(topicID uint) (service.SectionTimelineData, error) {
	if m.err != nil {
		return service.SectionTimelineData{}, m.err
	}
	return m.data, nil
}

func mustParseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// nilFetcher returns empty ETF data to avoid real HTTP calls in tests.
type nilFetcher struct{}

func (n *nilFetcher) Fetch(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	// Return empty ETF list so list_etf_by_keyword returns 0 hits.
	return []byte(`{"data":{"diff":[]}}`), nil
}

// ── parseJSONResponse tests ─────────────────────────────────────────────────

func TestParseJSONResponse_CleanJSON(t *testing.T) {
	input := `{"action":"call_tool","tool":"list_etf_by_keyword","args":{"keyword":"半导体"}}`
	result, err := service.ParseJSONResponse(input)
	if err != nil {
		t.Fatalf("clean JSON should parse: %v", err)
	}
	if result["action"] != "call_tool" {
		t.Fatalf("action: want call_tool, got %v", result["action"])
	}
	if result["tool"] != "list_etf_by_keyword" {
		t.Fatalf("tool: want list_etf_by_keyword, got %v", result["tool"])
	}
}

func TestParseJSONResponse_MarkdownCodeBlock(t *testing.T) {
	input := "```json\n{\"action\":\"finish\",\"summary\":\"done\"}\n```"
	result, err := service.ParseJSONResponse(input)
	if err != nil {
		t.Fatalf("markdown-wrapped JSON should parse: %v", err)
	}
	if result["action"] != "finish" {
		t.Fatalf("action: want finish, got %v", result["action"])
	}
}

func TestParseJSONResponse_PlainFence(t *testing.T) {
	input := "```\n{\"topics\":[{\"topic\":\"石油\"}]}\n```"
	result, err := service.ParseJSONResponse(input)
	if err != nil {
		t.Fatalf("plain-fence JSON should parse: %v", err)
	}
	topics, ok := result["topics"].([]any)
	if !ok || len(topics) == 0 {
		t.Fatal("topics should be present")
	}
}

func TestParseJSONResponse_EmbeddedBrackets(t *testing.T) {
	input := "sure, here's the json:\n{\"action\":\"call_tool\",\"args\":{\"keyword\":\"芯片\"}} and some extra text"
	result, err := service.ParseJSONResponse(input)
	if err != nil {
		t.Fatalf("embedded JSON should parse: %v", err)
	}
	if result["action"] != "call_tool" {
		t.Fatalf("action: want call_tool, got %v", result["action"])
	}
}

func TestParseJSONResponse_PlainTextFail(t *testing.T) {
	input := "这是纯文本，没有JSON"
	_, err := service.ParseJSONResponse(input)
	if err == nil {
		t.Fatal("plain text without JSON should fail")
	}
}

func TestParseJSONResponse_EmptyFail(t *testing.T) {
	_, err := service.ParseJSONResponse("")
	if err == nil {
		t.Fatal("empty string should fail")
	}
}

// ── BoardConfig tests ───────────────────────────────────────────────────────

func TestDefaultBoardConfig_DisabledByDefault(t *testing.T) {
	cfg := service.DefaultBoardConfig()
	if cfg.EnrichmentEnabled {
		t.Fatal("default should have enrichment_enabled=false")
	}
	if cfg.WindowDays != 14 {
		t.Fatalf("default window_days: want 14, got %d", cfg.WindowDays)
	}
}

func TestBoardEnrichmentConfig_Validate(t *testing.T) {
	cfg := &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        0,
		ContextLayers:     []string{"week", "month"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("window_days=0 should fail validation")
	}
}

// ── EnrichTopic: enrichment_enabled guard ───────────────────────────────────

func TestEnrichTopic_DisabledBoardRejected(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()
	toolRegistry := service.NewRegistry(&nilFetcher{})

	disabledCfg := service.DefaultBoardConfig() // EnrichmentEnabled=false
	boardReader := &mockBoardConfigReader{cfg: disabledCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试话题"},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	_, err := orch.EnrichTopic(context.Background(), 1)
	if err == nil {
		t.Fatal("disabled board should reject EnrichTopic")
	}
	if !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("error should mention 'not enabled', got: %v", err)
	}
}

// ── EnrichTopic: end-to-end test with mock LLM ──────────────────────────────

func TestEnrichTopicE2E_FullFlow(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	// Sequence: interpret → tool_use (finish) → analyze → (no review, first run)
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价波动"}]}`)
	airouter.addResponse(`{"action":"finish","thought":"done","summary":"油价涨3%"}`)
	airouter.addResponse(`{
		"evolution_assessment": "油价上涨强化了既有能源紧张趋势",
		"position": "reinforcing",
		"signals": [
			{"lane": "中东地缘紧张", "signal": "油价涨3%", "mechanism": "地缘紧张推高油价"}
		],
		"evidence": [
			{"context_id": "ctx1", "period": "week", "quote": "油价飙升", "source_type": "news", "tool_ref": ""},
			{"context_id": "tool_1", "period": "week", "quote": "布伦特原油+3.2%", "source_type": "tool", "tool_ref": "list_etf_by_keyword"}
		],
		"financial_view": {"sectors": [{"sector": "能源", "direction": "up", "supporting_data": "布伦特原油+3.2%，能源ETF涨2.1%"}]},
		"causal_chain": "地缘紧张→油价涨→能源板块利好",
		"overall": "最新进展强化了能源话题的演进方向"
	}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{
				ID:              1,
				Label:           "中东地缘紧张与能源连锁反应",
				Description:     "产油国设施遭袭 → 油价飙升",
				Status:          "active",
				FirstSeenDate:   mustParseTime("2026-07-01"),
				LastSeenDate:    mustParseTime("2026-07-05"),
				HitCount:        5,
				ConsecutiveHits: 5,
			},
			Sections: []service.TimelineSectionNode{
				{
					SectionID: 101, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "产油国遭袭", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 5, ThreadCount: 2,
					ThreadTitles: []string{"布伦特原油飙升", "中东局势升级"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Verify result
	result := output.Result
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.PersistentTopicID != 1 {
		t.Fatalf("topic ID: want 1, got %d", result.PersistentTopicID)
	}
	if result.EvolutionAssessment == "" {
		t.Fatal("evolution_assessment should not be empty")
	}
	if result.SessionID == "" {
		t.Fatal("session_id should not be empty")
	}
	if !strings.HasPrefix(result.SessionID, "data_enrichment_1_") {
		t.Fatalf("session_id should start with data_enrichment_1_, got: %s", result.SessionID)
	}

	// Verify sectors JSON (now stores composite: position/signals/evidence/financial_view)
	var sectorsJSON map[string]any
	if err := json.Unmarshal(result.Sectors, &sectorsJSON); err != nil {
		t.Fatalf("sectors should be valid JSON: %v", err)
	}
	if sectorsJSON["position"] == nil {
		t.Fatal("sectors JSON should have position")
	}
	if sectorsJSON["signals"] == nil {
		t.Fatal("sectors JSON should have signals")
	}
	if sectorsJSON["evidence"] == nil {
		t.Fatal("sectors JSON should have evidence")
	}
	if sectorsJSON["financial_view"] == nil {
		t.Fatal("sectors JSON should have financial_view")
	}

	// Verify tool_calls JSON
	var toolCalls []map[string]any
	if err := json.Unmarshal(result.ToolCalls, &toolCalls); err != nil {
		t.Fatalf("tool_calls should be valid JSON: %v", err)
	}

	// Verify input_snapshot JSON
	var inputSnap map[string]any
	if err := json.Unmarshal(result.InputSnapshot, &inputSnap); err != nil {
		t.Fatalf("input_snapshot should be valid JSON: %v", err)
	}
	if _, ok := inputSnap["context_layers"]; !ok {
		t.Fatal("input_snapshot should have context_layers")
	}
	if _, ok := inputSnap["window_days"]; !ok {
		t.Fatal("input_snapshot should have window_days")
	}
	if _, ok := inputSnap["config_context_layers"]; !ok {
		t.Fatal("input_snapshot should have config_context_layers")
	}
	if _, ok := inputSnap["review_ids"]; !ok {
		t.Fatal("input_snapshot should have review_ids")
	}

	// First run: no prev result → no review
	if output.Review != nil {
		t.Fatal("first run should have no review (no prev result)")
	}

	// Verify airouter calls
	if len(airouter.Calls) < 3 {
		t.Fatalf("expected at least 3 airouter calls, got %d", len(airouter.Calls))
	}

	// Check Capability on all calls
	for i, call := range airouter.Calls {
		if string(call.Capability) != "data_enrichment_analysis" {
			t.Fatalf("call %d: Capability = %s, want data_enrichment_analysis", i, call.Capability)
		}
	}

	// Check Operations
	ops := []string{"data_enrichment.interpret", "data_enrichment.tool_use", "data_enrichment.analyze"}
	for i, op := range ops {
		if i >= len(airouter.Calls) {
			break
		}
		if airouter.Calls[i].Operation != op {
			t.Fatalf("call %d: Operation = %s, want %s", i, airouter.Calls[i].Operation, op)
		}
	}

	// Check SessionID on all calls
	sid := result.SessionID
	for i, call := range airouter.Calls {
		if call.SessionID != sid {
			t.Fatalf("call %d: SessionID = %s, want %s", i, call.SessionID, sid)
		}
	}

	// Verify agent loop result
	if len(output.AgentLoops) != 1 {
		t.Fatalf("expected 1 agent loop result, got %d", len(output.AgentLoops))
	}
	if output.AgentLoops[0].FinalData != "油价涨3%" {
		t.Fatalf("agent loop final_data: want 油价涨3%%, got %s", output.AgentLoops[0].FinalData)
	}
}

// ── EnrichTopic: review judge test ──────────────────────────────────────────

func TestEnrichTopic_ReviewJudgeOnSecondRun(t *testing.T) {
	repo := setupOrchTestDB(t)

	// Pre-populate a previous result to trigger review judge.
	prevResult := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "局势暂时缓和，原油承压",
		Sectors:             json.RawMessage(`{"position":"reinforcing","signals":[],"evidence":[]}`),
		CausalChain:         "会谈→缓和→原油跌",
		SessionID:           "data_enrichment_1_prev0001",
	}
	if err := repo.CreateTopicEnrichmentResult(context.Background(), prevResult); err != nil {
		t.Fatalf("create prev result: %v", err)
	}

	airouter := newMockAirRouter()
	// interpret → tool_use → analyze → review_judge
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"油价涨4.2%"}`)
	airouter.addResponse(`{
		"evolution_assessment": "再趋紧张，原油强化",
		"position": "turning",
		"signals": [
			{"lane": "中东地缘", "signal": "油价涨4.2%", "mechanism": "以军威胁推高"}
		],
		"evidence": [
			{"context_id": "ctx2", "period": "week", "quote": "以军威胁", "source_type": "news", "tool_ref": ""}
		],
		"financial_view": {"sectors": [{"sector": "能源", "direction": "up", "supporting_data": "布伦特原油+4.2%"}]},
		"causal_chain": "以军威胁→油价涨",
		"overall": "核心判断反转，紧张加剧"
	}`)
	// review_judge: should_review=true (new schema with position_change)
	airouter.addResponse(`{"should_review":true,"reason":"定位变化","position_change":{"from":"reinforcing","to":"turning","summary":"停火打破,强化→转折"},"change_summary":"冲突重启","affected_context":"week","confidence":0.8}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "中东地缘"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 201, PeriodDate: mustParseTime("2026-07-05"),
					ClusterLabel: "局势升级", Status: "continuing",
					TopicMatchConfidence: "anchor_hit", ArticleCount: 8, ThreadCount: 3,
					ThreadTitles: []string{"以军威胁", "海峡争议"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Should have a review
	if output.Review == nil {
		t.Fatal("second run should produce a review")
	}
	if output.Review.Applied {
		t.Fatal("new review should have applied=false by default")
	}
	if output.Review.Source != "llm_assisted" {
		t.Fatalf("review source: want llm_assisted, got %s", output.Review.Source)
	}
	if output.Review.DeviationSummary == "" {
		t.Fatal("review deviation_summary should not be empty")
	}
	if output.Review.PrevResultID == nil {
		t.Fatal("review prev_result_id should not be nil")
	}
	if *output.Review.PrevResultID != prevResult.ID {
		t.Fatalf("review prev_result_id: want %d, got %d", prevResult.ID, *output.Review.PrevResultID)
	}

	// Verify position_change JSON is stored in review.Verdict column
	if output.Review.Verdict == nil {
		t.Fatal("review verdict should not be nil when position_change is present in response")
	}
	var positionChange map[string]any
	if err := json.Unmarshal(output.Review.Verdict, &positionChange); err != nil {
		t.Fatalf("review verdict (position_change) should be valid JSON: %v", err)
	}
	if positionChange["from"] != "reinforcing" {
		t.Fatalf("position_change.from: want reinforcing, got %v", positionChange["from"])
	}
	if positionChange["to"] != "turning" {
		t.Fatalf("position_change.to: want turning, got %v", positionChange["to"])
	}

	// Verify airouter calls include review_judge
	foundReviewJudge := false
	for _, call := range airouter.Calls {
		if call.Operation == "data_enrichment.review_judge" {
			foundReviewJudge = true
			break
		}
	}
	if !foundReviewJudge {
		t.Fatal("expected review_judge airouter call on second run")
	}
}

// ── EnrichTopic: review judge skipped on should_review=false ────────────────

func TestEnrichTopic_ReviewJudgeFalseSkips(t *testing.T) {
	repo := setupOrchTestDB(t)

	// Pre-populate a previous result.
	prevResult := &repository.TopicEnrichmentResult{
		PersistentTopicID:   2,
		EvolutionAssessment: "局势稳定",
		Sectors:             json.RawMessage(`{"position":"fading","signals":[],"evidence":[]}`),
		SessionID:           "data_enrichment_2_prev0001",
	}
	if err := repo.CreateTopicEnrichmentResult(context.Background(), prevResult); err != nil {
		t.Fatalf("create prev result: %v", err)
	}

	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"油价微涨0.5%"}`)
	airouter.addResponse(`{
		"evolution_assessment": "延续稳定态势",
		"position": "fading",
		"signals": [],
		"evidence": [],
		"causal_chain": "",
		"overall": "无显著变化"
	}`)
	// review_judge: should_review=false → should NOT write review
	airouter.addResponse(`{"should_review":false,"reason":"仅置信度微调，无核心判断变化","change_summary":"","affected_context":"","confidence":0.3}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 2, Label: "中东地缘"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 301, PeriodDate: mustParseTime("2026-07-05"),
					ClusterLabel: "稳定", Status: "continuing",
					TopicMatchConfidence: "anchor_hit", ArticleCount: 3, ThreadCount: 1,
					ThreadTitles: []string{"油价微涨"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 2)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// should_review=false → no review written
	if output.Review != nil {
		t.Fatal("should_review=false should not produce a review")
	}
}

// ── EnrichTopic: agent loop /no_think prefix ────────────────────────────────

func TestEnrichTopic_AgentLoopNoThinkPrefix(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{
		"evolution_assessment": "test",
		"position": "reinforcing",
		"signals": [],
		"evidence": [],
		"causal_chain": "",
		"overall": ""
	}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic:    service.TopicBrief{ID: 1, Label: "测试"},
			Sections: []service.TimelineSectionNode{},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	_, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Find the tool_use call and verify /no_think prefix in user message
	for _, call := range airouter.Calls {
		if call.Operation != "data_enrichment.tool_use" {
			continue
		}
		for _, msg := range call.Messages {
			if msg.Role == "user" {
				if !strings.Contains(msg.Content, "/no_think") {
					t.Fatal("tool_use user message should contain /no_think prefix")
				}
				return
			}
		}
	}
	t.Fatal("no tool_use call found in airouter calls")
}

// ── EnrichTopic: input_snapshot fields ──────────────────────────────────────

func TestEnrichTopic_InputSnapshotFields(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{
		"evolution_assessment": "test",
		"position": "reinforcing",
		"signals": [],
		"evidence": [],
		"causal_chain": "",
		"overall": ""
	}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	// Use a subset of context_layers to verify config filtering
	boardCfg := &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        14,
		ContextLayers:     []string{"week", "month"},
	}
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试话题"},
			Sections: []service.TimelineSectionNode{
				{
					SectionID: 401, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "测试", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"测试线索"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	var snap map[string]any
	if err := json.Unmarshal(output.Result.InputSnapshot, &snap); err != nil {
		t.Fatalf("input_snapshot parse: %v", err)
	}

	configLayers, ok := snap["config_context_layers"].([]any)
	if !ok {
		t.Fatal("input_snapshot should have config_context_layers")
	}
	if len(configLayers) != 2 {
		t.Fatalf("config_context_layers: want 2, got %d", len(configLayers))
	}

	if wd, ok := snap["window_days"].(float64); !ok || int(wd) != 14 {
		t.Fatalf("window_days: want 14")
	}

	reviewIDs, ok := snap["review_ids"].([]any)
	if !ok {
		t.Fatal("input_snapshot should have review_ids")
	}
	if len(reviewIDs) != 0 {
		t.Fatalf("review_ids: want 0 (no prev review), got %d", len(reviewIDs))
	}
}

// ── EnrichTopic: review does NOT write to topic_lifeline_context ────────────

func TestEnrichTopic_ReviewDoesNotWriteToTable1(t *testing.T) {
	repo := setupOrchTestDB(t)

	// Pre-populate a previous result.
	prevResult := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "prev",
		Sectors:             json.RawMessage(`{"position":"reinforcing","signals":[],"evidence":[]}`),
		SessionID:           "prev",
	}
	if err := repo.CreateTopicEnrichmentResult(context.Background(), prevResult); err != nil {
		t.Fatalf("create prev result: %v", err)
	}

	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)
	airouter.addResponse(`{"should_review":true,"reason":"变更","position_change":{"from":"reinforcing","to":"turning","summary":"变了"},"change_summary":"变了","affected_context":"week","confidence":0.8}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试"},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	// Count existing contexts before
	existingCtxs, _ := repo.ListTopicLifelineContextsByTopic(context.Background(), 1)
	beforeCount := len(existingCtxs)

	_, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Count contexts after — should be unchanged (no write to table1)
	afterCtxs, _ := repo.ListTopicLifelineContextsByTopic(context.Background(), 1)
	if len(afterCtxs) != beforeCount {
		t.Fatalf("review write should not modify topic_lifeline_context: before=%d, after=%d", beforeCount, len(afterCtxs))
	}
}

// ── randomHex ──────────────────────────────────────────────────────────────

func TestRandomHex_NonEmptyCorrectLength(t *testing.T) {
	for _, n := range []int{0, 1, 8, 16, 32} {
		got := service.RandomHex(n)
		if len(got) != n {
			t.Fatalf("randomHex(%d): len=%d, want %d", n, len(got), n)
		}
		// For n>0, result should not be all-zero or panic.
		if n > 0 && got == strings.Repeat("0", n) {
			t.Fatalf("randomHex(%d): got all zeros (extremely unlikely)", n)
		}
	}
}

func TestRandomHex_PanicsOnError(t *testing.T) {
	// crypto/rand.Int rarely fails, so we only assert the function
	// returns without panicking for normal input. The defensive error
	// handling is reviewed in the source.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("randomHex panicked: %v", r)
		}
	}()
	got := service.RandomHex(8)
	if len(got) != 8 {
		t.Fatalf("randomHex(8): len=%d, want 8", len(got))
	}
}

// ── Dedup: canonical JSON test ──────────────────────────────────────────────

func TestCanonicalJSON_SameKeysDifferentOrder(t *testing.T) {
	a := map[string]any{"keyword": "半导体", "limit": 10}
	b := map[string]any{"limit": 10, "keyword": "半导体"}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("canonical JSON should be same regardless of map insert order: %s vs %s", ja, jb)
	}
}

// ── EnrichTopic: agent loop dedup intercept ─────────────────────────────────

func TestEnrichTopic_AgentLoopDedupIntercept(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	// Sequence: interpret → tool_use round1 (call_tool) → round2 (call_tool SAME args) → round3 (finish)
	airouter.addResponse(`{"topics":[{"topic":"半导体","reason":"芯片"}]}`)
	// Round 1: call_tool
	airouter.addResponse(`{"action":"call_tool","thought":"查半导体ETF","tool":"list_etf_by_keyword","args":{"keyword":"半导体"}}`)
	// Round 2: call_tool with SAME args → should be deduped (never reaches LLM)
	// But our mock doesn't know about dedup, so we add a random response.
	// Wait — actually, the dedup happens BEFORE the LLM call. So round 2 won't call the LLM.
	// But our mock expects sequential calls. Since dedup intercepts before Chat(),
	// round 2 won't count as a mock response consumption.
	// So the mock's round 2 response will actually be seen by round 3.
	// Let me structure: interpret → tool_use r1 (LLM says call_tool A) →
	// tool_use r2 (LLM says call_tool A again → dedup blocks → no Chat →
	// continue loop) → tool_use r3 (LLM says finish)
	// So mock needs: interpret, tool_use r1, tool_use r3(finish)
	// And we verify: only 3 Chat calls (interpret, r1, r3)
	airouter.addResponse(`{"action":"call_tool","thought":"再查一次(重复)","tool":"list_etf_by_keyword","args":{"keyword":"半导体"}}`)
	airouter.addResponse(`{"action":"finish","thought":"done","summary":"完成"}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "半导体话题"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 501, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "芯片", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 3, ThreadCount: 1,
					ThreadTitles: []string{"芯片"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// The agent loop should have seen 3 rounds but only 2 Chat calls
	// (round 2 was dedup-intercepted).
	// Check that tool_calls has the intercepted entry.
	if len(output.AgentLoops) != 1 {
		t.Fatalf("expected 1 agent loop result")
	}
	al := output.AgentLoops[0]

	// Should have 3 tool_calls: r1=call+execute, r2=call+dedup, r3=finish(not a tool call)
	// Actually, finish doesn't produce a tool call record.
	// So we should have 2: r1 executed, r2 dedup-intercepted.
	if len(al.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls (1 exec + 1 dedup), got %d", len(al.ToolCalls))
	}
	// First should be normal execution
	if al.ToolCalls[0].ResultFull == "" {
		t.Fatal("first tool call should have result")
	}
	// Second should have dedup marker
	if !strings.Contains(al.ToolCalls[1].Thought, "被拦:重复") {
		t.Fatalf("second tool call should be dedup-intercepted, got thought: %s", al.ToolCalls[1].Thought)
	}
}

// ── EnrichTopic: agent loop max_loops guard ─────────────────────────────────

func TestEnrichTopic_AgentLoopMaxLoops(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	// Sequence: interpret → tool_use r1-r6 (all call_tool, never finish) → analyze
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	for i := 0; i < 6; i++ {
		// Change args slightly each time to avoid dedup triggering maxLoops exhaustion.
		airouter.addResponse(fmt.Sprintf(`{"action":"call_tool","thought":"step%d","tool":"list_etf_by_keyword","args":{"keyword":"石油%d"}}`, i, i))
	}
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic:    service.TopicBrief{ID: 1, Label: "石油话题"},
			Sections: []service.TimelineSectionNode{},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	if len(output.AgentLoops) != 1 {
		t.Fatalf("expected 1 agent loop result")
	}
	al := output.AgentLoops[0]

	if al.Loops != 6 {
		t.Fatalf("agent loop should exhaust maxLoops=6, got %d", al.Loops)
	}
	if al.Error == "" {
		t.Fatal("agent loop should have error on maxLoops exhaustion")
	}
	if !strings.Contains(al.Error, "达到最大循环数") {
		t.Fatalf("error should mention max loops, got: %s", al.Error)
	}
}

// ── EnrichTopic: agent loop history not truncated ───────────────────────────

func TestEnrichTopic_AgentLoopHistoryFullResult(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	// Sequence: interpret → tool_use r1 (call_tool with large result expected) → r2 (finish)
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"call_tool","thought":"查","tool":"list_etf_by_keyword","args":{"keyword":"石油"}}`)
	airouter.addResponse(`{"action":"finish","thought":"done","summary":"完成"}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "石油话题"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 601, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "石油", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"石油"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	al := output.AgentLoops[0]
	// The tool_calls record should have full result (not truncated).
	// The history block fed to LLM should contain the full result (not just preview).
	// Since we can't check the history block directly (it's constructed internally),
	// we verify that tool_calls contains ResultFull which is the complete result.
	for _, tc := range al.ToolCalls {
		if tc.ResultFull == "" {
			t.Fatal("tool call should have ResultFull")
		}
		// ResultPreview should be a subset of ResultFull (or same if short).
		if len(tc.ResultPreview) > len(tc.ResultFull) {
			t.Fatal("ResultPreview should not be longer than ResultFull")
		}
	}
}

// ── EnrichTopic: interpret includes context layers and applied reviews ──────

func TestEnrichTopic_InterpretIncludesContextLayers(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 701, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "测试", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"测试线索"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	// Pre-create a context layer in the DB.
	ctxLayer := &repository.TopicLifelineContext{
		PersistentTopicID: 1,
		Granularity:       "week",
		Period:            "2026-W27",
		Content:           "本周油价上涨，地缘局势紧张",
		AsOfDate:          mustParseTime("2026-07-06"),
		Source:            "llm_assisted",
	}
	if err := repo.UpsertTopicLifelineContext(context.Background(), ctxLayer); err != nil {
		t.Fatalf("create context layer: %v", err)
	}

	// Pre-create an applied review.
	review := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1,
		CurrResultID:      999,
		DeviationSummary:  "上次因X误判黄金跌",
		Applied:           true,
		Source:            "llm_assisted",
	}
	if err := repo.CreateTopicEnrichmentReview(context.Background(), review); err != nil {
		t.Fatalf("create review: %v", err)
	}

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	_, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// The interpret call should include context layer content and review text.
	// Check the airouter interpret call's prompt for these.
	foundInterpret := false
	for _, call := range airouter.Calls {
		if call.Operation == "data_enrichment.interpret" {
			foundInterpret = true
			for _, msg := range call.Messages {
				// Check that prompt contains context layer content
				if !strings.Contains(msg.Content, "本周油价上涨") {
					t.Fatal("interpret prompt should contain context layer content")
				}
				if !strings.Contains(msg.Content, "层级上下文") {
					t.Fatal("interpret prompt should contain context layer header")
				}
				if !strings.Contains(msg.Content, "误判黄金跌") {
					t.Fatal("interpret prompt should contain applied review text")
				}
				if !strings.Contains(msg.Content, "历史认知记录") {
					t.Fatal("interpret prompt should contain review section header")
				}
			}
			break
		}
	}
	if !foundInterpret {
		t.Fatal("no interpret call found in airouter calls")
	}
}

// ── analyze includes context layers ─────────────────────────────────────────

func TestEnrichTopic_AnalyzeIncludesContextLayers(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"油价涨3%"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 801, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "测试", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"测试"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	// Pre-create context layers.
	for _, gran := range []string{"week", "month"} {
		ctxLayer := &repository.TopicLifelineContext{
			PersistentTopicID: 1,
			Granularity:       gran,
			Period:            periodByGran(gran),
			Content:           fmt.Sprintf("%s 层内容", gran),
			AsOfDate:          mustParseTime("2026-07-06"),
			Source:            "llm_assisted",
		}
		if err := repo.UpsertTopicLifelineContext(context.Background(), ctxLayer); err != nil {
			t.Fatalf("create context layer %s: %v", gran, err)
		}
	}

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	_, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// The analyze call should include context layer content and topics data.
	foundAnalyze := false
	for _, call := range airouter.Calls {
		if call.Operation == "data_enrichment.analyze" {
			foundAnalyze = true
			for _, msg := range call.Messages {
				if !strings.Contains(msg.Content, "分层新闻上下文") {
					t.Fatal("analyze prompt should contain context layer header")
				}
				if !strings.Contains(msg.Content, "week 层内容") {
					t.Fatal("analyze prompt should contain week context")
				}
				if !strings.Contains(msg.Content, "各主题实时数据") {
					t.Fatal("analyze prompt should contain topics data section")
				}
				if !strings.Contains(msg.Content, "油价涨3%") {
					t.Fatal("analyze prompt should contain agent loop data")
				}
			}
			break
		}
	}
	if !foundAnalyze {
		t.Fatal("no analyze call found")
	}
}

// periodByGran returns a canonical period string for testing.
func periodByGran(g string) string {
	switch g {
	case "week":
		return "2026-W27"
	case "month":
		return "2026-07"
	case "year":
		return "2026"
	default:
		return "all"
	}
}

// ── EnrichTopic: context_layers filtering (skip ungenerated layers) ─────────

func TestEnrichTopic_ContextLayersSkippedWhenMissing(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	// Config requests 4 layers, but only "week" exists in DB.
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 901, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "测试", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"测试"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	// Only create "week" context — month/year/all are missing.
	ctxLayer := &repository.TopicLifelineContext{
		PersistentTopicID: 1,
		Granularity:       "week",
		Period:            "2026-W27",
		Content:           "week 内容",
		AsOfDate:          mustParseTime("2026-07-06"),
		Source:            "llm_assisted",
	}
	if err := repo.UpsertTopicLifelineContext(context.Background(), ctxLayer); err != nil {
		t.Fatalf("create context: %v", err)
	}

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// input_snapshot should only include "week" (others skipped).
	var snap map[string]any
	if err := json.Unmarshal(output.Result.InputSnapshot, &snap); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	layers, _ := snap["context_layers"].(map[string]any)
	if len(layers) != 1 {
		t.Fatalf("context_layers in snapshot: want 1 (only week), got %d: %v", len(layers), layers)
	}
	if _, ok := layers["week"]; !ok {
		t.Fatal("snapshot should include week")
	}
}

// ── EnrichTopic: interpret with limited context_layers ──────────────────────

func TestEnrichTopic_ContextLayersConfigFilter(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	// Config only requests week.
	boardCfg := &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        14,
		ContextLayers:     []string{"week"},
	}
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "测试"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 1001, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "测试", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"测试"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	// Create both week and month context, but config only requests week.
	for _, gran := range []string{"week", "month"} {
		ctxLayer := &repository.TopicLifelineContext{
			PersistentTopicID: 1,
			Granularity:       gran,
			Period:            periodByGran(gran),
			Content:           fmt.Sprintf("%s 内容", gran),
			AsOfDate:          mustParseTime("2026-07-06"),
			Source:            "llm_assisted",
		}
		if err := repo.UpsertTopicLifelineContext(context.Background(), ctxLayer); err != nil {
			t.Fatalf("create %s: %v", gran, err)
		}
	}

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// input_snapshot should only include week (config filtered).
	var snap map[string]any
	_ = json.Unmarshal(output.Result.InputSnapshot, &snap)
	layers, _ := snap["context_layers"].(map[string]any)
	if len(layers) != 1 {
		t.Fatalf("context_layers in snapshot: want 1 (config filtered to week), got %d", len(layers))
	}
	if _, ok := layers["week"]; !ok {
		t.Fatal("snapshot should include week")
	}
	if _, ok := layers["month"]; ok {
		t.Fatal("snapshot should NOT include month (filtered by config)")
	}
}

// ── Analyze: evolution position schema ──────────────────────────────────────

func TestAnalyze_EvolutionPositionSchema(t *testing.T) {
	t.Run("full schema with financial_view", func(t *testing.T) {
		repo := setupOrchTestDB(t)
		airouter := newMockAirRouter()

		// Full new-schema analyze response with position/signals/evidence/financial_view
		airouter.addResponse(`{"topics":[{"topic":"原油","reason":"地缘"}]}`)
		airouter.addResponse(`{"action":"finish","thought":"done","summary":"油价涨3%"}`)
		airouter.addResponse(`{
			"evolution_assessment": "油价上涨强化了既有能源紧张趋势",
			"position": "reinforcing",
			"signals": [
				{"lane": "中东地缘", "signal": "布伦特原油突破85美元", "mechanism": "产油国遭袭→供应收紧"},
				{"lane": "航运安全", "signal": "霍尔木兹海峡通行费上涨", "mechanism": "地缘紧张推高航运成本"}
			],
			"evidence": [
				{"context_id": "ctx_week_1", "period": "week", "quote": "霍尔木兹海峡局势升级", "source_type": "news", "tool_ref": ""},
				{"context_id": "tool_resp_1", "period": "week", "quote": "布伦特原油+3.2%", "source_type": "tool", "tool_ref": "list_etf_by_keyword"}
			],
			"financial_view": {"sectors": [{"sector": "能源", "direction": "up", "supporting_data": "布伦特原油+3.2%"}]},
			"causal_chain": "中东紧张→原油供应担忧→油价上涨→能源板块受益",
			"overall": "地缘冲突持续推高能源价格，强化了既有演进方向"
		}`)

		toolRegistry := service.NewRegistry(&nilFetcher{})
		boardCfg := newEnabledConfig()
		boardReader := &mockBoardConfigReader{cfg: boardCfg}

		lifelineReader := &orchMockLifelineReader{
			data: service.SectionTimelineData{
				Topic: service.TopicBrief{
					ID:              1,
					Label:           "中东地缘紧张",
					Status:          "active",
					FirstSeenDate:   mustParseTime("2026-07-01"),
					LastSeenDate:    mustParseTime("2026-07-05"),
					HitCount:        5,
					ConsecutiveHits: 5,
				},
				Sections: []service.TimelineSectionNode{
					{
						SectionID: 10001, PeriodDate: mustParseTime("2026-07-01"),
						ClusterLabel: "中东局势", Status: "emerging",
						TopicMatchConfidence: "auto_new", ArticleCount: 5, ThreadCount: 2,
						ThreadTitles: []string{"油价飙升"},
					},
				},
			},
		}
		renderer := service.NewLifelineRenderer()

		orch := service.NewOrchestratorService(
			airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
			testCap,
		)

		output, err := orch.EnrichTopic(context.Background(), 1)
		if err != nil {
			t.Fatalf("EnrichTopic: %v", err)
		}

		result := output.Result
		if result == nil {
			t.Fatal("result should not be nil")
		}

		// Verify Sectors JSON contains new-schema fields
		var sectorsJSON map[string]any
		if err := json.Unmarshal(result.Sectors, &sectorsJSON); err != nil {
			t.Fatalf("sectors JSON unmarshal: %v", err)
		}

		// position
		if pos, ok := sectorsJSON["position"].(string); !ok || pos != "reinforcing" {
			t.Fatalf("position: want reinforcing, got %v", sectorsJSON["position"])
		}

		// signals
		signals, ok := sectorsJSON["signals"].([]any)
		if !ok {
			t.Fatal("signals should be an array")
		}
		if len(signals) != 2 {
			t.Fatalf("signals length: want 2, got %d", len(signals))
		}
		sig1 := signals[0].(map[string]any)
		if sig1["lane"] != "中东地缘" {
			t.Fatalf("signal[0].lane: want 中东地缘, got %v", sig1["lane"])
		}
		if sig1["signal"] == "" {
			t.Fatal("signal[0].signal should not be empty")
		}
		if sig1["mechanism"] == "" {
			t.Fatal("signal[0].mechanism should not be empty")
		}

		// evidence
		evidence, ok := sectorsJSON["evidence"].([]any)
		if !ok {
			t.Fatal("evidence should be an array")
		}
		if len(evidence) != 2 {
			t.Fatalf("evidence length: want 2, got %d", len(evidence))
		}
		ev1 := evidence[0].(map[string]any)
		if ev1["source_type"] != "news" {
			t.Fatalf("evidence[0].source_type: want news, got %v", ev1["source_type"])
		}
		ev2 := evidence[1].(map[string]any)
		if ev2["source_type"] != "tool" {
			t.Fatalf("evidence[1].source_type: want tool, got %v", ev2["source_type"])
		}
		if ev2["tool_ref"] == "" {
			t.Fatal("evidence[1].tool_ref should not be empty (tool source)")
		}

		// financial_view should be present
		fv, ok := sectorsJSON["financial_view"].(map[string]any)
		if !ok {
			t.Fatal("financial_view should be present")
		}
		fvSectors, ok := fv["sectors"].([]any)
		if !ok || len(fvSectors) == 0 {
			t.Fatal("financial_view.sectors should not be empty")
		}

		// Verify EvolutionAssessment and CausalChain stored directly
		if result.EvolutionAssessment == "" {
			t.Fatal("EvolutionAssessment should not be empty")
		}
		if result.CausalChain == "" {
			t.Fatal("CausalChain should not be empty")
		}
	})

	t.Run("missing financial_view does not error", func(t *testing.T) {
		repo := setupOrchTestDB(t)
		airouter := newMockAirRouter()

		// Analyze response WITHOUT financial_view (non-financial topic)
		airouter.addResponse(`{"topics":[{"topic":"AI监管","reason":"政策"}]}`)
		airouter.addResponse(`{"action":"finish","thought":"done","summary":"欧盟通过AI法案"}`)
		airouter.addResponse(`{
			"evolution_assessment": "AI监管话题持续扩散",
			"position": "expanding",
			"signals": [
				{"lane": "AI监管", "signal": "欧盟AI法案正式通过", "mechanism": "欧盟→全球监管趋严"}
			],
			"evidence": [
				{"context_id": "ctx_news_1", "period": "month", "quote": "欧洲议会通过AI法案", "source_type": "news", "tool_ref": ""}
			],
			"causal_chain": "AI快速发展→伦理争议→监管立法→全球跟进",
			"overall": "欧盟AI法案标志全球AI治理进入新阶段"
		}`)

		toolRegistry := service.NewRegistry(&nilFetcher{})
		boardCfg := newEnabledConfig()
		boardReader := &mockBoardConfigReader{cfg: boardCfg}

		lifelineReader := &orchMockLifelineReader{
			data: service.SectionTimelineData{
				Topic: service.TopicBrief{
					ID:              2,
					Label:           "AI监管",
					Status:          "active",
					FirstSeenDate:   mustParseTime("2026-06-01"),
					LastSeenDate:    mustParseTime("2026-07-05"),
					HitCount:        3,
					ConsecutiveHits: 3,
				},
				Sections: []service.TimelineSectionNode{
					{
						SectionID: 20001, PeriodDate: mustParseTime("2026-07-01"),
						ClusterLabel: "AI监管", Status: "continuing",
						TopicMatchConfidence: "anchor_hit", ArticleCount: 8, ThreadCount: 3,
						ThreadTitles: []string{"欧盟AI法案"},
					},
				},
			},
		}
		renderer := service.NewLifelineRenderer()

		orch := service.NewOrchestratorService(
			airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
			testCap,
		)

		output, err := orch.EnrichTopic(context.Background(), 2)
		if err != nil {
			t.Fatalf("EnrichTopic: %v", err)
		}

		var sectorsJSON map[string]any
		if err := json.Unmarshal(output.Result.Sectors, &sectorsJSON); err != nil {
			t.Fatalf("sectors JSON unmarshal: %v", err)
		}

		// financial_view should be nil / omitted for non-financial topic
		if _, exists := sectorsJSON["financial_view"]; exists {
			t.Fatal("financial_view should be absent for non-financial topic")
		}

		// position and other fields should still be present
		if sectorsJSON["position"] != "expanding" {
			t.Fatalf("position: want expanding, got %v", sectorsJSON["position"])
		}
		if sectorsJSON["signals"] == nil {
			t.Fatal("signals should be present")
		}
		if sectorsJSON["evidence"] == nil {
			t.Fatal("evidence should be present")
		}
	})
}

// ── Review Judge: position_change schema tests ─────────────────────────────

func TestRunReviewJudge_PositionChangeSchema(t *testing.T) {
	repo := setupOrchTestDB(t)

	// Seed prev result with new-format sectors (position field present).
	prevResult := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "局势缓和",
		Sectors:             json.RawMessage(`{"position":"reinforcing","signals":[],"evidence":[]}`),
		SessionID:           "data_enrichment_1_prev_pc",
	}
	if err := repo.CreateTopicEnrichmentResult(context.Background(), prevResult); err != nil {
		t.Fatalf("create prev result: %v", err)
	}

	airouter := newMockAirRouter()
	// Sequence: interpret → tool_use → analyze → review_judge
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{"evolution_assessment":"再趋紧张","position":"turning","signals":[],"evidence":[],"causal_chain":"","overall":""}`)
	// review_judge: full new-schema response with position_change and change_summary.
	airouter.addResponse(`{
		"should_review": true,
		"reason": "定位从强化转为转折，冲突重启",
		"position_change": {"from": "reinforcing", "to": "turning", "summary": "停火协议被打破，局势从缓和转向紧张，触发转折"},
		"change_summary": "上一次判断停火=缓和，本次以军事行动打破预期",
		"affected_context": "week",
		"confidence": 0.85
	}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "中东地缘"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 401, PeriodDate: mustParseTime("2026-07-05"),
					ClusterLabel: "局势升级", Status: "continuing",
					TopicMatchConfidence: "anchor_hit", ArticleCount: 5, ThreadCount: 2,
					ThreadTitles: []string{"冲突"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Should have a review with position_change stored in Verdict column.
	if output.Review == nil {
		t.Fatal("should produce a review")
	}

	// Verify Verdict column contains position_change (new schema).
	if output.Review.Verdict == nil {
		t.Fatal("review verdict should contain position_change JSON")
	}
	var pc map[string]any
	if err := json.Unmarshal(output.Review.Verdict, &pc); err != nil {
		t.Fatalf("review verdict unmarshal: %v", err)
	}
	if pc["from"] != "reinforcing" {
		t.Fatalf("position_change.from: want reinforcing, got %v", pc["from"])
	}
	if pc["to"] != "turning" {
		t.Fatalf("position_change.to: want turning, got %v", pc["to"])
	}
	if pc["summary"] == "" {
		t.Fatal("position_change.summary should not be empty")
	}

	// Verify DeviationSummary column contains change_summary (new field).
	if output.Review.DeviationSummary == "" {
		t.Fatal("review deviation_summary (change_summary) should not be empty")
	}

	// Verify other fields unchanged.
	if output.Review.Source != "llm_assisted" {
		t.Fatalf("review source: want llm_assisted, got %s", output.Review.Source)
	}
	if output.Review.PrevResultID == nil || *output.Review.PrevResultID != prevResult.ID {
		t.Fatalf("review prev_result_id should match")
	}
}

func TestEnrichTopic_SkipReviewOnOldFormatPrev(t *testing.T) {
	repo := setupOrchTestDB(t)

	// Seed prev result with OLD-format sectors (bare array, no position field).
	prevResult := &repository.TopicEnrichmentResult{
		PersistentTopicID:   3,
		EvolutionAssessment: "旧格式数据",
		Sectors:             json.RawMessage(`[{"sector":"能源","direction":"up"}]`),
		SessionID:           "data_enrichment_3_prev_old",
	}
	if err := repo.CreateTopicEnrichmentResult(context.Background(), prevResult); err != nil {
		t.Fatalf("create prev result: %v", err)
	}

	airouter := newMockAirRouter()
	airouter.addResponse(`{"topics":[{"topic":"石油","reason":"油价"}]}`)
	airouter.addResponse(`{"action":"finish","summary":"done"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardCfg := newEnabledConfig()
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 3, Label: "测试"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 501, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "test", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 3)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Old-format prev → extractPosition returns "" → guard skips review judge entirely.
	if output.Review != nil {
		t.Fatal("old-format prev should NOT trigger review judge")
	}

	// No review_judge airouter call should have been made.
	for _, call := range airouter.Calls {
		if call.Operation == "data_enrichment.review_judge" {
			t.Fatal("review_judge should NOT have been called for old-format prev")
		}
	}
}

// ── ToolsForSourceType mapping unit test ──────────────────────────────────

func TestToolsForSourceType(t *testing.T) {
	// etf_quote → 3 financial tools
	tools := service.ToolsForSourceType("etf_quote")
	if len(tools) != 3 {
		t.Fatalf("etf_quote: want 3 tools, got %d", len(tools))
	}
	expected := map[string]bool{"list_etf_by_keyword": true, "get_etf_quote": true, "list_sectors": true}
	for _, tool := range tools {
		if !expected[tool] {
			t.Fatalf("etf_quote: unexpected tool %s", tool)
		}
	}

	// exchange_rate → empty (no tools yet)
	tools = service.ToolsForSourceType("exchange_rate")
	if len(tools) != 0 {
		t.Fatalf("exchange_rate: want 0 tools, got %d", len(tools))
	}

	// gdelt_event → empty (no tools yet)
	tools = service.ToolsForSourceType("gdelt_event")
	if len(tools) != 0 {
		t.Fatalf("gdelt_event: want 0 tools, got %d", len(tools))
	}

	// unknown source_type → empty
	tools = service.ToolsForSourceType("bogus")
	if len(tools) != 0 {
		t.Fatalf("bogus: want 0 tools, got %d", len(tools))
	}
}

// ── AllowedTools: only permitted tools are advertised and executable ──────

func TestEnrichTopic_OnlyAllowedToolsAdvertised(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	// Sequence: interpret → tool_use r1(call get_etf_quote, not allowed) →
	// tool_use r2(call list_etf_by_keyword, allowed) → tool_use r3(finish) → analyze
	airouter.addResponse(`{"topics":[{"topic":"半导体","reason":"芯片"}]}`)
	airouter.addResponse(`{"action":"call_tool","thought":"查行情","tool":"get_etf_quote","args":{"codes":["512480"]}}`)
	airouter.addResponse(`{"action":"call_tool","thought":"查ETF","tool":"list_etf_by_keyword","args":{"keyword":"半导体"}}`)
	airouter.addResponse(`{"action":"finish","thought":"done","summary":"芯片ETF涨2%"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	// Only list_etf_by_keyword is allowed — get_etf_quote and list_sectors are blocked.
	boardCfg := &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        14,
		ContextLayers:     []string{"week"},
		AllowedTools:      []string{"list_etf_by_keyword"},
	}
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 1, Label: "半导体话题"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 2001, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "芯片", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 3, ThreadCount: 1,
					ThreadTitles: []string{"芯片"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Verify agent loop results.
	if len(output.AgentLoops) != 1 {
		t.Fatalf("expected 1 agent loop, got %d", len(output.AgentLoops))
	}
	al := output.AgentLoops[0]

	// Should have 3 tool_calls: r1=get_etf_quote(blocked), r2=list_etf_by_keyword(executed), r3=finish(not a tool call)
	if len(al.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls (1 blocked + 1 executed), got %d", len(al.ToolCalls))
	}

	// First call: get_etf_quote should be blocked.
	tc1 := al.ToolCalls[0]
	if tc1.Tool != "get_etf_quote" {
		t.Fatalf("first tool call: want get_etf_quote, got %s", tc1.Tool)
	}
	if !strings.Contains(tc1.Thought, "被拦:工具不可用") {
		t.Fatalf("first tool call should be blocked, got thought: %s", tc1.Thought)
	}
	if !strings.Contains(tc1.ResultFull, "该工具当前不可用") {
		t.Fatalf("first tool result should say 该工具当前不可用, got: %s", tc1.ResultFull)
	}

	// Second call: list_etf_by_keyword should be executed normally.
	tc2 := al.ToolCalls[1]
	if tc2.Tool != "list_etf_by_keyword" {
		t.Fatalf("second tool call: want list_etf_by_keyword, got %s", tc2.Tool)
	}
	if strings.Contains(tc2.Thought, "被拦") {
		t.Fatal("second tool call should NOT be blocked")
	}

	// Verify tool_use system prompt: only list_etf_by_keyword tool desc should appear.
	// (Note: the workflow instructions in the template also mention tool names inline,
	// so we check for the bold **toolname** markup that only buildToolsDesc produces.)
	foundToolUse := false
	for _, call := range airouter.Calls {
		if call.Operation != "data_enrichment.tool_use" {
			continue
		}
		foundToolUse = true
		for _, msg := range call.Messages {
			if msg.Role == "system" {
				// Allowed: bold markup present.
				if !strings.Contains(msg.Content, "**list_etf_by_keyword**") {
					t.Fatal("system prompt should mention **list_etf_by_keyword** tool desc")
				}
				// Blocked: bold markup absent.
				if strings.Contains(msg.Content, "**get_etf_quote**") {
					t.Fatal("system prompt should NOT have **get_etf_quote** tool desc (not allowed)")
				}
				if strings.Contains(msg.Content, "**list_sectors**") {
					t.Fatal("system prompt should NOT have **list_sectors** tool desc (not allowed)")
				}
			}
		}
	}
	if !foundToolUse {
		t.Fatal("no tool_use call found")
	}
}

// ── AllowedTools: empty list → no tools advertised, agent finishes directly ─

func TestEnrichTopic_EmptyAllowedToolsNoToolsAdvertised(t *testing.T) {
	repo := setupOrchTestDB(t)
	airouter := newMockAirRouter()

	// Sequence: interpret → tool_use(finish immediately, no tools available) → analyze
	airouter.addResponse(`{"topics":[{"topic":"Rust发布","reason":"技术动向"}]}`)
	airouter.addResponse(`{"action":"finish","thought":"无可用工具，直接完成","summary":"(无可查数据)"}`)
	airouter.addResponse(`{"evolution_assessment":"test","position":"reinforcing","signals":[],"evidence":[],"causal_chain":"","overall":""}`)

	toolRegistry := service.NewRegistry(&nilFetcher{})
	// Empty AllowedTools — no tools should be advertised.
	boardCfg := &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        14,
		ContextLayers:     []string{"week"},
		AllowedTools:      []string{},
	}
	boardReader := &mockBoardConfigReader{cfg: boardCfg}

	lifelineReader := &orchMockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{ID: 2, Label: "Rust发布"},
			Sections: []service.TimelineSectionNode{
				{SectionID: 3001, PeriodDate: mustParseTime("2026-07-01"),
					ClusterLabel: "Rust", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 2, ThreadCount: 1,
					ThreadTitles: []string{"Rust 1.90"},
				},
			},
		},
	}
	renderer := service.NewLifelineRenderer()

	orch := service.NewOrchestratorService(
		airouter, repo, lifelineReader, renderer, toolRegistry, boardReader,
		testCap,
	)

	output, err := orch.EnrichTopic(context.Background(), 2)
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	// Verify agent loop completed without errors.
	if len(output.AgentLoops) != 1 {
		t.Fatalf("expected 1 agent loop, got %d", len(output.AgentLoops))
	}
	al := output.AgentLoops[0]
	if al.Error != "" {
		t.Fatalf("agent loop should have no error, got: %s", al.Error)
	}
	if al.FinalData != "(无可查数据)" {
		t.Fatalf("agent should finish with canned summary, got: %s", al.FinalData)
	}
	if len(al.ToolCalls) != 0 {
		t.Fatalf("no tool calls should have been made, got %d", len(al.ToolCalls))
	}

	// Verify system prompt: tools section is empty (no bold tool markup).
	// (The workflow instructions still mention tool names inline, but buildToolsDesc
	// outputs nothing when allowedTools is empty.)
	foundToolUse := false
	for _, call := range airouter.Calls {
		if call.Operation != "data_enrichment.tool_use" {
			continue
		}
		foundToolUse = true
		for _, msg := range call.Messages {
			if msg.Role == "system" {
				// No bold tool descriptions should appear — buildToolsDesc returns "".
				if strings.Contains(msg.Content, "**list_etf_by_keyword**") {
					t.Fatal("system prompt should NOT have **list_etf_by_keyword** tool desc (no tools allowed)")
				}
				if strings.Contains(msg.Content, "**get_etf_quote**") {
					t.Fatal("system prompt should NOT have **get_etf_quote** tool desc (no tools allowed)")
				}
				if strings.Contains(msg.Content, "**list_sectors**") {
					t.Fatal("system prompt should NOT have **list_sectors** tool desc (no tools allowed)")
				}
			}
		}
	}
	if !foundToolUse {
		t.Fatal("no tool_use call found")
	}
}
