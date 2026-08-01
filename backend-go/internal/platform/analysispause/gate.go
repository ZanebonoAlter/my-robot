// Package analysispause provides the global "analysis pause" switch that
// suspends all analysis-heavy work (AI summary, full-text crawling, tagging,
// embedding, reports, lifelines) while keeping ingestion (auto_refresh) and
// lightweight maintenance jobs running.
//
// The switch is persisted in ai_settings.analysis_paused (see
// internal/platform/aisettings) and survives service restarts. Consumers fall
// into two groups:
//
//   - Scheduler JobFuncs: wrap with scheduler.PauseAware (which calls IsPaused).
//   - Long-lived workers (TagQueue / EmbeddingQueue / MergeReembeddingQueue):
//     call IsPaused directly in the lease loop.
//
// IsPaused is fail-open: on read error it returns false so analysis is not
// accidentally blocked by a transient DB failure.
package analysispause

import (
	"time"

	"syntopica-backend/internal/platform/aisettings"
)

// IsPaused reports whether the global analysis pause switch is currently on.
// Fail-open: on read error returns false (do not block analysis).
func IsPaused() bool {
	paused, _, err := aisettings.LoadAnalysisPausedConfig()
	if err != nil {
		return false
	}
	return paused
}

// PausedAt returns the time the pause was last engaged, or nil if not paused
// or the timestamp is unknown. Read errors yield nil.
func PausedAt() *time.Time {
	_, at, err := aisettings.LoadAnalysisPausedConfig()
	if err != nil {
		return nil
	}
	return at
}

// SetPaused engages (true) or releases (false) the global analysis pause.
// Engaging stamps paused_at to now; releasing clears it. Returns the
// underlying store error if the write fails.
func SetPaused(paused bool) error {
	return aisettings.SaveAnalysisPausedConfig(paused)
}
