package topicgraph

import (
	"gorm.io/gorm"

	"syntopica-backend/internal/topicgraph/handler"
	"syntopica-backend/internal/topicgraph/repository"
	"syntopica-backend/internal/topicgraph/service"
)

// InitRepository initializes the topicgraph repository singleton.
func InitRepository(db *gorm.DB) {
	repository.InitRepository(db)
}

// RegisterDailyReportRoutes registers all daily report routes.
var RegisterDailyReportRoutes = handler.RegisterDailyReportRoutes

// Graph API handlers
var (
	GetTopicGraph                 = handler.GetTopicGraph
	GetTopicDetail                = handler.GetTopicDetail
	GetTopicsByCategory           = handler.GetTopicsByCategory
	GetTopicArticles              = handler.GetTopicArticles
	GetDigestsByArticleTagHandler = handler.GetDigestsByArticleTagHandler
	GetPendingArticlesByTagHandler = handler.GetPendingArticlesByTagHandler

	// Service layer re-exports (used by admin scheduler)
	CollectBoardIDsForDate = service.CollectBoardIDsForDate
	GenerateDailyReport    = service.GenerateDailyReport
	SaveReport             = service.SaveReport
)
