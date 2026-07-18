package app

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"syntopica-backend/internal/admin"
	"syntopica-backend/internal/admin/scheduler"
	"syntopica-backend/internal/dataenrichment"
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

	// Register each scheduler using the BaseScheduler factory pattern.
	// Each scheduler is configured with its JobFunc, interval, startup delay,
	// and optional TaskPersistence for DB state tracking.

	registry.Register("log_cleanup", scheduler.New(scheduler.Config{
		Name:         "Log Cleanup",
		Interval:     86400 * time.Second,
		StartupDelay: 5 * time.Minute,
		Job:          admin.LogCleanupJob,
		Persistence: admin.NewTaskPersistence("log_cleanup",
			"清理过期的 AI 调用日志和追踪数据"),
	}))

	registry.Register("aux_label_cleanup", scheduler.New(scheduler.Config{
		Name:         "Aux Label Cleanup",
		Interval:     3600 * time.Second,
		StartupDelay: 10 * time.Minute,
		Job:          admin.AuxLabelCleanupJob,
		Persistence: admin.NewTaskPersistence("aux_label_cleanup",
			"清理无活跃标签引用的辅助标签"),
	}))

	registry.Register("blocked_article_recovery", scheduler.New(scheduler.Config{
		Name:     "Blocked Article Recovery",
		Interval: 3600 * time.Second,
		Job:      admin.BlockedArticleRecoveryJob,
		Persistence: admin.NewTaskPersistence("blocked_article_recovery",
			"恢复被阻塞的文章"),
	}))

	// Medium schedulers: with SchedulerTask DB persistence
	registry.Register("preference_update", scheduler.New(scheduler.Config{
		Name:     "Preference Update",
		Interval: 1800 * time.Second,
		Job:      admin.PreferenceUpdateJob,
		Persistence: admin.NewTaskPersistence("preference_update",
			"Update reading preferences from behavior data"),
	}))

	registry.Register("tag_quality_score", scheduler.New(scheduler.Config{
		Name:     "Tag Quality Score",
		Interval: 3600 * time.Second,
		Job:      admin.TagQualityScoreJob,
		Persistence: admin.NewTaskPersistence("tag_quality_score",
			"Recompute persistent quality scores for topic tags"),
	}))

	registry.Register("auto_refresh", scheduler.New(scheduler.Config{
		Name:     "Auto Refresh",
		Interval: 60 * time.Second,
		Job:      admin.AutoRefreshJob,
		Persistence: admin.NewTaskPersistence("auto_refresh",
			"Auto-refresh RSS feeds"),
	}))

	// Complex schedulers
	content.InitContentCompletionHandler()
	registry.Register("content_completion", scheduler.New(scheduler.Config{
		Name:     "Content Completion",
		Interval: 60 * time.Second,
		Job:      admin.ContentCompletionJob(content.GetContentCompletionService()),
		Persistence: admin.NewTaskPersistence("ai_summary",
			"Complete article content and generate article summaries"),
	}))

	// DailyReport: wrapped with TriggerNowWithDate support
	dailyReportNextRunFn := scheduler.NextDailyReportTime
	dailyReportBase := scheduler.New(scheduler.Config{
		Name:    "Daily Report",
		NextRun: dailyReportNextRunFn,
		Job:     admin.DailyReportJob(), // uses current time at each execution
		Persistence: admin.NewTaskPersistenceWithNextRun("daily_report",
			"Generate daily reports for all active semantic boards",
			dailyReportNextRunFn),
	})
	dailyReportWrapper := admin.NewDailyReportSchedulerWrapper(dailyReportBase)
	registry.Register("daily_report", dailyReportWrapper)

	// BoardUpgradeSuggest: daily discover_new generation + watch GC (design D4).
	// Loosely coupled fixed-time trigger (default 06:30), not chained to the
	// daily report. Manual trigger via POST /upgrade-suggestions/generate.
	boardUpgradeNextRunFn := scheduler.NextBoardUpgradeSuggestTime
	registry.Register("board_upgrade_suggest", scheduler.New(scheduler.Config{
		Name:    "Board Upgrade Suggest",
		NextRun: boardUpgradeNextRunFn,
		Job:     admin.BoardUpgradeSuggestJob(),
		Persistence: admin.NewTaskPersistenceWithNextRun("board_upgrade_suggest",
			"每日生成版块升级建议(discover_new)+观察池GC",
			boardUpgradeNextRunFn),
	}))

	// Firecrawl: with custom status enricher
	firecrawlQueue := content.NewFirecrawlJobQueue(database.DB)
	registry.Register("firecrawl", scheduler.New(scheduler.Config{
		Name:         "Firecrawl Crawler",
		Interval:     300 * time.Second,
		StartupDelay: 0,
		Job:          admin.FirecrawlJob(firecrawlQueue, "scheduled"),
		StatusDetail: admin.FirecrawlStatusEnricher(),
		Persistence: admin.NewTaskPersistence("firecrawl",
			"自动爬取文章全文"),
	}))

	// ── Lifeline context schedulers (cycle A) ──────────────────────────────────
	// The repository, cycle-A service, cycle-B orchestrator, and HTTP handler
	// singleton are all wired by dataenrichment.Init in main.go (which runs
	// BEFORE SetupRoutes). StartRuntime only registers the schedulers below.
	lifelineSvc := dataenrichment.GetLifelineService()
	lister := dataenrichment.GetTopicLister()

	// Weekly lifeline: every Monday 03:00 Asia/Shanghai.
	weeklyNextRun := dataenrichment.NextWeeklyLifelineTime
	registry.Register("lifeline_weekly", scheduler.New(scheduler.Config{
		Name:    "Lifeline Weekly Refresh",
		NextRun: weeklyNextRun,
		Job:     dataenrichment.WeeklyLifelineJob(lifelineSvc, lister),
		Persistence: admin.NewTaskPersistenceWithNextRun("lifeline_weekly",
			"每周一刷新所有活跃话题的周度新闻汇总上下文", weeklyNextRun),
	}))

	// Monthly lifeline: every 1st of month 03:30 Asia/Shanghai.
	monthlyNextRun := dataenrichment.NextMonthlyLifelineTime
	registry.Register("lifeline_monthly", scheduler.New(scheduler.Config{
		Name:    "Lifeline Monthly Refresh",
		NextRun: monthlyNextRun,
		Job:     dataenrichment.MonthlyLifelineJob(lifelineSvc, lister),
		Persistence: admin.NewTaskPersistenceWithNextRun("lifeline_monthly",
			"每月1号刷新所有活跃话题的月度新闻汇总上下文", monthlyNextRun),
	}))

	// Yearly lifeline: every Jan 1 04:00 Asia/Shanghai.
	yearlyNextRun := dataenrichment.NextYearlyLifelineTime
	registry.Register("lifeline_yearly", scheduler.New(scheduler.Config{
		Name:    "Lifeline Yearly Refresh",
		NextRun: yearlyNextRun,
		Job:     dataenrichment.YearlyLifelineJob(lifelineSvc, lister),
		Persistence: admin.NewTaskPersistenceWithNextRun("lifeline_yearly",
			"每年1月1号刷新所有活跃话题的年度新闻汇总上下文", yearlyNextRun),
	}))

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
