package app

import (
	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/admin"
	"syntopica-backend/internal/dataenrichment"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/middleware"
	"syntopica-backend/internal/platform/tracing"
	"syntopica-backend/internal/platform/ws"
	"syntopica-backend/internal/reader"
	tagmanagement "syntopica-backend/internal/tagmanagement"
	"syntopica-backend/internal/topicgraph"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": "connected",
		})
	})

	r.GET("/api/tasks/status", admin.GetTasksStatus)

	// In public read-only demo mode, the WebSocket endpoint is unused (the
	// frontend degrades silently) and background schedulers are disabled, so we
	// skip registering it to avoid an unused connection surface.
	if !middleware.IsDemoReadOnly() {
		r.GET("/ws", ws.HandleWebSocket)
	}

	api := r.Group("/api")
	// Enforce read-only access when DEMO_READ_ONLY=1; no-op in production.
	api.Use(middleware.ReadOnly())
	{
		reader.RegisterRoutes(api)
		topicgraph.RegisterRoutes(api)
		admin.RegisterRoutes(api)

		tagmanagement.RegisterRoutes(api)
		dataenrichment.RegisterRoutes(api)

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
