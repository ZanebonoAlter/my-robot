package scheduler

import (
	"context"
	"fmt"

	"syntopica-backend/internal/admin/repository"
	adminservice "syntopica-backend/internal/admin/service"
)

// PreferenceUpdateJob updates reading preferences from behavior data.
func PreferenceUpdateJob(ctx context.Context) (*JobResult, error) {
	preferenceService := adminservice.NewPreferenceService(repository.Repo.DB())
	if err := preferenceService.UpdateAllPreferences(); err != nil {
		return nil, fmt.Errorf("preference update failed: %w", err)
	}
	return &JobResult{
		Data:    map[string]interface{}{},
		Summary: "preferences updated successfully",
	}, nil
}
