// Package analysispause provides the global "analysis pause" switch that
// suspends all analysis-heavy work (AI summary, full-text crawling, tagging,
// embedding, reports, lifelines) while keeping ingestion (auto_refresh) and
// lightweight maintenance jobs running.
//
// The user switch is persisted in ai_settings.analysis_paused (see
// internal/platform/aisettings) and survives service restarts. Consumers fall
// into two groups:
//
//   - Scheduler JobFuncs: wrap with scheduler.PauseAware (which calls IsPaused).
//   - Long-lived workers (TagQueue / EmbeddingQueue / MergeReembeddingQueue):
//     call IsPaused directly in the lease loop.
//
// There are two readings of the switch:
//
//   - UserPaused(): the user's intent only (the persisted analysis_paused flag).
//     This is what the pause API and the scheduler-status "analysis_paused"
//     field expose so the frontend button never flips because of health.
//   - IsPaused(): the effective pause = UserPaused() || NOT ai-health-ready.
//     Workers/PauseAware use this so analysis does not lease when the model
//     layer is down (spec: 健康门硬执行). The health dimension is fail-closed
//     on the startup race: aihealth.Healthy() returns false until the first
//     startup probe completes, so IsPaused() is true during that window.
//
// UserPaused is fail-open: on read error it returns false so analysis is not
// accidentally blocked by a transient DB failure. The health dimension is
// fail-closed (snapshot not ready → paused).
package analysispause

import (
	"time"

	"syntopica-backend/internal/platform/aihealth"
	"syntopica-backend/internal/platform/aisettings"
)

// UserPaused reports the user's intent only: whether the persisted
// analysis_paused flag is on. It is unaffected by AI model health. Fail-open:
// on read error returns false (do not block analysis on a transient DB error).
//
// Use this for surfaces that must reflect user intent (the pause API, the
// schedulers/status "analysis_paused" field, the favicon/button state).
func UserPaused() bool {
	paused, _, err := aisettings.LoadAnalysisPausedConfig()
	if err != nil {
		return false
	}
	return paused
}

// IsPaused reports the effective analysis pause: the user switch OR the AI
// model layer not being ready (aihealth.Healthy()==false, which also covers the
// startup-race window before the first probe completes). Workers and PauseAware
// use this so analysis does not lease when models are down.
func IsPaused() bool {
	return UserPaused() || !aihealth.Healthy()
}

// PauseReason returns a short machine-readable reason string when IsPaused() is
// true, or "" when it is false. It is consistent with IsPaused(): it is
// non-empty if and only if IsPaused() is true. Useful for skip/observability
// messages. Possible values: "user_paused", "model_unhealthy",
// "user_paused+model_unhealthy".
func PauseReason() string {
	userPaused := UserPaused()
	modelUnhealthy := !aihealth.Healthy()
	switch {
	case userPaused && modelUnhealthy:
		return "user_paused+model_unhealthy"
	case userPaused:
		return "user_paused"
	case modelUnhealthy:
		return "model_unhealthy"
	default:
		return ""
	}
}

// PausedAt returns the time the user pause was last engaged, or nil if not
// paused or the timestamp is unknown. It reflects user intent only (it is NOT
// stamped when analysis is paused solely because models are unhealthy). Read
// errors yield nil.
func PausedAt() *time.Time {
	_, at, err := aisettings.LoadAnalysisPausedConfig()
	if err != nil {
		return nil
	}
	return at
}

// SetPaused engages (true) or releases (false) the user analysis-pause switch.
// Engaging stamps paused_at to now; releasing clears it. Returns the
// underlying store error if the write fails. This only ever reflects user
// intent; it must not be driven by health state.
func SetPaused(paused bool) error {
	return aisettings.SaveAnalysisPausedConfig(paused)
}
