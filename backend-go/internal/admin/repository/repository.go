package repository

import (
	"time"

	"gorm.io/gorm"
	"syntopica-backend/internal/models"
)

// ============================================================================
// Package-level wiring
// ============================================================================

var Repo *AdminRepository

func InitRepository(db *gorm.DB) {
	Repo = NewAdminRepository(db)
}

// ============================================================================
// AdminRepository — centralized data access for the admin feature module
// ============================================================================

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// DB returns the underlying gorm.DB for complex ad-hoc queries.
func (r *AdminRepository) DB() *gorm.DB {
	return r.db
}

// ============================================================================
// AI Provider operations
// ============================================================================

func (r *AdminRepository) ListProviders() ([]models.AIProvider, error) {
	var providers []models.AIProvider
	err := r.db.Order("name ASC").Find(&providers).Error
	return providers, err
}

func (r *AdminRepository) GetProviderByID(id uint) (*models.AIProvider, error) {
	var provider models.AIProvider
	if err := r.db.First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *AdminRepository) UpsertProvider(p *models.AIProvider) error {
	return r.db.Save(p).Error
}

func (r *AdminRepository) DeleteProvider(id uint) error {
	return r.db.Delete(&models.AIProvider{}, id).Error
}

// ============================================================================
// AI Route operations
// ============================================================================

func (r *AdminRepository) ListRoutes() ([]models.AIRoute, error) {
	var routes []models.AIRoute
	err := r.db.Preload("RouteProviders.Provider").
		Preload("RouteProviders", func(db *gorm.DB) *gorm.DB {
			return db.Order("priority ASC")
		}).
		Order("capability ASC, name ASC").
		Find(&routes).Error
	return routes, err
}

func (r *AdminRepository) GetRouteByID(id uint) (*models.AIRoute, error) {
	var route models.AIRoute
	if err := r.db.Preload("RouteProviders.Provider").First(&route, id).Error; err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *AdminRepository) UpsertRoute(rt *models.AIRoute) error {
	return r.db.Save(rt).Error
}

func (r *AdminRepository) DeleteRoute(id uint) error {
	return r.db.Delete(&models.AIRoute{}, id).Error
}

// ============================================================================
// AI Settings operations
// ============================================================================

func (r *AdminRepository) GetAISettings(key string) (*models.AISettings, error) {
	var setting models.AISettings
	if err := r.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *AdminRepository) GetAllAISettings() (map[string]string, error) {
	var settings []models.AISettings
	if err := r.db.Order("key ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *AdminRepository) SaveAISettings(settings []models.AISettings) error {
	for _, s := range settings {
		var existing models.AISettings
		err := r.db.Where("key = ?", s.Key).First(&existing).Error
		switch err {
		case nil:
			existing.Value = s.Value
			if s.Description != "" {
				existing.Description = s.Description
			}
			if saveErr := r.db.Save(&existing).Error; saveErr != nil {
				return saveErr
			}
		case gorm.ErrRecordNotFound:
			if createErr := r.db.Create(&models.AISettings{
				Key:         s.Key,
				Value:       s.Value,
				Description: s.Description,
			}).Error; createErr != nil {
				return createErr
			}
		default:
			return err
		}
	}
	return nil
}

// ============================================================================
// Reading Behavior operations
// ============================================================================

func (r *AdminRepository) TrackReadingBehavior(b *models.ReadingBehavior) error {
	return r.db.Create(b).Error
}

func (r *AdminRepository) TrackReadingBehaviors(behaviors []models.ReadingBehavior) error {
	return r.db.Create(&behaviors).Error
}

func (r *AdminRepository) GetReadingStats(days int) (*ReadingStatsResult, error) {
	var stats ReadingStatsResult

	r.db.Model(&models.ReadingBehavior{}).
		Distinct("article_id").
		Count(&stats.TotalArticles)

	if err := r.db.Model(&models.ReadingBehavior{}).
		Select("COALESCE(SUM(reading_time), 0)").
		Scan(&stats.TotalReadingTime).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&models.ReadingBehavior{}).
		Where("reading_time > 0").
		Select("AVG(reading_time)").
		Scan(&stats.AvgReadingTime).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&models.ReadingBehavior{}).
		Where("scroll_depth > 0").
		Select("AVG(scroll_depth)").
		Scan(&stats.AvgScrollDepth).Error; err != nil {
		return nil, err
	}

	var feedCounts []struct {
		FeedID uint
		Count  int64
	}
	if err := r.db.Model(&models.ReadingBehavior{}).
		Select("feed_id, COUNT(*) as count").
		Group("feed_id").
		Order("count DESC").
		Limit(1).
		Scan(&feedCounts).Error; err != nil {
		return nil, err
	}
	if len(feedCounts) > 0 {
		stats.MostActiveFeedID = &feedCounts[0].FeedID
	}

	var categoryCounts []struct {
		CategoryID *uint
		Count      int64
	}
	if err := r.db.Model(&models.ReadingBehavior{}).
		Select("category_id, COUNT(*) as count").
		Where("category_id IS NOT NULL").
		Group("category_id").
		Order("count DESC").
		Limit(1).
		Scan(&categoryCounts).Error; err != nil {
		return nil, err
	}
	if len(categoryCounts) > 0 {
		stats.MostActiveCategory = categoryCounts[0].CategoryID
	}

	return &stats, nil
}

type ReadingStatsResult struct {
	TotalArticles      int64   `json:"total_articles"`
	TotalReadingTime   int     `json:"total_reading_time"`
	AvgReadingTime     float64 `json:"avg_reading_time"`
	AvgScrollDepth     float64 `json:"avg_scroll_depth"`
	MostActiveFeedID   *uint   `json:"most_active_feed_id"`
	MostActiveCategory *uint   `json:"most_active_category"`
}

func (r *AdminRepository) GetFeedByID(id uint) (*models.Feed, error) {
	var feed models.Feed
	if err := r.db.First(&feed, id).Error; err != nil {
		return nil, err
	}
	return &feed, nil
}

// ============================================================================
// Narrative operations
// ============================================================================

// BoardNarrativeRow — joined result of narrative_summaries + narrative_boards
type BoardNarrativeRow struct {
	models.NarrativeSummary
	BoardName        string    `json:"board_name"`
	BoardDescription string    `json:"board_description"`
	BoardPeriodDate  time.Time `json:"board_period_date"`
}

func (r *AdminRepository) ListNarrativesByDate(date string) ([]BoardNarrativeRow, error) {
	var rows []BoardNarrativeRow
	err := r.db.Table("narrative_summaries").
		Select("narrative_summaries.*, narrative_boards.name AS board_name, narrative_boards.description AS board_description, narrative_boards.period_date AS board_period_date").
		Joins("LEFT JOIN narrative_boards ON narrative_boards.id = narrative_summaries.board_id").
		Where("narrative_summaries.period_date = ?", date).
		Order("narrative_summaries.generation ASC, narrative_summaries.id ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *AdminRepository) GetNarrativeByID(id uint) (*models.NarrativeSummary, error) {
	var narrative models.NarrativeSummary
	if err := r.db.First(&narrative, id).Error; err != nil {
		return nil, err
	}
	return &narrative, nil
}

func (r *AdminRepository) DeleteNarrativeByDate(date time.Time, scopeType string, categoryID *uint) (int, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := r.db.Where("period = ? AND period_date >= ? AND period_date < ?", "daily", startOfDay, endOfDay)

	if scopeType != "" {
		query = query.Where("scope_type = ?", scopeType)
		if categoryID != nil {
			query = query.Where("scope_category_id = ?", *categoryID)
		}
	}

	result := query.Delete(&models.NarrativeSummary{})
	return int(result.RowsAffected), result.Error
}

func (r *AdminRepository) SaveNarrative(n *models.NarrativeSummary) error {
	return r.db.Save(n).Error
}

func (r *AdminRepository) SaveBatchNarratives(narratives []models.NarrativeSummary) error {
	if len(narratives) == 0 {
		return nil
	}
	return r.db.Create(&narratives).Error
}

func (r *AdminRepository) ListNarrativeBoards() ([]models.NarrativeBoard, error) {
	var boards []models.NarrativeBoard
	err := r.db.Order("period_date DESC, id ASC").Find(&boards).Error
	return boards, err
}

func (r *AdminRepository) GetNarrativeBoardByID(id uint) (*models.NarrativeBoard, error) {
	var board models.NarrativeBoard
	if err := r.db.First(&board, id).Error; err != nil {
		return nil, err
	}
	return &board, nil
}

type NarrativeScopeItem struct {
	CategoryID   uint   `json:"category_id"`
	CategoryName string `json:"category_name"`
	BoardCount   int    `json:"board_count"`
}

func (r *AdminRepository) ListNarrativeScopes(days int) ([]NarrativeScopeItem, error) {
	// This is a simplified version; the full scopes query is in narrative_service.go
	var scopes []NarrativeScopeItem
	now := time.Now()
	startOfAnchor := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	rangeStart := startOfAnchor.AddDate(0, 0, -(days - 1))
	rangeEnd := startOfAnchor.Add(24 * time.Hour)

	type scopeRow struct {
		ScopeType       string
		ScopeCategoryID uint
		ScopeLabel      string
		Count           int
	}
	var rows []scopeRow
	if err := r.db.Model(&models.NarrativeBoard{}).
		Select("scope_type, scope_category_id, scope_label, COUNT(*) as count").
		Where("period_date >= ? AND period_date < ?", rangeStart, rangeEnd).
		Group("scope_type, scope_category_id, scope_label").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		scopes = append(scopes, NarrativeScopeItem{
			CategoryID:   row.ScopeCategoryID,
			CategoryName: row.ScopeLabel,
			BoardCount:   row.Count,
		})
	}
	return scopes, nil
}

// ============================================================================
// Scheduler Task operations
// ============================================================================

func (r *AdminRepository) ListSchedulerTasks() ([]models.SchedulerTask, error) {
	var tasks []models.SchedulerTask
	err := r.db.Order("name ASC").Find(&tasks).Error
	return tasks, err
}

func (r *AdminRepository) GetSchedulerTaskByName(name string) (*models.SchedulerTask, error) {
	var task models.SchedulerTask
	if err := r.db.Where("name = ?", name).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *AdminRepository) SaveSchedulerTask(task *models.SchedulerTask) error {
	return r.db.Save(task).Error
}

func (r *AdminRepository) UpdateSchedulerTask(name string, updates map[string]interface{}) error {
	var task models.SchedulerTask
	if err := r.db.Where("name = ?", name).First(&task).Error; err != nil {
		return err
	}
	return r.db.Model(&task).Updates(updates).Error
}
