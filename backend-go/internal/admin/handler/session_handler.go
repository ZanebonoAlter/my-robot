package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/tracing"
)

// tokenUsageAgg accumulates prompt/completion/total tokens across the call
// logs of one session. The JSON shape mirrors airouter.TokenUsage
// ({"prompt":N,"completion":N,"total":N}) so the stored token_usage strings
// can be summed without importing the airouter package.
type tokenUsageAgg struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// GetSession handles GET /api/ai/sessions/:session_id.
//
// It aggregates, for one orchestration session, the business call logs
// (ai_call_logs, keyed by session_id) and the tracing timeline (otel_spans,
// reached via the trace_ids written into those call logs). The join key is
// trace_id, NOT the otel_spans.attributes jsonb — see design §1.2.
//
// Empty state (unknown session_id) returns success=true with empty arrays and
// zeroed summary, never 404 (design §1.4), so the frontend can render a uniform
// empty state.
func GetSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "session_id is required"})
		return
	}

	db := repository.Repo.DB()

	// 1. Business call logs for this session, in chronological order.
	var callLogs []models.AICallLog
	if err := db.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&callLogs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 2. Distinct trace_ids → full tracing timeline (trace_id reverse lookup).
	traceIDSet := make(map[string]struct{}, len(callLogs))
	for _, cl := range callLogs {
		if cl.TraceID != "" {
			traceIDSet[cl.TraceID] = struct{}{}
		}
	}
	traceIDs := make([]string, 0, len(traceIDSet))
	for id := range traceIDSet {
		traceIDs = append(traceIDs, id)
	}

	spans, err := tracing.QuerySpansByTraceIDs(db, traceIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	timeline := tracing.BuildSpanTree(spans)
	// BuildSpanTree returns a nil slice when there are no spans; emit `[]`
	// (not null) so the empty-state contract holds (design §1.4).
	if timeline == nil {
		timeline = []tracing.TraceDetail{}
	}

	// 3. Summary aggregation.
	var (
		tokens     tokenUsageAgg
		errorCount int
		startedAt  *time.Time
		endedAt    *time.Time
	)
	for _, cl := range callLogs {
		if !cl.Success {
			errorCount++
		}
		if u := parseTokenUsage(cl.TokenUsage); u != nil {
			tokens.Prompt += u.Prompt
			tokens.Completion += u.Completion
			tokens.Total += u.Total
		}
		startedAt, endedAt = stretchWindow(startedAt, endedAt, cl.CreatedAt, cl.CreatedAt)
	}
	for _, sp := range spans {
		st := time.Unix(0, sp.StartTimeUnixNano)
		en := time.Unix(0, sp.EndTimeUnixNano)
		startedAt, endedAt = stretchWindow(startedAt, endedAt, st, en)
	}

	// 4. Serialize call logs (same item shape as ListCallLogs + trace_id).
	logItems := make([]gin.H, 0, len(callLogs))
	for _, cl := range callLogs {
		logItems = append(logItems, gin.H{
			"id":               cl.ID,
			"operation":        cl.Operation,
			"session_id":       cl.SessionID,
			"trace_id":         cl.TraceID,
			"capability":       cl.Capability,
			"route_name":       cl.RouteName,
			"provider_name":    cl.ProviderName,
			"model":            cl.Model,
			"success":          cl.Success,
			"is_fallback":      cl.IsFallback,
			"latency_ms":       cl.LatencyMs,
			"error_code":       cl.ErrorCode,
			"error_message":    cl.ErrorMessage,
			"token_usage":      cl.TokenUsage,
			"prompt":           truncatePrompt(cl.Prompt, 500),
			"response_snippet": cl.ResponseSnippet,
			"created_at":       cl.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"session_id": sessionID,
			"summary": gin.H{
				"call_count":   len(callLogs),
				"span_count":   len(spans),
				"started_at":   startedAt,
				"ended_at":     endedAt,
				"total_tokens": tokens,
				"error_count":  errorCount,
			},
			"call_logs": logItems,
			"timeline":  timeline,
		},
	})
}

// parseTokenUsage decodes a stored token_usage jsonb string. Returns nil for
// empty or malformed payloads (malformed is treated as "no tokens", not an
// error — one bad row must not fail the whole aggregation).
func parseTokenUsage(raw string) *tokenUsageAgg {
	if raw == "" {
		return nil
	}
	var u tokenUsageAgg
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		return nil
	}
	return &u
}

// stretchWindow widens the [startedAt, endedAt] window to cover [st, en],
// returning the new bounds. Nil bounds are treated as unbounded on first call.
func stretchWindow(startedAt, endedAt *time.Time, st, en time.Time) (*time.Time, *time.Time) {
	newStart, newEnd := st, en
	if startedAt != nil && startedAt.Before(newStart) {
		newStart = *startedAt
	}
	if endedAt != nil && endedAt.After(newEnd) {
		newEnd = *endedAt
	}
	return &newStart, &newEnd
}
