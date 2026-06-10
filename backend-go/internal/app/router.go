package app

import (
	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/admin"
	"syntopica-backend/internal/reader"
	tagmanagement "syntopica-backend/internal/tagmanagement"
	taganalysis "syntopica-backend/internal/tagmanagement/analysis"
	tagwatched "syntopica-backend/internal/tagmanagement/watched"
	"syntopica-backend/internal/topicgraph"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/tracing"
	"syntopica-backend/internal/platform/ws"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": "connected",
		})
	})

	r.GET("/api/tasks/status", admin.GetTasksStatus)

	r.GET("/ws", ws.HandleWebSocket)

	api := r.Group("/api")
	{
		categories := api.Group("/categories")
		{
			categories.GET("", reader.GetCategories)
			categories.POST("", reader.CreateCategory)
			categories.PUT("/:category_id", reader.UpdateCategory)
			categories.DELETE("/:category_id", reader.DeleteCategory)
		}

		feeds := api.Group("/feeds")
		{
			feeds.GET("", reader.GetFeeds)
			feeds.GET("/:feed_id", reader.GetFeed)
			feeds.POST("", reader.CreateFeed)
			feeds.PUT("/:feed_id", reader.UpdateFeed)
			feeds.DELETE("/:feed_id", reader.DeleteFeed)
			feeds.POST("/:feed_id/refresh", reader.RefreshFeed)
			feeds.POST("/fetch", reader.FetchFeed)
			feeds.POST("/refresh-all", reader.RefreshAllFeeds)
		}

		articles := api.Group("/articles")
		{
			articles.GET("/stats", reader.GetArticlesStats)
			articles.GET("", reader.GetArticles)
			articles.GET("/:article_id", reader.GetArticle)
			articles.POST("/:article_id/tags", reader.RetagArticleHandler)
			articles.PUT("/:article_id", reader.UpdateArticle)
			articles.PUT("/bulk-update", reader.BulkUpdateArticles)
		}

		ai := api.Group("/ai")
		{
			ai.GET("/providers", admin.ListProviders)
			ai.POST("/providers", admin.UpsertProvider)
			ai.PUT("/providers/:provider_id", admin.UpdateProvider)
			ai.DELETE("/providers/:provider_id", admin.DeleteProvider)
			ai.GET("/routes", admin.ListRoutes)
			ai.PUT("/routes/:capability", admin.UpdateRoute)
			ai.GET("/settings", admin.GetSettings)
			ai.POST("/settings", admin.SaveSettings)
		}

		opml := api.Group("")
		{
			opml.POST("/import-opml", reader.ImportOPML)
			opml.GET("/export-opml", reader.ExportOPML)
		}

		schedulers := api.Group("/schedulers")
		{
			schedulers.GET("/status", admin.GetSchedulersStatus)
			schedulers.GET("/:name/status", admin.GetSchedulerStatus)
			schedulers.POST("/:name/trigger", admin.TriggerScheduler)
			schedulers.POST("/:name/reset", admin.ResetSchedulerStats)
			schedulers.PUT("/:name/interval", admin.UpdateSchedulerInterval)
		}

		readingBehavior := api.Group("/reading-behavior")
		{
			readingBehavior.POST("/track", admin.TrackReadingBehavior)
			readingBehavior.POST("/track-batch", admin.BatchTrackReadingBehavior)
			readingBehavior.GET("/stats", admin.GetReadingStats)
		}

		preferences := api.Group("/user-preferences")
		{
			preferences.GET("", admin.GetUserPreferences)
			preferences.POST("/update", admin.TriggerPreferenceUpdate)
		}

		contentCompletion := api.Group("/content-completion")
		{
			contentCompletion.POST("/articles/:article_id/complete", reader.CompleteArticleContent)
			contentCompletion.POST("/feeds/:feed_id/complete-all", reader.CompleteFeedArticles)
			contentCompletion.GET("/articles/:article_id/status", reader.GetCompletionStatus)
			contentCompletion.GET("/overview", reader.GetCompletionOverview)
		}

		firecrawl := api.Group("/firecrawl")
		{
			firecrawl.POST("/article/:id", reader.CrawlArticle)
			firecrawl.POST("/feed/:id/enable", reader.EnableFeedFirecrawl)
			firecrawl.GET("/status", reader.GetFirecrawlStatus)
			firecrawl.POST("/settings", reader.SaveFirecrawlSettings)
		}

		topicGraph := api.Group("/topic-graph")
		{
			topicGraph.GET("/:type", topicgraph.GetTopicGraph)
			topicGraph.GET("/topic/:slug", topicgraph.GetTopicDetail)
			topicGraph.GET("/by-category", topicgraph.GetTopicsByCategory)
			topicGraph.GET("/tag/:slug/digests", topicgraph.GetDigestsByArticleTagHandler)
			topicGraph.GET("/tag/:slug/pending-articles", topicgraph.GetPendingArticlesByTagHandler)
			topicGraph.GET("/topic/:slug/articles", topicgraph.GetTopicArticles)
		}
		taganalysis.RegisterAnalysisRoutes(topicGraph, taganalysis.GetAnalysisService(database.DB))
		tagmanagement.RegisterEmbeddingConfigRoutes(api)
		tagmanagement.RegisterEmbeddingQueueRoutes(api)
		tagmanagement.RegisterMergeReembeddingQueueRoutes(api)
		tagmanagement.RegisterTagQueueRoutes(api)
		tagmanagement.RegisterTagManagementRoutes(api)
		tagwatched.RegisterWatchedTagsRoutes(api)
		tagmanagement.RegisterTagMergePreviewRoutes(api)
		tagmanagement.RegisterSemanticBoardRoutes(api)

		admin.RegisterNarrativeRoutes(api)

		topicgraph.RegisterDailyReportRoutes(api)

		traceHandler := tracing.NewTraceHandler(database.DB)
		traces := api.Group("/traces")
		{
			traces.GET("", traceHandler.GetTraceByTraceID)
			traces.GET("/recent", traceHandler.GetRecentTraces)
			traces.GET("/search", traceHandler.SearchTraces)
			traces.GET("/stats", traceHandler.GetTraceStats)
			traces.GET("/:trace_id/timeline", traceHandler.GetTraceTimeline)
			traces.GET("/:trace_id/otlp", traceHandler.ExportTraceOTLP)
		}
	}
}
