package reader

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all reader module routes under the given router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	categories := rg.Group("/categories")
	{
		categories.GET("", GetCategories)
		categories.POST("", CreateCategory)
		categories.PUT("/:category_id", UpdateCategory)
		categories.DELETE("/:category_id", DeleteCategory)
	}

	feeds := rg.Group("/feeds")
	{
		feeds.GET("", GetFeeds)
		feeds.GET("/:feed_id", GetFeed)
		feeds.POST("", CreateFeed)
		feeds.PUT("/:feed_id", UpdateFeed)
		feeds.DELETE("/:feed_id", DeleteFeed)
		feeds.POST("/:feed_id/refresh", RefreshFeed)
		feeds.POST("/fetch", FetchFeed)
		feeds.POST("/refresh-all", RefreshAllFeeds)
	}

	articles := rg.Group("/articles")
	{
		articles.GET("/stats", GetArticlesStats)
		articles.GET("", GetArticles)
		articles.GET("/:article_id", GetArticle)
		articles.POST("/:article_id/tags", RetagArticleHandler)
		articles.PUT("/:article_id", UpdateArticle)
		articles.PUT("/bulk-update", BulkUpdateArticles)
	}

	opml := rg.Group("")
	{
		opml.POST("/import-opml", ImportOPML)
		opml.GET("/export-opml", ExportOPML)
	}

	contentCompletion := rg.Group("/content-completion")
	{
		contentCompletion.POST("/articles/:article_id/complete", CompleteArticleContent)
		contentCompletion.POST("/feeds/:feed_id/complete-all", CompleteFeedArticles)
		contentCompletion.GET("/articles/:article_id/status", GetCompletionStatus)
		contentCompletion.GET("/overview", GetCompletionOverview)
	}

	firecrawl := rg.Group("/firecrawl")
	{
		firecrawl.POST("/article/:id", CrawlArticle)
		firecrawl.POST("/feed/:id/enable", EnableFeedFirecrawl)
		firecrawl.GET("/status", GetFirecrawlStatus)
		firecrawl.POST("/settings", SaveFirecrawlSettings)
	}
}
