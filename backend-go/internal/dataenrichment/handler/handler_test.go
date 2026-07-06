package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/handler"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/platform/database"
)

// ── Test helpers ────────────────────────────────────────────────────────────

func setupHandlerTestDB(t *testing.T) *gorm.DB {
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
	return db
}

// alwaysEnabledBoardConfig returns enrichment enabled for any topic.
type alwaysEnabledBoardConfig struct{}

func (c *alwaysEnabledBoardConfig) GetBoardConfig(ctx context.Context, topicID uint) (*service.BoardEnrichmentConfig, error) {
	return &service.BoardEnrichmentConfig{
		EnrichmentEnabled: true,
		WindowDays:        14,
		ContextLayers:     []string{"week", "month", "year", "all"},
	}, nil
}

// alwaysDisabledBoardConfig returns enrichment disabled for any topic.
type alwaysDisabledBoardConfig struct{}

func (c *alwaysDisabledBoardConfig) GetBoardConfig(ctx context.Context, topicID uint) (*service.BoardEnrichmentConfig, error) {
	return service.DefaultBoardConfig(), nil
}

// mockLifelineService is a mock for handler.LifelineService that records calls.
type mockLifelineService struct {
	lastTopicID     uint
	lastGranularity string
	shouldFail      bool
}

func (m *mockLifelineService) RefreshGranularity(ctx context.Context, topicID uint, granularity string, now time.Time) error {
	m.lastTopicID = topicID
	m.lastGranularity = granularity
	if m.shouldFail {
		return fmt.Errorf("mock refresh error")
	}
	return nil
}

// mockOrchestrator is a mock for handler.Orchestrator.
type mockOrchestrator struct {
	lastTopicID uint
	shouldFail  bool
	output      *service.EnrichmentOutput
}

func (m *mockOrchestrator) EnrichTopic(ctx context.Context, topicID uint) (*service.EnrichmentOutput, error) {
	m.lastTopicID = topicID
	if m.shouldFail {
		return nil, fmt.Errorf("mock enrich error")
	}
	return m.output, nil
}

func newTestHandler(db *gorm.DB, lifelineSvc handler.LifelineService, orch handler.Orchestrator, cfg service.BoardConfigReader) *handler.EnrichmentHandler {
	return handler.NewHandler(repository.Repo, lifelineSvc, orch, cfg, db)
}

// newTestRouter creates a Gin engine with the handler routes registered.
func newTestRouter(h *handler.EnrichmentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h.RegisterRoutes(engine.Group("/api"))
	return engine
}

// doRequest is a convenience helper for test HTTP requests.
func doRequest(t *testing.T, r *gin.Engine, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// expectJSONSuccess asserts the response has success=true and parses data into v.
func expectJSONSuccess(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got error=%q", resp.Error)
	}
	if v != nil {
		if err := json.Unmarshal(resp.Data, v); err != nil {
			t.Fatalf("parse data: %v", err)
		}
	}
}

// expectJSONError asserts the response has success=false with the given status.
func expectJSONError(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("expected %d, got %d: %s", status, w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if s, ok := resp["success"].(bool); ok && s {
		t.Fatal("expected success=false")
	}
	return resp
}

// ── Route smoke test ────────────────────────────────────────────────────────

func TestRoutesRegisterWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	// Use a handler with all nil dependencies — route registration shouldn't
	// call any service method, it just registers path patterns.
	h := &handler.EnrichmentHandler{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterRoutes panicked: %v", r)
		}
	}()
	h.RegisterRoutes(engine.Group("/api"))
}

// ── Context CRUD tests ──────────────────────────────────────────────────────

func TestListContexts(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	// Seed a context.
	lc := &repository.TopicLifelineContext{
		PersistentTopicID: 1, Granularity: "week", Content: "week summary",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/contexts", "")
	var list []repository.TopicLifelineContext
	expectJSONSuccess(t, w, &list)
	if len(list) != 1 {
		t.Fatalf("list count = %d, want 1", len(list))
	}
	if list[0].Granularity != "week" {
		t.Fatalf("granularity = %q, want week", list[0].Granularity)
	}
}

func TestGetContext(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	lc := &repository.TopicLifelineContext{
		PersistentTopicID: 1, Granularity: "month", Content: "month summary",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/contexts/month", "")
	var got repository.TopicLifelineContext
	expectJSONSuccess(t, w, &got)
	if got.Content != "month summary" {
		t.Fatalf("content = %q, want 'month summary'", got.Content)
	}
}

func TestGetContextNotFound(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/contexts/week", "")
	expectJSONError(t, w, http.StatusNotFound)
}

func TestGetContextInvalidGranularity(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/contexts/fake", "")
	expectJSONError(t, w, http.StatusBadRequest)
}

func TestUpdateContext(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	lc := &repository.TopicLifelineContext{
		PersistentTopicID: 1, Granularity: "week", Content: "original content",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	body := `{"content": "edited content"}`
	w := doRequest(t, r, "PUT", "/api/persistent-topics/1/enrichment/contexts/week", body)
	var got repository.TopicLifelineContext
	expectJSONSuccess(t, w, &got)
	if got.Content != "edited content" {
		t.Fatalf("content = %q, want 'edited content'", got.Content)
	}
	if got.Source != "manual" {
		t.Fatalf("source = %q, want 'manual'", got.Source)
	}
}

func TestRegenerateContext(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	// Seed a context for the regenerate to overwrite.
	lc := &repository.TopicLifelineContext{
		PersistentTopicID: 1, Granularity: "week", Content: "old content",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	mockLifeline := &mockLifelineService{}
	h := newTestHandler(db, mockLifeline, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "POST", "/api/persistent-topics/1/enrichment/contexts/week/regenerate", "")
	expectJSONSuccess(t, w, nil)

	if mockLifeline.lastTopicID != 1 || mockLifeline.lastGranularity != "week" {
		t.Fatalf("RefreshGranularity called with (%d, %s), want (1, week)",
			mockLifeline.lastTopicID, mockLifeline.lastGranularity)
	}
}

// ── Result CRUD tests ───────────────────────────────────────────────────────

func TestListResults(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	r1 := &repository.TopicEnrichmentResult{
		PersistentTopicID: 1, EvolutionAssessment: "first", SessionID: "s1",
	}
	r2 := &repository.TopicEnrichmentResult{
		PersistentTopicID: 1, EvolutionAssessment: "second", SessionID: "s2",
	}
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r1)
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r2)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/results", "")
	var list []map[string]any
	expectJSONSuccess(t, w, &list)
	if len(list) != 2 {
		t.Fatalf("list count = %d, want 2", len(list))
	}
	// First item should have the largest ID (newest first).
	id1, _ := list[0]["id"].(float64)
	id2, _ := list[1]["id"].(float64)
	if id1 <= id2 {
		t.Fatal("expected descending order by id")
	}
}

func TestGetResult(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	result := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "test assessment",
		SessionID:           "session-123",
	}
	if err := repository.Repo.CreateTopicEnrichmentResult(ctx, result); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/persistent-topics/1/enrichment/results/%d", result.ID), "")
	var got map[string]any
	expectJSONSuccess(t, w, &got)
	if id, _ := got["id"].(float64); uint(id) != result.ID {
		t.Fatalf("id = %v, want %d", id, result.ID)
	}
	if sid, _ := got["session_id"].(string); sid != "session-123" {
		t.Fatalf("session_id = %q, want 'session-123'", sid)
	}
}

func TestGetResultNotFound(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/results/99999", "")
	expectJSONError(t, w, http.StatusNotFound)
}

func TestGetResultIDORProtection(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	// Create result for topic 1.
	result1 := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "topic 1 result",
		SessionID:           "session-1",
	}
	if err := repository.Repo.CreateTopicEnrichmentResult(ctx, result1); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	// Cross-topic access: topic 2 tries to read topic 1's result → 404.
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/persistent-topics/2/enrichment/results/%d", result1.ID), "")
	expectJSONError(t, w, http.StatusNotFound)

	// Same-topic access → 200.
	w2 := doRequest(t, r, "GET", fmt.Sprintf("/api/persistent-topics/1/enrichment/results/%d", result1.ID), "")
	var got map[string]any
	expectJSONSuccess(t, w2, &got)
	if id, _ := got["id"].(float64); uint(id) != result1.ID {
		t.Fatalf("id = %v, want %d", id, result1.ID)
	}
}

func TestTriggerEnrichmentSuccess(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	// Seed a result that the orchestrator mock will "create".
	result := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "mock enrichment result",
		SessionID:           "mock-session",
	}
	if err := repository.Repo.CreateTopicEnrichmentResult(ctx, result); err != nil {
		t.Fatalf("seed: %v", err)
	}

	mockOrch := &mockOrchestrator{
		output: &service.EnrichmentOutput{
			Result: result,
		},
	}
	h := newTestHandler(db, nil, mockOrch, &alwaysEnabledBoardConfig{})
	r := newTestRouter(h)

	w := doRequest(t, r, "POST", "/api/persistent-topics/1/enrichment/results/trigger", "")
	expectJSONSuccess(t, w, nil)

	if mockOrch.lastTopicID != 1 {
		t.Fatalf("EnrichTopic called with %d, want 1", mockOrch.lastTopicID)
	}
}

func TestTriggerEnrichmentDisabled(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, &alwaysDisabledBoardConfig{})
	r := newTestRouter(h)

	w := doRequest(t, r, "POST", "/api/persistent-topics/1/enrichment/results/trigger", "")
	resp := expectJSONError(t, w, http.StatusBadRequest)
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(errMsg, "not enabled") {
		t.Fatalf("expected 'not enabled' error, got %q", errMsg)
	}
}

// ── Review CRUD tests ───────────────────────────────────────────────────────

func TestListReviews(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	rv1 := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 10, DeviationSummary: "review 1",
	}
	rv2 := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 20, DeviationSummary: "review 2",
	}
	_ = repository.Repo.CreateTopicEnrichmentReview(ctx, rv1)
	_ = repository.Repo.CreateTopicEnrichmentReview(ctx, rv2)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/persistent-topics/1/enrichment/reviews", "")
	var list []repository.TopicEnrichmentReview
	expectJSONSuccess(t, w, &list)
	if len(list) != 2 {
		t.Fatalf("list count = %d, want 2", len(list))
	}
}

func TestUpdateReviewDeviation(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	rv := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 10, DeviationSummary: "original",
	}
	if err := repository.Repo.CreateTopicEnrichmentReview(ctx, rv); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	body := `{"deviation_summary": "updated deviation"}`
	w := doRequest(t, r, "PUT", fmt.Sprintf("/api/persistent-topics/1/enrichment/reviews/%d", rv.ID), body)
	var got repository.TopicEnrichmentReview
	expectJSONSuccess(t, w, &got)
	if got.DeviationSummary != "updated deviation" {
		t.Fatalf("deviation = %q, want 'updated deviation'", got.DeviationSummary)
	}
}

func TestApplyReview(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	rv := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 10, DeviationSummary: "needs apply", Applied: false,
	}
	if err := repository.Repo.CreateTopicEnrichmentReview(ctx, rv); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "POST", fmt.Sprintf("/api/persistent-topics/1/enrichment/reviews/%d/apply", rv.ID), "")
	var got repository.TopicEnrichmentReview
	expectJSONSuccess(t, w, &got)
	if !got.Applied {
		t.Fatal("expected applied=true after apply")
	}

	// Verify context table was NOT modified (design §4.3: no write-back to table 1).
	// This is an architectural invariant tested here.
	// Context table is empty — if apply wrote back, there'd be content.
	ctxList, _ := repository.Repo.ListTopicLifelineContextsByTopic(ctx, 1)
	if len(ctxList) > 0 {
		t.Fatal("applyReview should NOT write back to table 1 (topic_lifeline_context)")
	}
}

func TestCreateManualReview(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	body := `{"curr_result_id": 1, "deviation_summary": "manual annotation"}`
	w := doRequest(t, r, "POST", "/api/persistent-topics/1/enrichment/reviews", body)

	var got repository.TopicEnrichmentReview
	expectJSONSuccess(t, w, &got)
	if got.Source != "manual" {
		t.Fatalf("source = %q, want 'manual'", got.Source)
	}
	if !got.Applied {
		t.Fatal("expected applied=true for manual reviews")
	}
	if got.DeviationSummary != "manual annotation" {
		t.Fatalf("deviation = %q, want 'manual annotation'", got.DeviationSummary)
	}
}

func TestCreateManualReviewWithPrevResultID(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	prevID := uint(5)
	body := fmt.Sprintf(`{"curr_result_id": 2, "deviation_summary": "with prev", "prev_result_id": %d}`, prevID)
	w := doRequest(t, r, "POST", "/api/persistent-topics/1/enrichment/reviews", body)

	var got repository.TopicEnrichmentReview
	expectJSONSuccess(t, w, &got)
	if got.PrevResultID == nil || *got.PrevResultID != 5 {
		t.Fatalf("prev_result_id = %v, want 5", got.PrevResultID)
	}
}

func TestCreateManualReviewPrevResultIDOmittable(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	body := `{"curr_result_id": 3, "deviation_summary": "no prev"}`
	w := doRequest(t, r, "POST", "/api/persistent-topics/1/enrichment/reviews", body)

	var got repository.TopicEnrichmentReview
	expectJSONSuccess(t, w, &got)
	if got.PrevResultID != nil {
		t.Fatalf("prev_result_id should be nil when omitted, got %v", *got.PrevResultID)
	}
}

// ── Board data source tests ─────────────────────────────────────────────────

func TestListDataSources(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	ds1 := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote", Enabled: true}
	ds2 := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "exchange_rate", Enabled: true}
	_ = repository.Repo.CreateBoardDataSource(ctx, ds1)
	_ = repository.Repo.CreateBoardDataSource(ctx, ds2)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "GET", "/api/semantic-boards/1/data-sources", "")
	var list []repository.BoardDataSource
	expectJSONSuccess(t, w, &list)
	if len(list) != 2 {
		t.Fatalf("list count = %d, want 2", len(list))
	}
}

func TestUpsertDataSource(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	body := `{"source_type": "etf_quote", "config": {"keywords": ["半导体"]}, "enabled": true}`
	w := doRequest(t, r, "PUT", "/api/semantic-boards/1/data-sources", body)
	var ds repository.BoardDataSource
	expectJSONSuccess(t, w, &ds)

	// Verify persisted.
	got, err := repository.Repo.GetBoardDataSourceByBoardAndType(ctx, 1, "etf_quote")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.Enabled {
		t.Fatal("expected enabled=true")
	}
}

func TestUpsertDataSourceUniqueConstraint(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	// Pre-create.
	ds := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote", Enabled: true}
	_ = repository.Repo.CreateBoardDataSource(ctx, ds)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	// Upsert should succeed (update not insert).
	body := `{"source_type": "etf_quote", "enabled": false}`
	w := doRequest(t, r, "PUT", "/api/semantic-boards/1/data-sources", body)
	var updated repository.BoardDataSource
	expectJSONSuccess(t, w, &updated)
	if updated.Enabled {
		t.Fatal("expected enabled=false after upsert update")
	}
}

func TestDeleteDataSource(t *testing.T) {
	db := setupHandlerTestDB(t)
	ctx := context.Background()

	ds := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote", Enabled: true}
	if err := repository.Repo.CreateBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "DELETE", "/api/semantic-boards/1/data-sources/etf_quote", "")
	var got map[string]any
	expectJSONSuccess(t, w, &got)
	if d, _ := got["deleted"].(bool); !d {
		t.Fatal("expected deleted=true")
	}

	// Verify deleted.
	_, err := repository.Repo.GetBoardDataSourceByBoardAndType(ctx, 1, "etf_quote")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteDataSourceNotFound(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	w := doRequest(t, r, "DELETE", "/api/semantic-boards/1/data-sources/nonexistent", "")
	expectJSONError(t, w, http.StatusNotFound)
}

// ── Invalid param tests ─────────────────────────────────────────────────────

func TestInvalidTopicID(t *testing.T) {
	db := setupHandlerTestDB(t)

	h := newTestHandler(db, nil, nil, nil)
	r := newTestRouter(h)

	tests := []struct{ path string }{
		{"/api/persistent-topics/abc/enrichment/contexts"},
		{"/api/persistent-topics/0/enrichment/contexts"},
		{"/api/persistent-topics/abc/enrichment/results"},
		{"/api/persistent-topics/abc/enrichment/reviews"},
	}
	for _, tt := range tests {
		w := doRequest(t, r, "GET", tt.path, "")
		expectJSONError(t, w, http.StatusBadRequest)
	}
}
