// Package admin is the public facade for the internal/admin feature module.
// It re-exports symbols from handler/, service/, scheduler/, and repository/
// sub-packages so that external consumers (internal/app, cmd/server) can
// reference everything as admin.X without knowing the internal layout.
package admin

import (
	"gorm.io/gorm"

	"syntopica-backend/internal/admin/handler"
	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/admin/scheduler"
)

// ============================================================================
// Repository wiring
// ============================================================================

// InitRepository delegates to the repository sub-package.
func InitRepository(db *gorm.DB) {
	repository.InitRepository(db)
}

// ============================================================================
// Handler re-exports (HTTP handlers registered in internal/app/router.go)
// ============================================================================

// AI provider / route / settings handlers
var (
	ListProviders     = handler.ListProviders
	UpsertProvider    = handler.UpsertProvider
	UpdateProvider    = handler.UpdateProvider
	DeleteProvider    = handler.DeleteProvider
	ListRoutes        = handler.ListRoutes
	UpdateRoute       = handler.UpdateRoute
	GetSettings       = handler.GetSettings
	SaveSettings      = handler.SaveSettings
)

// Scheduler handlers
var (
	GetSchedulersStatus    = handler.GetSchedulersStatus
	GetSchedulerStatus     = handler.GetSchedulerStatus
	TriggerScheduler       = handler.TriggerScheduler
	ResetSchedulerStats    = handler.ResetSchedulerStats
	UpdateSchedulerInterval = handler.UpdateSchedulerInterval
	GetTasksStatus         = handler.GetTasksStatus
)

// Reading behavior handlers
var (
	TrackReadingBehavior      = handler.TrackReadingBehavior
	BatchTrackReadingBehavior = handler.BatchTrackReadingBehavior
	GetReadingStats           = handler.GetReadingStats
)

// User preference handlers
var (
	GetUserPreferences      = handler.GetUserPreferences
	TriggerPreferenceUpdate = handler.TriggerPreferenceUpdate
)

// Narrative route registration
var RegisterNarrativeRoutes = handler.RegisterNarrativeRoutes

// ============================================================================
// Scheduler re-exports (types and constructors used in internal/app/runtime.go)
// ============================================================================

type (
	AutoRefreshScheduler            = scheduler.AutoRefreshScheduler
	PreferenceUpdateScheduler       = scheduler.PreferenceUpdateScheduler
	ContentCompletionScheduler      = scheduler.ContentCompletionScheduler
	FirecrawlScheduler              = scheduler.FirecrawlScheduler
	BlockedArticleRecoveryScheduler = scheduler.BlockedArticleRecoveryScheduler
	TagQualityScoreScheduler        = scheduler.TagQualityScoreScheduler
	DailyReportScheduler            = scheduler.DailyReportScheduler
	LogCleanupScheduler             = scheduler.LogCleanupScheduler
	AuxLabelCleanupScheduler        = scheduler.AuxLabelCleanupScheduler
)

var (
	NewAutoRefreshScheduler            = scheduler.NewAutoRefreshScheduler
	NewPreferenceUpdateScheduler       = scheduler.NewPreferenceUpdateScheduler
	NewContentCompletionScheduler      = scheduler.NewContentCompletionScheduler
	NewFirecrawlScheduler              = scheduler.NewFirecrawlScheduler
	NewBlockedArticleRecoveryScheduler = scheduler.NewBlockedArticleRecoveryScheduler
	NewTagQualityScoreScheduler        = scheduler.NewTagQualityScoreScheduler
	NewDailyReportScheduler            = scheduler.NewDailyReportScheduler
	NewLogCleanupScheduler             = scheduler.NewLogCleanupScheduler
	NewAuxLabelCleanupScheduler        = scheduler.NewAuxLabelCleanupScheduler
)
