package service

import (
	"time"

	tagging "syntopica-backend/internal/tagmanagement"
	"syntopica-backend/internal/topicgraph/repository"
)

// BuildTopicGraph builds a topic graph for a given time window.
// Delegates to the repository layer.
func BuildTopicGraph(kind string, anchor time.Time, categoryID, feedID *uint) (*tagging.TopicGraphResponse, error) {
	return repository.Repo.BuildTopicGraph(kind, anchor, categoryID, feedID)
}

// BuildTopicDetail builds a detailed view of a topic.
// Delegates to the repository layer.
func BuildTopicDetail(kind string, slug string, anchor time.Time, categoryID, feedID *uint) (*tagging.TopicDetail, error) {
	return repository.Repo.BuildTopicDetail(kind, slug, anchor, categoryID, feedID)
}

// BuildTopicsByCategory builds topic lists grouped by category from article tags.
// Delegates to the repository layer.
func BuildTopicsByCategory(kind string, anchor time.Time, categoryID, feedID *uint) (*tagging.TopicsByCategoryResult, error) {
	return repository.Repo.BuildTopicsByCategory(kind, anchor, categoryID, feedID)
}

// FetchTopicArticles retrieves paginated articles for a topic.
// Delegates to the repository layer.
func FetchTopicArticles(slug string, kind string, anchor time.Time, page, pageSize int) ([]tagging.TopicArticleCard, int64, error) {
	return repository.Repo.FetchTopicArticles(slug, kind, anchor, page, pageSize)
}

// GetPendingArticlesByTag retrieves articles that have the given tag but are not in any digest.
// Delegates to the repository layer.
func GetPendingArticlesByTag(tagSlug string, kind string, anchor time.Time) (*tagging.PendingArticlesResponse, error) {
	return repository.Repo.GetPendingArticlesByTag(tagSlug, kind, anchor)
}

// GetDigestsByArticleTag retrieves digests for a given tag.
// Delegates to the repository layer.
func GetDigestsByArticleTag(tagSlug string, windowKind string, anchor time.Time, limit int, tagKind string) ([]repository.HotspotDigestCard, error) {
	return repository.Repo.GetDigestsByArticleTag(tagSlug, windowKind, anchor, limit, tagKind)
}

// CollectBoardIDsForDate returns all board IDs that have active event tags on a date.
// Delegates to the repository layer.
func CollectBoardIDsForDate(date time.Time) ([]uint, error) {
	return repository.Repo.CollectBoardIDsForDate(date)
}

// SaveReport saves a daily report and its sections.
// Delegates to the repository layer.
func SaveReport(report *repository.BoardDailyReport, sections []repository.DailyReportSection, threadBatches [][]repository.DailyReportThread) error {
	return repository.Repo.SaveReport(report, sections, threadBatches)
}

// ListReports returns recent reports for a board.
// Delegates to the repository layer.
func ListReports(boardID uint, days int) ([]repository.ReportListItem, error) {
	return repository.Repo.ListReports(boardID, days)
}

// GetReportByID retrieves a single daily report by its primary key.
// Delegates to the repository layer.
func GetReportByID(id uint) (*repository.BoardDailyReport, error) {
	return repository.Repo.GetReportByID(id)
}

// GetBoardSectionTimeline fetches all sections and their relations for a board within a date range.
func GetBoardSectionTimeline(boardID uint, days int) (repository.SectionTimelineResponse, error) {
	return repository.Repo.GetBoardSectionTimeline(boardID, days)
}

// GetSectionLifecycle fetches the clicked section and its directly connected neighbors (1 hop).
func GetSectionLifecycle(sectionID uint) (repository.SectionTimelineResponse, error) {
	return repository.Repo.GetSectionLifecycle(sectionID)
}

// GetCategoryColor returns the color for a given category.
func GetCategoryColor(category string) string {
	switch category {
	case "event":
		return "#f59e0b" // amber
	case "person":
		return "#10b981" // emerald
	case "keyword":
		return "#6366f1" // indigo
	default:
		return "#6366f1" // default to indigo for unknown categories
	}
}
