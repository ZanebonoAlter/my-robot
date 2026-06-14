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

// SetRegistry sets the global scheduler registry for handler access.
func SetRegistry(reg handler.SchedulerRegistry) {
	handler.Reg = reg
}

// ============================================================================
// Handler re-exports (HTTP handlers registered in internal/app/router.go)
// ============================================================================

// AI provider / route / settings handlers
var (
	ListProviders  = handler.ListProviders
	UpsertProvider = handler.UpsertProvider
	UpdateProvider = handler.UpdateProvider
	DeleteProvider = handler.DeleteProvider
	ListRoutes     = handler.ListRoutes
	UpdateRoute    = handler.UpdateRoute
	GetSettings    = handler.GetSettings
	SaveSettings   = handler.SaveSettings
	TestConnection = handler.TestConnection
)

// Scheduler handlers
var (
	GetSchedulersStatus     = handler.GetSchedulersStatus
	GetSchedulerStatus      = handler.GetSchedulerStatus
	TriggerScheduler        = handler.TriggerScheduler
	ResetSchedulerStats     = handler.ResetSchedulerStats
	UpdateSchedulerInterval = handler.UpdateSchedulerInterval
	GetTasksStatus          = handler.GetTasksStatus
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

// ============================================================================
// Scheduler re-exports (types and constructors used in internal/app/runtime.go)
// ============================================================================

// SchedulerRegistry is the registry that manages named scheduler instances.
type SchedulerRegistry = scheduler.Registry

var (
	NewSchedulerRegistry = scheduler.NewRegistry
)

// BaseScheduler types and constructors for the factory pattern.
// Runtime uses scheduler.New(scheduler.Config{...}) directly.
var (
	NewBaseScheduler   = scheduler.New
	NewTaskPersistence = scheduler.NewTaskPersistence
)

// DailyReportSchedulerWrapper for TriggerNowWithDate support.
type DailyReportScheduler = scheduler.DailyReportSchedulerWrapper

var (
	NewDailyReportSchedulerWrapper = scheduler.NewDailyReportSchedulerWrapper
)

// Job functions (for use in runtime.go when creating schedulers).
var (
	LogCleanupJob             = scheduler.LogCleanupJob
	AuxLabelCleanupJob        = scheduler.AuxLabelCleanupJob
	BlockedArticleRecoveryJob = scheduler.BlockedArticleRecoveryJob
	PreferenceUpdateJob       = scheduler.PreferenceUpdateJob
	TagQualityScoreJob        = scheduler.TagQualityScoreJob
	AutoRefreshJob            = scheduler.AutoRefreshJob
	ContentCompletionJob      = scheduler.ContentCompletionJob
	DailyReportJob            = scheduler.DailyReportJob
	FirecrawlJob              = scheduler.FirecrawlJob
	FirecrawlStatusEnricher   = scheduler.FirecrawlStatusEnricher
)
