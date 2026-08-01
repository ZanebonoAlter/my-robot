package handler

import (
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
)

// truncatePrompt truncates s to at most max runes. Appends "..." if truncated.
// Returns the original string unchanged if it fits within max runes.
func truncatePrompt(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var buf []byte
	count := 0
	for _, r := range s {
		if count >= max {
			break
		}
		buf = append(buf, string(r)...)
		count++
	}
	return string(buf) + "..."
}

// ListCallLogs handles GET /api/ai/call-logs.
// Supports optional query params: operation, session_id, capability,
// from, to (ISO8601), limit (default 50, max 200), offset.
// Returns gin.H{"success": true, "data": [...]}.
func ListCallLogs(c *gin.Context) {
	query := c.Request.URL.Query()

	operation := query.Get("operation")
	sessionID := query.Get("session_id")
	capability := query.Get("capability")
	fromStr := query.Get("from")
	toStr := query.Get("to")
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	// --- pagination defaults & caps ---
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	db := repository.Repo.DB()
	q := db.Model(&models.AICallLog{})

	// --- filters ---
	if operation != "" {
		q = q.Where("operation = ?", operation)
	}
	if sessionID != "" {
		q = q.Where("session_id = ?", sessionID)
	}
	if capability != "" {
		q = q.Where("capability = ?", capability)
	}
	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			q = q.Where("created_at <= ?", t)
		}
	}

	// --- sort ---
	if sessionID != "" {
		q = q.Order("created_at ASC")
	} else {
		q = q.Order("created_at DESC")
	}

	q = q.Limit(limit).Offset(offset)

	var logs []models.AICallLog
	if err := q.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	data := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		data = append(data, gin.H{
			"id":               l.ID,
			"operation":        l.Operation,
			"session_id":       l.SessionID,
			"capability":       l.Capability,
			"route_name":       l.RouteName,
			"provider_name":    l.ProviderName,
			"model":            l.Model,
			"success":          l.Success,
			"latency_ms":       l.LatencyMs,
			"token_usage":      l.TokenUsage,
			"prompt":           truncatePrompt(l.Prompt, 500),
			"response_snippet": l.ResponseSnippet,
			"created_at":       l.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
