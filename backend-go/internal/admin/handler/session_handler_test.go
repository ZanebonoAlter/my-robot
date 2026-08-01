package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/tracing"
)

// setupSessionTestDB mirrors setupCallLogTestDB but also migrates the
// otel_spans table, since the session endpoint aggregates spans + call logs.
func setupSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AICallLog{}, &tracing.OtelSpan{}))
	database.DB = db
	repository.InitRepository(database.DB)
	return db
}

func newSessionContext(sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "session_id", Value: sessionID}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return ctx, recorder
}

// TestGetSession_EmptyState verifies the no-data path returns success=true with
// empty arrays (NOT 404), per design §1.4.
func TestGetSession_EmptyState(t *testing.T) {
	setupSessionTestDB(t)

	ctx, recorder := newSessionContext("no-such-session")
	GetSession(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])

	data := body["data"].(map[string]interface{})
	require.Equal(t, "no-such-session", data["session_id"])
	require.Empty(t, data["call_logs"].([]interface{}))
	require.Empty(t, data["timeline"].([]interface{}))

	summary := data["summary"].(map[string]interface{})
	require.Equal(t, float64(0), summary["call_count"])
	require.Equal(t, float64(0), summary["span_count"])
	require.Equal(t, float64(0), summary["error_count"])
	require.Nil(t, summary["started_at"])
	require.Nil(t, summary["ended_at"])
	tokens := summary["total_tokens"].(map[string]interface{})
	require.Equal(t, float64(0), tokens["prompt"])
	require.Equal(t, float64(0), tokens["completion"])
	require.Equal(t, float64(0), tokens["total"])
}

// TestGetSession_AggregatesCallLogsAndTimeline verifies call_logs + spans are
// joined via trace_id, the timeline tree is assembled (parent→child), and the
// shared trace_id is de-duplicated (span_count reflects actual spans, not 2x).
func TestGetSession_AggregatesCallLogsAndTimeline(t *testing.T) {
	db := setupSessionTestDB(t)
	const sessionID = "sess-aggregate"
	const traceID = "trace-aaa"

	// Two call logs sharing ONE trace_id (dedup must query the trace once).
	now := time.Now()
	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "daily_report.cluster_tags", SessionID: sessionID, TraceID: traceID,
		Capability: "summary", Success: true, LatencyMs: 100, CreatedAt: now,
	}).Error)
	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "daily_report.highlights", SessionID: sessionID, TraceID: traceID,
		Capability: "summary", Success: true, LatencyMs: 200, CreatedAt: now.Add(time.Second),
	}).Error)

	// Parent + child span on the same trace.
	require.NoError(t, db.Create(&tracing.OtelSpan{
		TraceID: traceID, SpanID: "span-root", ParentSpanID: "",
		Name:              "workflow.daily_report.generate",
		StartTimeUnixNano: now.Add(-500 * time.Millisecond).UnixNano(),
		EndTimeUnixNano:   now.Add(5 * time.Second).UnixNano(),
	}).Error)
	require.NoError(t, db.Create(&tracing.OtelSpan{
		TraceID: traceID, SpanID: "span-child", ParentSpanID: "span-root",
		Name:              "workflow.daily_report.cluster_tags",
		StartTimeUnixNano: now.UnixNano(),
		EndTimeUnixNano:   now.Add(2 * time.Second).UnixNano(),
	}).Error)

	ctx, recorder := newSessionContext(sessionID)
	GetSession(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])

	data := body["data"].(map[string]interface{})
	require.Equal(t, sessionID, data["session_id"])

	summary := data["summary"].(map[string]interface{})
	require.Equal(t, float64(2), summary["call_count"])
	// Dedup: two call logs share one trace_id → spans queried once → 2 spans.
	require.Equal(t, float64(2), summary["span_count"])
	require.Equal(t, float64(0), summary["error_count"])
	require.NotNil(t, summary["started_at"])
	require.NotNil(t, summary["ended_at"])

	callLogs := data["call_logs"].([]interface{})
	require.Len(t, callLogs, 2)
	for _, cl := range callLogs {
		m := cl.(map[string]interface{})
		require.Equal(t, traceID, m["trace_id"])
	}

	// Timeline: one root whose Children holds the nested span.
	timeline := data["timeline"].([]interface{})
	require.Len(t, timeline, 1)
	root := timeline[0].(map[string]interface{})
	require.Equal(t, "workflow.daily_report.generate", root["Name"])
	children := root["children"].([]interface{})
	require.Len(t, children, 1)
	require.Equal(t, "workflow.daily_report.cluster_tags", children[0].(map[string]interface{})["Name"])
}

// TestGetSession_TokenAggregationAndErrorCount verifies total_tokens sums
// prompt/completion/total across call logs (empty token_usage skipped) and
// error_count counts failed logs.
func TestGetSession_TokenAggregationAndErrorCount(t *testing.T) {
	db := setupSessionTestDB(t)
	const sessionID = "sess-tokens"
	const traceID = "trace-bbb"
	now := time.Now()

	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "op1", SessionID: sessionID, TraceID: traceID,
		Success: true, TokenUsage: `{"prompt":100,"completion":20,"total":120}`, CreatedAt: now,
	}).Error)
	// Empty token_usage must be skipped, not counted as parse error.
	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "op2", SessionID: sessionID, TraceID: traceID,
		Success: false, TokenUsage: "", ErrorCode: "timeout", CreatedAt: now.Add(time.Second),
	}).Error)
	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "op3", SessionID: sessionID, TraceID: traceID,
		Success: true, TokenUsage: `{"prompt":50,"completion":10,"total":60}`, CreatedAt: now.Add(2 * time.Second),
	}).Error)

	ctx, recorder := newSessionContext(sessionID)
	GetSession(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	summary := body["data"].(map[string]interface{})["summary"].(map[string]interface{})
	require.Equal(t, float64(3), summary["call_count"])
	require.Equal(t, float64(1), summary["error_count"])
	tokens := summary["total_tokens"].(map[string]interface{})
	require.Equal(t, float64(150), tokens["prompt"])
	require.Equal(t, float64(30), tokens["completion"])
	require.Equal(t, float64(180), tokens["total"])
}

// TestGetSession_MultipleTraceIDs verifies the IN query spans multiple traces
// for one session and the timeline includes spans from every trace.
func TestGetSession_MultipleTraceIDs(t *testing.T) {
	db := setupSessionTestDB(t)
	const sessionID = "sess-multi"
	now := time.Now()

	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "op1", SessionID: sessionID, TraceID: "trace-1", Success: true, CreatedAt: now,
	}).Error)
	require.NoError(t, db.Create(&models.AICallLog{
		Operation: "op2", SessionID: sessionID, TraceID: "trace-2", Success: true, CreatedAt: now.Add(time.Second),
	}).Error)

	// One root span per trace.
	require.NoError(t, db.Create(&tracing.OtelSpan{
		TraceID: "trace-1", SpanID: "s1", ParentSpanID: "", Name: "root-1",
		StartTimeUnixNano: now.UnixNano(), EndTimeUnixNano: now.Add(time.Second).UnixNano(),
	}).Error)
	require.NoError(t, db.Create(&tracing.OtelSpan{
		TraceID: "trace-2", SpanID: "s2", ParentSpanID: "", Name: "root-2",
		StartTimeUnixNano: now.Add(2 * time.Second).UnixNano(), EndTimeUnixNano: now.Add(3 * time.Second).UnixNano(),
	}).Error)

	ctx, recorder := newSessionContext(sessionID)
	GetSession(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	data := body["data"].(map[string]interface{})
	summary := data["summary"].(map[string]interface{})
	require.Equal(t, float64(2), summary["call_count"])
	// Both traces contribute their span.
	require.Equal(t, float64(2), summary["span_count"])
	timeline := data["timeline"].([]interface{})
	require.Len(t, timeline, 2)
}
