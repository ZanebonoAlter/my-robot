package app

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"syntopica-backend/internal/admin"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
	content "syntopica-backend/internal/reader"
	tagging "syntopica-backend/internal/tagmanagement"
)

type Runtime struct {
	Registry *admin.SchedulerRegistry
}

func resetStaleStates() {
	resetCount := 0

	result := database.DB.Model(&models.SchedulerTask{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status":     "idle",
			"last_error": "reset on startup: previous process terminated unexpectedly",
		})
	resetCount += int(result.RowsAffected)

	if resetCount > 0 {
		logging.Infof("Reset %d scheduler task(s) stuck in 'running' state", resetCount)
	}

	feedResult := database.DB.Model(&models.Feed{}).
		Where("refresh_status = ?", "refreshing").
		Updates(map[string]interface{}{
			"refresh_status": "idle",
			"refresh_error":  "reset on startup: previous process terminated unexpectedly",
		})
	if feedResult.RowsAffected > 0 {
		logging.Infof("Reset %d feed(s) stuck in 'refreshing' state", feedResult.RowsAffected)
	}

	articleResult := database.DB.Model(&models.Article{}).
		Where("firecrawl_status = ?", "processing").
		Updates(map[string]interface{}{
			"firecrawl_status": "pending",
			"firecrawl_error":  "reset on startup: previous process terminated unexpectedly",
		})
	if articleResult.RowsAffected > 0 {
		logging.Infof("Reset %d article(s) stuck in 'processing' firecrawl state", articleResult.RowsAffected)
	}

	jobResult := database.DB.Model(&models.FirecrawlJob{}).
		Where("status = ?", "leased").
		Updates(map[string]interface{}{
			"status":           "pending",
			"leased_at":        nil,
			"lease_expires_at": nil,
		})
	if jobResult.RowsAffected > 0 {
		logging.Infof("Reset %d firecrawl job(s) stuck in 'leased' state", jobResult.RowsAffected)
	}

	tagJobResult := database.DB.Model(&models.TagJob{}).
		Where("status = ?", "leased").
		Updates(map[string]interface{}{
			"status":           "pending",
			"leased_at":        nil,
			"lease_expires_at": nil,
		})
	if tagJobResult.RowsAffected > 0 {
		logging.Infof("Reset %d tag job(s) stuck in 'leased' state", tagJobResult.RowsAffected)
	}
}

func StartRuntime() *Runtime {
	resetStaleStates()

	tagging.StartAllWorkers()

	registry := admin.NewSchedulerRegistry()

	// Register each scheduler
	registry.Register("auto_refresh", admin.NewAutoRefreshScheduler(60))
	registry.Register("preference_update", admin.NewPreferenceUpdateScheduler(1800))
	registry.Register("firecrawl", admin.NewFirecrawlScheduler())

	content.InitContentCompletionHandler()
	registry.Register("content_completion", admin.NewContentCompletionScheduler(
		content.GetContentCompletionService(),
		60,
	))

	registry.Register("blocked_article_recovery", admin.NewBlockedArticleRecoveryScheduler(3600))
	registry.Register("tag_quality_score", admin.NewTagQualityScoreScheduler(3600))
	registry.Register("log_cleanup", admin.NewLogCleanupScheduler(86400))
	registry.Register("daily_report", admin.NewDailyReportScheduler(86400))
	registry.Register("aux_label_cleanup", admin.NewAuxLabelCleanupScheduler(3600))

	// Set global registry for handler access
	admin.SetRegistry(registry)
	content.SetSchedulerLookup(func(name string) interface{} {
		s, _ := registry.Get(name)
		return s
	})

	registry.StartAll()

	return &Runtime{Registry: registry}
}

func SetupGracefulShutdown(runtime *Runtime) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logging.Infof("Received signal: %v, shutting down gracefully...", sig)

		tagging.StopAllWorkers()

		if runtime.Registry != nil {
			runtime.Registry.StopAll(30 * time.Second)
		}

		logging.Infoln("Graceful shutdown completed")
		os.Exit(0)
	}()
}
