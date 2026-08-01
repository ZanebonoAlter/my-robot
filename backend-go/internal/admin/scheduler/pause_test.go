package scheduler

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/platform/analysispause"
	"syntopica-backend/internal/platform/testutil"
)

// TestPauseAware_SkipsWhenPaused verifies the D1/D3 gate behavior: while the
// global analysis pause is on, the wrapped job is NOT invoked and the wrapper
// returns a benign success result ("skipped: analysis paused", err=nil) instead
// of an error — so it never pollutes the scheduler's failed-runs counter.
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
	require.Equal(t, "paused", result.Data["skipped"])
}

// TestPauseAware_RunsWhenNotPaused verifies the pass-through path: with the
// pause released, the wrapper invokes the original job and returns its result
// unchanged.
func TestPauseAware_RunsWhenNotPaused(t *testing.T) {
	testutil.SetupTestDB(t)
	require.NoError(t, analysispause.SetPaused(false))

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
