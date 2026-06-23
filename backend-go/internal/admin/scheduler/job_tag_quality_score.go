package scheduler

import (
	"context"
	"fmt"
	"time"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	tagservice "syntopica-backend/internal/tagmanagement/service"
)

// TagQualityScoreJob recomputes persistent quality scores for topic tags.
func TagQualityScoreJob(ctx context.Context) (*JobResult, error) {
	startTime := time.Now()

	if err := tagservice.ComputeAllQualityScores(); err != nil {
		return nil, fmt.Errorf("failed to compute quality scores: %w", err)
	}

	// Reconcile: fix ref_count for auxiliary labels
	if result := repository.Repo.DB().Exec(`
		UPDATE semantic_labels
		SET ref_count = (
			SELECT COUNT(*) FROM topic_tag_semantic_labels
			WHERE semantic_label_id = semantic_labels.id
		)
		WHERE label_type = 'auxiliary'`); result.Error != nil {
		logging.Warnf("TagQualityScore: failed to reconcile auxiliary label ref_count: %v", result.Error)
	}

	// Reconcile: remove orphan auxiliary labels
	var orphanAuxDeleted int64
	cutoff := time.Now().AddDate(0, 0, -1)
	if result := repository.Repo.DB().Where(
		"label_type = ? AND ref_count = 0 AND protected = false AND status = ? AND created_at < ? AND id NOT IN (SELECT auxiliary_label_id FROM board_composition)",
		"auxiliary", "active", cutoff,
	).Delete(&models.SemanticLabel{}); result.Error != nil {
		logging.Warnf("TagQualityScore: failed to clean orphan auxiliary labels: %v", result.Error)
	} else if result.RowsAffected > 0 {
		orphanAuxDeleted = result.RowsAffected
		logging.Infof("TagQualityScore: cleaned up %d orphan auxiliary labels", result.RowsAffected)
	}

	return &JobResult{
		Data: map[string]interface{}{
			"updated_count":      0, // ComputedAllQualityScores doesn't return a count
			"orphan_aux_deleted": orphanAuxDeleted,
			"finished_at":        time.Now().Format(time.RFC3339),
			"started_at":         startTime.Format(time.RFC3339),
		},
		Summary: fmt.Sprintf("quality scores recomputed (orphan_aux_deleted=%d)", orphanAuxDeleted),
	}, nil
}
