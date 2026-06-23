package scheduler

import (
	"context"
	"fmt"
	"time"

	"syntopica-backend/internal/admin/repository"
)

// LogCleanupJob deletes expired ai_call_logs and otel_spans rows.
func LogCleanupJob(ctx context.Context) (*JobResult, error) {
	cutoff := time.Now().AddDate(0, 0, -7)

	var aiCallLogsDeleted int64
	result := repository.Repo.DB().Exec("DELETE FROM ai_call_logs WHERE created_at < ?", cutoff)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to clean ai_call_logs: %w", result.Error)
	}
	aiCallLogsDeleted = result.RowsAffected

	var otelSpansDeleted int64
	cutoffNano := cutoff.UnixNano()
	result = repository.Repo.DB().Exec("DELETE FROM otel_spans WHERE start_time_unix_nano < ?", cutoffNano)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to clean otel_spans: %w", result.Error)
	}
	otelSpansDeleted = result.RowsAffected

	return &JobResult{
		Data: map[string]interface{}{
			"last_ai_call_logs_deleted": aiCallLogsDeleted,
			"last_otel_spans_deleted":   otelSpansDeleted,
		},
		Summary: fmt.Sprintf("ai_call_logs=%d, otel_spans=%d", aiCallLogsDeleted, otelSpansDeleted),
	}, nil
}
