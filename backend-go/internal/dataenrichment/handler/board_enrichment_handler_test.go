package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/handler"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
)

// ── 版块级分析 API（tasks 4.1 / M6）──────────────────────────────────────────

func newBoardAnalysisRouter(t *testing.T, orch handler.Orchestrator, db *gorm.DB) *gin.Engine {
	t.Helper()
	repo := repository.NewRepository(db)
	gin.SetMode(gin.TestMode)
	h := handler.NewHandler(repo, nil, orch, &enabledBoardConfigReader{}, nil, nil, db)
	r := gin.New()
	h.RegisterRoutes(&r.RouterGroup)
	return r
}

// enabledBoardConfigReader always reports enrichment enabled (topic trigger
// gate passes; board trigger has its own resolver inside the orchestrator).
type enabledBoardConfigReader struct{}

func (enabledBoardConfigReader) GetBoardConfig(ctx context.Context, topicID uint) (*service.BoardEnrichmentConfig, error) {
	cfg := service.DefaultBoardConfig()
	cfg.EnrichmentEnabled = true
	return cfg, nil
}

func jsonContains(body, needle string) bool { return strings.Contains(body, needle) }

func itoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }

func bodyReader(s string) io.Reader { return strings.NewReader(s) }

func boardResultRow(t *testing.T, db *gorm.DB, boardID uint, thesis string) *repository.TopicEnrichmentResult {
	t.Helper()
	repo := repository.NewRepository(db)
	res := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID),
		AnalysisScope:   "board",
		Sectors:         json.RawMessage(`{"scope":"board","thesis":"` + thesis + `"}`),
		SessionID:       "data_enrichment_board_" + thesis,
	}
	if err := repo.CreateTopicEnrichmentResult(context.Background(), res); err != nil {
		t.Fatalf("seed board result: %v", err)
	}
	return res
}

// M6.1 未开启板块 → 4xx 可区分错误。
func TestBoardEnrichmentRoutes_DisabledBoardRejected(t *testing.T) {
	db := setupHandlerTestDB(t)
	orch := &mockOrchestrator{boardErr: fmt.Errorf("enrich board 5: enrichment not enabled for this board")}
	r := newBoardAnalysisRouter(t, orch, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/semantic-boards/5/enrichment/analysis/trigger", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("disabled board: want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !jsonContains(w.Body.String(), "not enabled") {
		t.Fatalf("error must be distinguishable: %s", w.Body.String())
	}
}

// M6.2 异步触发：立即返回 started，后台跑完落库，status 轮询拿 result_id，
// 详情携带 sectors 五字段（fix-board-analysis-material 8.x 重写）。
func TestBoardEnrichmentRoutes_Trigger(t *testing.T) {
	db := setupHandlerTestDB(t)
	res := boardResultRow(t, db, 5, "命题甲")
	orch := &mockOrchestrator{boardOut: &service.BoardEnrichmentOutput{Result: res}}
	r := newBoardAnalysisRouter(t, orch, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/semantic-boards/5/enrichment/analysis/trigger", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !jsonContains(w.Body.String(), `"started"`) {
		t.Fatalf("trigger: want 200 started, got %d body=%s", w.Code, w.Body.String())
	}

	st := pollBoardAnalysisStatus(t, r, 5)
	if errStr, _ := st["error"].(string); errStr != "" {
		t.Fatalf("background analysis failed: %s", errStr)
	}
	gotID, _ := st["result_id"].(float64)
	if uint(gotID) != res.ID {
		t.Fatalf("result_id = %v, want %d", st["result_id"], res.ID)
	}

	// Detail carries the five-sector payload.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/semantic-boards/5/enrichment/analysis/results/"+itoa(res.ID), nil)
	r.ServeHTTP(w2, req2)
	for _, want := range []string{`"analysis_scope"`, `"sectors"`, `"thesis"`, `"session_id"`} {
		if !jsonContains(w2.Body.String(), want) {
			t.Fatalf("detail missing %s: %s", want, w2.Body.String())
		}
	}
}

// M8.x 防重入：同板块在跑时重复触发 → 409；跑完后可再触发。
func TestBoardEnrichmentRoutes_AlreadyRunning(t *testing.T) {
	db := setupHandlerTestDB(t)
	block := make(chan struct{})
	orch := &mockOrchestrator{block: block}
	r := newBoardAnalysisRouter(t, orch, db)

	do := func() int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/semantic-boards/5/enrichment/analysis/trigger", nil)
		r.ServeHTTP(w, req)
		return w.Code
	}
	if code := do(); code != http.StatusOK {
		t.Fatalf("first trigger: want 200, got %d", code)
	}
	if code := do(); code != http.StatusConflict {
		t.Fatalf("second trigger while running: want 409, got %d", code)
	}
	close(block)
	pollBoardAnalysisStatus(t, r, 5)
	if code := do(); code != http.StatusOK {
		t.Fatalf("re-trigger after finish: want 200, got %d", code)
	}
}

// pollBoardAnalysisStatus polls the async status endpoint (no /api prefix in
// this router) until the board job finishes.
func pollBoardAnalysisStatus(t *testing.T, r *gin.Engine, boardID uint) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/enrichment/analysis-status?scope=board&id=%d", boardID), nil)
		r.ServeHTTP(w, req)
		var resp struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if finished, _ := resp.Data["finished"].(bool); finished {
			return resp.Data
		}
		if time.Now().After(deadline) {
			t.Fatalf("board analysis not finished in 5s: %v", resp.Data)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// M6.3 列表 scope 隔离：board 档列表不含 topic 档；详情拒绝他板块 result。
func TestBoardEnrichmentRoutes_ListAndDetail(t *testing.T) {
	db := setupHandlerTestDB(t)
	b1 := boardResultRow(t, db, 5, "命题一")
	boardResultRow(t, db, 6, "命题二") // other board — must not leak
	// topic-scope row for board 5's lane — must not leak either.
	topicRes := &repository.TopicEnrichmentResult{
		PersistentTopicID: repository.TopicIDPtr(501),
		AnalysisScope:     "topic",
		Sectors:           json.RawMessage(`{"form":"sparse"}`),
		SessionID:         "topic-scope",
	}
	if err := repository.NewRepository(db).CreateTopicEnrichmentResult(context.Background(), topicRes); err != nil {
		t.Fatalf("seed topic result: %v", err)
	}
	r := newBoardAnalysisRouter(t, &mockOrchestrator{}, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/semantic-boards/5/enrichment/analysis/results", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	body := w.Body.String()
	if !jsonContains(body, "命题一") {
		t.Fatal("own board result must be listed")
	}
	if jsonContains(body, "命题二") || jsonContains(body, "topic-scope") {
		t.Fatalf("foreign/topic-scope results leaked: %s", body)
	}

	// Detail: own result OK.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/semantic-boards/5/enrichment/analysis/results/"+itoa(b1.ID), nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("detail own: %d", w2.Code)
	}

	// Detail: foreign board's result → 404.
	foreign := boardResultRow(t, db, 6, "命题二b")
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/semantic-boards/5/enrichment/analysis/results/"+itoa(foreign.ID), nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("detail foreign: want 404, got %d", w3.Code)
	}
}

// M6.4 prefill_lens：单泳道 trigger body 可选字段透传（异步化重写：错误进
// status，透传断言不变）。
func TestTopicTrigger_PrefillLens(t *testing.T) {
	db := setupHandlerTestDB(t)
	orch := &mockOrchestrator{shouldFail: true} // failure lands in status.error
	r := newBoardAnalysisRouter(t, orch, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/persistent-topics/501/enrichment/results/trigger", bodyReader(`{"prefill_lens":"供需错配"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: want 200 started, got %d", w.Code)
	}

	st := pollTopicAnalysisStatus(t, r, 501)
	if errStr, _ := st["error"].(string); errStr == "" {
		t.Fatal("failing orchestrator must surface in status.error")
	}
	if orch.lastTopicID != 501 {
		t.Fatalf("prefill_lens body must not break topic resolution, got %d", orch.lastTopicID)
	}
	if orch.lastLens != "供需错配" {
		t.Fatalf("prefill_lens must reach the orchestrator, got %q", orch.lastLens)
	}
}

func pollTopicAnalysisStatus(t *testing.T, r *gin.Engine, topicID uint) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/enrichment/analysis-status?scope=topic&id=%d", topicID), nil)
		r.ServeHTTP(w, req)
		var resp struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if finished, _ := resp.Data["finished"].(bool); finished {
			return resp.Data
		}
		if time.Now().After(deadline) {
			t.Fatalf("topic analysis not finished in 5s: %v", resp.Data)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
