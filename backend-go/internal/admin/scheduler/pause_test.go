package scheduler

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/platform/aihealth"
	"syntopica-backend/internal/platform/analysispause"
	"syntopica-backend/internal/platform/testutil"
)

// useHealthySnapshot installs a ready+healthy aihealth snapshot so IsPaused()
// reflects only the user switch, and restores the not-ready state on cleanup so
// the process-global snapshot never leaks between tests. The health gate now
// folds into IsPaused(): a not-ready snapshot (Healthy()==false) would make
// every "not paused" case look paused.
func useHealthySnapshot(t *testing.T) {
	t.Helper()
	now := time.Now()
	aihealth.SetSnapshotForTest(aihealth.Snapshot{Healthy: true, CheckedAt: &now})
	t.Cleanup(func() { aihealth.SetSnapshotForTest(aihealth.Snapshot{}) })
}

// TestPauseAware_SkipsWhenPaused verifies the D1/D3 gate behavior: while the
// global analysis pause is on, the wrapped job is NOT invoked and the wrapper
// returns a benign success result ("skipped: <reason>", err=nil) instead of an
// error — so it never pollutes the scheduler's failed-runs counter. The summary
// carries the PauseReason (here "user_paused") for observability.
func TestPauseAware_SkipsWhenPaused(t *testing.T) {
	testutil.SetupTestDB(t)
	require.NoError(t, analysispause.SetPaused(true))

	var called int32
	job := func(ctx context.Context) (*JobResult, error) {
		atomic.AddInt32(&called, 1)
		return &JobResult{Summary: "real job ran", Data: map[string]interface{}{"ran": true}}, nil
	}

	wrapped := PauseAware(job)
	result, err := wrapped(context.Background())

	require.NoError(t, err, "skipped result must be a success (err=nil)")
	require.EqualValues(t, 0, atomic.LoadInt32(&called), "real job must NOT run while paused")
	require.NotNil(t, result)
	require.Contains(t, result.Summary, "skipped")
	require.Contains(t, result.Summary, "user_paused", "summary should carry the pause reason")
	require.Equal(t, "paused", result.Data["skipped"])
}

// TestPauseAware_SkipsWhenModelUnhealthy verifies the health-gate path: with
// the user switch released but models NOT healthy, the job is still skipped and
// the reason reflects the health dimension (model_unhealthy), not user_paused.
func TestPauseAware_SkipsWhenModelUnhealthy(t *testing.T) {
	testutil.SetupTestDB(t)
	require.NoError(t, analysispause.SetPaused(false))
	// Ready snapshot but unhealthy.
	now := time.Now()
	aihealth.SetSnapshotForTest(aihealth.Snapshot{Healthy: false, CheckedAt: &now})
	t.Cleanup(func() { aihealth.SetSnapshotForTest(aihealth.Snapshot{}) })

	var called int32
	job := func(ctx context.Context) (*JobResult, error) {
		atomic.AddInt32(&called, 1)
		return &JobResult{Summary: "real job ran"}, nil
	}

	result, err := PauseAware(job)(context.Background())

	require.NoError(t, err)
	require.EqualValues(t, 0, atomic.LoadInt32(&called), "real job must NOT run when models are unhealthy")
	require.NotNil(t, result)
	require.True(t, strings.Contains(result.Summary, "model_unhealthy"), "summary should flag the health reason")
}

// TestPauseAware_RunsWhenNotPaused verifies the pass-through path: with the
// user switch released AND models healthy, the wrapper invokes the original job
// and returns its result unchanged.
func TestPauseAware_RunsWhenNotPaused(t *testing.T) {
	testutil.SetupTestDB(t)
	require.NoError(t, analysispause.SetPaused(false))
	useHealthySnapshot(t)

	var called int32
	job := func(ctx context.Context) (*JobResult, error) {
		atomic.AddInt32(&called, 1)
		return &JobResult{Summary: "real job ran", Data: map[string]interface{}{"ran": true}}, nil
	}

	wrapped := PauseAware(job)
	result, err := wrapped(context.Background())

	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&called), "real job must run when not paused")
	require.NotNil(t, result)
	require.Equal(t, "real job ran", result.Summary)
	require.Equal(t, true, result.Data["ran"])
}
