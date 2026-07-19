package admin

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all admin module routes under the given router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	ai := rg.Group("/ai")
	{
		ai.GET("/providers", ListProviders)
		ai.POST("/providers", UpsertProvider)
		ai.PUT("/providers/:provider_id", UpdateProvider)
		ai.DELETE("/providers/:provider_id", DeleteProvider)
		ai.GET("/routes", ListRoutes)
		ai.PUT("/routes/:capability", UpdateRoute)
		ai.GET("/settings", GetSettings)
		ai.POST("/settings", SaveSettings)
		ai.POST("/test", TestConnection)
		ai.GET("/call-logs", ListCallLogs)
	}

	schedulers := rg.Group("/schedulers")
	{
		schedulers.GET("/status", GetSchedulersStatus)
		schedulers.GET("/:name/status", GetSchedulerStatus)
		schedulers.POST("/:name/trigger", TriggerScheduler)
		schedulers.POST("/:name/reset", ResetSchedulerStats)
		schedulers.PUT("/:name/interval", UpdateSchedulerInterval)
		schedulers.PUT("/:name/schedule-time", UpdateSchedulerScheduleTime)
	}

	readingBehavior := rg.Group("/reading-behavior")
	{
		readingBehavior.POST("/track", TrackReadingBehavior)
		readingBehavior.POST("/track-batch", BatchTrackReadingBehavior)
		readingBehavior.GET("/stats", GetReadingStats)
	}

	preferences := rg.Group("/user-preferences")
	{
		preferences.GET("", GetUserPreferences)
		preferences.POST("/update", TriggerPreferenceUpdate)
	}
}
