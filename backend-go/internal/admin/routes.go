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
		ai.GET("/sessions/:session_id", GetSession)
		ai.GET("/health", GetAIHealth)
		ai.PUT("/health/auto-start-models", SetAutoStartModels)
		ai.POST("/health/reprobe", ReprobeAIHealth)
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

	analysis := rg.Group("/analysis")
	{
		analysis.GET("/pause", GetAnalysisPause)
		analysis.POST("/pause", SetAnalysisPause)
	}

	readingBehavior := rg.Group("/reading-behavior")
	{
		readingBehavior.POST("/track", TrackReadingBehavior)
		readingBehavior.POST("/track-batch", BatchTrackReadingBehavior)
		readingBehavior.GET("/stats", GetReadingStats)
	}

	preferenceProfile := rg.Group("/preference-profile")
	{
		preferenceProfile.GET("", GetPreferenceProfile)
		preferenceProfile.POST("/recompute", RecomputePreferenceProfile)
	}

	discovery := rg.Group("/discovery")
	{
		discovery.POST("/catalog/sync", SyncCatalog)
		discovery.GET("/catalog/status", GetCatalogStatus)
		discovery.GET("/recommendations", GetRecommendations)
		discovery.POST("/recommendations/refresh", RefreshRecommendations)
		discovery.POST("/recommendations/:id/accept", AcceptRecommendation)
		discovery.POST("/recommendations/:id/dismiss", DismissRecommendation)
		discovery.POST("/ask", Ask)
	}

	settings := rg.Group("/settings")
	{
		settings.GET("/rsshub", GetRSSHubSettings)
		settings.POST("/rsshub", SaveRSSHubSettings)
		settings.GET("/proxy", GetProxySettings)
		settings.POST("/proxy", SaveProxySettings)
		settings.GET("/bocha", GetBochaSettings)
		settings.POST("/bocha", SaveBochaSettings)
	}

	// 路由参数可选值字典 CRUD（feed-param-options）
	routeParamOptions := rg.Group("/admin/route-param-options")
	{
		routeParamOptions.GET("", ListRouteParamOptions)
		routeParamOptions.POST("", CreateRouteParamOption)
		routeParamOptions.PUT("/:id", UpdateRouteParamOption)
		routeParamOptions.DELETE("/:id", DeleteRouteParamOption)
	}
}
