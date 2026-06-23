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
	n, err := preferenceService.UpdateAllPreferencesWithCount()
	if err != nil {
		return nil, fmt.Errorf("preference update failed: %w", err)
	}
	return &JobResult{
		Data:    map[string]interface{}{"updated_count": n},
		Summary: fmt.Sprintf("updated %d preferences", n),
	}, nil
}
