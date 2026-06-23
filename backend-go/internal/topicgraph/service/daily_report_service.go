package service

import (
	"time"

	"syntopica-backend/internal/topicgraph/repository"
)

// CollectBoardIDsForDate returns all board IDs that have active event tags on a date.
// Delegates to the repository layer. Used by the admin scheduler.
func CollectBoardIDsForDate(date time.Time) ([]uint, error) {
	return repository.Repo.CollectBoardIDsForDate(date)
}

// SaveReport saves a daily report and its sections.
// Delegates to the repository layer. Used by the admin scheduler.
func SaveReport(report *repository.BoardDailyReport, sections []repository.DailyReportSection, threadBatches [][]repository.DailyReportThread) error {
	return repository.Repo.SaveReport(report, sections, threadBatches)
}
