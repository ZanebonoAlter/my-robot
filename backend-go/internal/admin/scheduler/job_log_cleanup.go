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

	// Embedding cache rows older than 14 days are stale: same-name model
	// upgrades repopulate fresh vectors anyway, and old ones would shadow-hit.
	// 14d (not 90d): hits overwhelmingly land within a day of write (nightly
	// processing windows), so longer retention was pure disk waste at
	// ~30KB/row for jsonb vectors.
	embeddingCacheCutoff := time.Now().AddDate(0, 0, -14)
	result = repository.Repo.DB().Exec("DELETE FROM ai_embedding_cache WHERE created_at < ?", embeddingCacheCutoff)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to clean ai_embedding_cache: %w", result.Error)
	}
	embeddingCacheDeleted := result.RowsAffected

	// Completed embedding queue rows older than 30 days are history-only:
	// retry/pending rows keep their own status, and the partial index
	// idx_embedding_queues_completed_created backs this delete.
	embeddingQueueCutoff := time.Now().AddDate(0, 0, -30)
	result = repository.Repo.DB().Exec("DELETE FROM embedding_queues WHERE status = 'completed' AND created_at < ?", embeddingQueueCutoff)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to clean embedding_queues: %w", result.Error)
	}
	embeddingQueueDeleted := result.RowsAffected

	return &JobResult{
		Data: map[string]interface{}{
			"last_ai_call_logs_deleted":    aiCallLogsDeleted,
			"last_otel_spans_deleted":      otelSpansDeleted,
			"last_embedding_cache_deleted": embeddingCacheDeleted,
			"last_embedding_queue_deleted": embeddingQueueDeleted,
		},
		Summary: fmt.Sprintf("ai_call_logs=%d, otel_spans=%d, ai_embedding_cache=%d, embedding_queues=%d", aiCallLogsDeleted, otelSpansDeleted, embeddingCacheDeleted, embeddingQueueDeleted),
	}, nil
}
