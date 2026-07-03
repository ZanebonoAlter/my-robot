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

// Service layer re-exports (used by admin scheduler)
var (
	CollectBoardIDsForDate = service.CollectBoardIDsForDate
	GenerateDailyReport    = service.GenerateDailyReport
	SaveReport             = service.SaveReport
	GenerateAndSaveReport  = service.GenerateAndSaveReport
)
