package scheduler

import (
	"context"
	"fmt"

	"syntopica-backend/internal/admin/repository"
	tagging "syntopica-backend/internal/tagmanagement"
)

// AuxLabelCleanupJob disables auxiliary labels with no active topic_tag references.
func AuxLabelCleanupJob(ctx context.Context) (*JobResult, error) {
	service := tagging.NewAuxiliaryLabelService(repository.Repo.DB(), nil)
	result, err := service.GC(ctx, tagging.AuxLabelGCRequest{
		Mode:      tagging.AuxLabelGCModeDisable,
		GraceDays: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("aux label GC failed: %w", err)
	}

	return &JobResult{
		Data: map[string]interface{}{
			"last_disabled_count": result.AffectedCount,
		},
		Summary: fmt.Sprintf("disabled %d labels", result.AffectedCount),
	}, nil
}
