package topicgraph

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all topicgraph module routes under the given router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	topicGraph := rg.Group("/topic-graph")
	{
		topicGraph.GET("/:type", GetTopicGraph)
		topicGraph.GET("/topic/:slug", GetTopicDetail)
		topicGraph.GET("/by-category", GetTopicsByCategory)
		topicGraph.GET("/tag/:slug/digests", GetDigestsByArticleTagHandler)
		topicGraph.GET("/tag/:slug/pending-articles", GetPendingArticlesByTagHandler)
		topicGraph.GET("/topic/:slug/articles", GetTopicArticles)
	}

	RegisterDailyReportRoutes(rg)
}
