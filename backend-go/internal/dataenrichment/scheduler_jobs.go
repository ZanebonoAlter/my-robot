package dataenrichment

import (
	"context"
	"fmt"
	"time"

	"syntopica-backend/internal/admin/scheduler"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
)

// WeeklyLifelineJob creates a scheduler JobFunc that:
// 1. Runs HealStale for week granularity (self-heal any missed weeks).
// 2. Refreshes all active topics' week context for the current week.
func WeeklyLifelineJob(svc *service.LifelineContextService, lister ActiveTopicLister) scheduler.JobFunc {
	return func(ctx context.Context) (*scheduler.JobResult, error) {
		gran := string(repository.GranularityWeek)
		now := time.Now()
		startedAt := now

		// Step 1: self-heal.
		if err := svc.HealStale(ctx, gran, now); err != nil {
			return nil, fmt.Errorf("weekly lifeline: heal stale: %w", err)
		}

		// Step 2: refresh all active topics.
		topics, err := lister.ListActiveTopicIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("weekly lifeline: list active topics: %w", err)
		}

		refreshed, failed := 0, 0
		for _, tid := range topics {
			if err := svc.RefreshWeek(ctx, tid, now); err != nil {
				failed++
				continue
			}
			refreshed++
		}

		return &scheduler.JobResult{
			Data: map[string]interface{}{
				"refreshed":    refreshed,
				"failed":       failed,
				"total_topics": len(topics),
				"granularity":  gran,
				"started_at":   startedAt.Format(time.RFC3339),
				"finished_at":  time.Now().Format(time.RFC3339),
			},
			Summary: fmt.Sprintf("lifeline week: healed stale + refreshed %d/%d topics", refreshed, len(topics)),
		}, nil
	}
}

// MonthlyLifelineJob creates a scheduler JobFunc for monthly lifeline context.
func MonthlyLifelineJob(svc *service.LifelineContextService, lister ActiveTopicLister) scheduler.JobFunc {
	return func(ctx context.Context) (*scheduler.JobResult, error) {
		gran := string(repository.GranularityMonth)
		now := time.Now()
		startedAt := now

		if err := svc.HealStale(ctx, gran, now); err != nil {
			return nil, fmt.Errorf("monthly lifeline: heal stale: %w", err)
		}

		topics, err := lister.ListActiveTopicIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("monthly lifeline: list active topics: %w", err)
		}

		refreshed, failed := 0, 0
		for _, tid := range topics {
			if err := svc.RefreshMonth(ctx, tid, now); err != nil {
				failed++
				continue
			}
			refreshed++
		}

		return &scheduler.JobResult{
			Data: map[string]interface{}{
				"refreshed":    refreshed,
				"failed":       failed,
				"total_topics": len(topics),
				"granularity":  gran,
				"started_at":   startedAt.Format(time.RFC3339),
				"finished_at":  time.Now().Format(time.RFC3339),
			},
			Summary: fmt.Sprintf("lifeline month: healed stale + refreshed %d/%d topics", refreshed, len(topics)),
		}, nil
	}
}

// YearlyLifelineJob creates a scheduler JobFunc for yearly lifeline context.
func YearlyLifelineJob(svc *service.LifelineContextService, lister ActiveTopicLister) scheduler.JobFunc {
	return func(ctx context.Context) (*scheduler.JobResult, error) {
		gran := string(repository.GranularityYear)
		now := time.Now()
		startedAt := now

		if err := svc.HealStale(ctx, gran, now); err != nil {
			return nil, fmt.Errorf("yearly lifeline: heal stale: %w", err)
		}

		topics, err := lister.ListActiveTopicIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("yearly lifeline: list active topics: %w", err)
		}

		refreshed, failed := 0, 0
		for _, tid := range topics {
			if err := svc.RefreshYear(ctx, tid, now); err != nil {
				failed++
				continue
			}
			refreshed++
		}

		return &scheduler.JobResult{
			Data: map[string]interface{}{
				"refreshed":    refreshed,
				"failed":       failed,
				"total_topics": len(topics),
				"granularity":  gran,
				"started_at":   startedAt.Format(time.RFC3339),
				"finished_at":  time.Now().Format(time.RFC3339),
			},
			Summary: fmt.Sprintf("lifeline year: healed stale + refreshed %d/%d topics", refreshed, len(topics)),
		}, nil
	}
}
