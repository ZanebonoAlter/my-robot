package scheduler

import (
	"context"

	"syntopica-backend/internal/platform/analysispause"
)

// PauseAware wraps a JobFunc so that, when the global analysis pause is active,
// the job is skipped: it returns a benign "skipped: paused" JobResult instead
// of leasing/processing queue tasks. When not paused, the original job runs
// unchanged.
//
// The skipped result is a *success* (err=nil) so it does NOT pollute the
// scheduler's failed-runs counter or trigger failure-isolation logic (see
// docs/reference/flow/scheduler.md 业务约束 #4). This realizes design D1/D3
// of the pause-analysis change: the gate sits right before the lease, so a
// batch already in flight runs to completion (graceful stop).
func PauseAware(job JobFunc) JobFunc {
	return func(ctx context.Context) (*JobResult, error) {
		if analysispause.IsPaused() {
			return &JobResult{
				Summary: "skipped: analysis paused",
				Data:    map[string]interface{}{"skipped": "paused"},
			}, nil
		}
		return job(ctx)
	}
}
