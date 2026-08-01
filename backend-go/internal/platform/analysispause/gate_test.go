package analysispause

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/platform/aisettings"
	"syntopica-backend/internal/platform/testutil"
)

// setupTestDB initializes database.DB through the same testcontainer helper the
// rest of the codebase uses (testutil.SetupTestDB), exactly like the aisettings
// config-store tests: the pause switch is persisted in ai_settings.analysis_paused.
func setupTestDB(t *testing.T) {
	t.Helper()
	testutil.SetupTestDB(t)
}

// TestIsPaused covers the gate's core contract: after SetPaused(true) the gate
// reports paused, after SetPaused(false) it reports not paused.
func TestIsPaused(t *testing.T) {
	setupTestDB(t)

	require.NoError(t, SetPaused(true))
	require.True(t, IsPaused())

	require.NoError(t, SetPaused(false))
	require.False(t, IsPaused())
}

// TestSetPausedRoundTrip verifies the persisted round trip through the
// aisettings store: engaging writes {"paused":true,"paused_at":...}, releasing
// writes {"paused":false} with no timestamp.
func TestSetPausedRoundTrip(t *testing.T) {
	setupTestDB(t)

	require.NoError(t, SetPaused(true))
	paused, pausedAt, err := aisettings.LoadAnalysisPausedConfig()
	require.NoError(t, err)
	require.True(t, paused)
	require.NotNil(t, pausedAt)

	require.NoError(t, SetPaused(false))
	paused, pausedAt, err = aisettings.LoadAnalysisPausedConfig()
	require.NoError(t, err)
	require.False(t, paused)
	require.Nil(t, pausedAt)
}

// TestPausedAt covers the timestamp surface: nil while not paused, a recent
// timestamp while paused, nil again after release.
func TestPausedAt(t *testing.T) {
	setupTestDB(t)

	require.Nil(t, PausedAt())

	require.NoError(t, SetPaused(true))
	at := PausedAt()
	require.NotNil(t, at)
	require.WithinDuration(t, time.Now().UTC(), *at, 2*time.Minute,
		"paused_at should be stamped near now")

	require.NoError(t, SetPaused(false))
	require.Nil(t, PausedAt())
}
