package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// Repo is the package-level repository singleton.
var Repo *Repository

// Repository provides data access for data enrichment entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new repository with the given DB connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// InitRepo creates and stores the global repository singleton.
func InitRepo(db *gorm.DB) {
	Repo = NewRepository(db)
}

// DB returns the underlying *gorm.DB.
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// SetRepo replaces the global repo (for testing).
func SetRepo(r *Repository) {
	Repo = r
}

// ── BoardDataSource ──────────────────────────────────────────────────────────

// CreateBoardDataSource inserts a new board data source.
func (r *Repository) CreateBoardDataSource(ctx context.Context, ds *BoardDataSource) error {
	if err := ValidateSourceType(ds.SourceType); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(ds).Error
}

// GetBoardDataSourceByBoardAndType fetches a single data source by board + source type.
func (r *Repository) GetBoardDataSourceByBoardAndType(ctx context.Context, boardID uint, sourceType string) (*BoardDataSource, error) {
	var ds BoardDataSource
	err := r.db.WithContext(ctx).
		Where("semantic_board_id = ? AND source_type = ?", boardID, sourceType).
		First(&ds).Error
	if err != nil {
		return nil, fmt.Errorf("get board data source: %w", err)
	}
	return &ds, nil
}

// UpsertBoardDataSource creates or updates a data source (by board_id + source_type unique key).
// Wraps read-then-write in a transaction to avoid TOCTOU race.
func (r *Repository) UpsertBoardDataSource(ctx context.Context, ds *BoardDataSource) error {
	if err := ValidateSourceType(ds.SourceType); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing BoardDataSource
		err := tx.Where("semantic_board_id = ? AND source_type = ?",
			ds.SemanticBoardID, ds.SourceType).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(ds).Error
		}
		if err != nil {
			return err
		}
		ds.ID = existing.ID
		ds.CreatedAt = existing.CreatedAt
		return tx.Save(ds).Error
	})
}

// ListBoardDataSourcesByBoardID returns all data sources for a board.
func (r *Repository) ListBoardDataSourcesByBoardID(ctx context.Context, boardID uint) ([]BoardDataSource, error) {
	var list []BoardDataSource
	err := r.db.WithContext(ctx).
		Where("semantic_board_id = ?", boardID).
		Order("source_type ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list board data sources: %w", err)
	}
	return list, nil
}

// DeleteBoardDataSource removes a data source by ID.
func (r *Repository) DeleteBoardDataSource(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&BoardDataSource{}, id).Error
}

// ── TopicLifelineContext ────────────────────────────────────────────────────

// UpsertTopicLifelineContext creates or updates a lifeline context (by topic_id + granularity + period).
// Wraps read-then-write in a transaction to avoid TOCTOU race.
func (r *Repository) UpsertTopicLifelineContext(ctx context.Context, lc *TopicLifelineContext) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing TopicLifelineContext
		err := tx.Where("persistent_topic_id = ? AND granularity = ? AND period = ?",
			lc.PersistentTopicID, lc.Granularity, lc.Period).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(lc).Error
		}
		if err != nil {
			return err
		}
		lc.ID = existing.ID
		lc.CreatedAt = existing.CreatedAt
		return tx.Save(lc).Error
	})
}

// GetTopicLifelineContext fetches a single lifeline context by topic + granularity + period.
func (r *Repository) GetTopicLifelineContext(ctx context.Context, topicID uint, granularity, period string) (*TopicLifelineContext, error) {
	var lc TopicLifelineContext
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ? AND granularity = ? AND period = ?", topicID, granularity, period).
		First(&lc).Error
	if err != nil {
		return nil, fmt.Errorf("get topic lifeline context: %w", err)
	}
	return &lc, nil
}

// GetTopicLifelineContextLatest returns the row with the highest period for a given granularity.
// Used by orchestrator readContextLayers to pick the freshest period.
func (r *Repository) GetTopicLifelineContextLatest(ctx context.Context, topicID uint, granularity string) (*TopicLifelineContext, error) {
	var lc TopicLifelineContext
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ? AND granularity = ?", topicID, granularity).
		Order("period DESC").
		First(&lc).Error
	if err != nil {
		return nil, fmt.Errorf("get latest topic lifeline context: %w", err)
	}
	return &lc, nil
}

// ListTopicLifelineContextsByTopic returns all granularity+period contexts for a topic.
func (r *Repository) ListTopicLifelineContextsByTopic(ctx context.Context, topicID uint) ([]TopicLifelineContext, error) {
	var list []TopicLifelineContext
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ?", topicID).
		Order("granularity ASC, period DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list topic lifeline contexts: %w", err)
	}
	return list, nil
}

// ListTopicLifelineContextsByGranularity returns all periods for a topic + granularity.
func (r *Repository) ListTopicLifelineContextsByGranularity(ctx context.Context, topicID uint, granularity string) ([]TopicLifelineContext, error) {
	var list []TopicLifelineContext
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ? AND granularity = ?", topicID, granularity).
		Order("period DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list lifeline contexts by granularity: %w", err)
	}
	return list, nil
}

// DeleteTopicLifelineContextsOlderThan deletes rows whose period is before the cutoff
// for a given granularity. Used by archive/prune logic.
func (r *Repository) DeleteTopicLifelineContextsOlderThan(ctx context.Context, granularity, cutoffPeriod string) error {
	return r.db.WithContext(ctx).
		Where("granularity = ? AND period < ?", granularity, cutoffPeriod).
		Delete(&TopicLifelineContext{}).Error
}

// ── TopicEnrichmentResult ───────────────────────────────────────────────────

// CreateTopicEnrichmentResult inserts a new immutable result snapshot.
func (r *Repository) CreateTopicEnrichmentResult(ctx context.Context, result *TopicEnrichmentResult) error {
	return r.db.WithContext(ctx).Create(result).Error
}

// ListTopicEnrichmentResultsByTopic returns all results for a topic, newest first.
func (r *Repository) ListTopicEnrichmentResultsByTopic(ctx context.Context, topicID uint) ([]TopicEnrichmentResult, error) {
	var list []TopicEnrichmentResult
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ?", topicID).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list enrichment results: %w", err)
	}
	return list, nil
}

// GetLatestTopicEnrichmentResult returns the newest result for a topic.
func (r *Repository) GetLatestTopicEnrichmentResult(ctx context.Context, topicID uint) (*TopicEnrichmentResult, error) {
	var result TopicEnrichmentResult
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ?", topicID).
		Order("id DESC").
		First(&result).Error
	if err != nil {
		return nil, fmt.Errorf("get latest enrichment result: %w", err)
	}
	return &result, nil
}

// GetTopicEnrichmentResultByID fetches a single result by ID.
func (r *Repository) GetTopicEnrichmentResultByID(ctx context.Context, id uint) (*TopicEnrichmentResult, error) {
	var result TopicEnrichmentResult
	err := r.db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, fmt.Errorf("get enrichment result: %w", err)
	}
	return &result, nil
}

// GetPrevLatestTopicEnrichmentResult returns the newest result before the given ID.
func (r *Repository) GetPrevLatestTopicEnrichmentResult(ctx context.Context, topicID uint, beforeID uint) (*TopicEnrichmentResult, error) {
	var result TopicEnrichmentResult
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ? AND id < ?", topicID, beforeID).
		Order("id DESC").
		First(&result).Error
	if err != nil {
		return nil, fmt.Errorf("get prev enrichment result: %w", err)
	}
	return &result, nil
}

// ── TopicEnrichmentReview ───────────────────────────────────────────────────

// CreateTopicEnrichmentReview inserts a new review.
func (r *Repository) CreateTopicEnrichmentReview(ctx context.Context, review *TopicEnrichmentReview) error {
	return r.db.WithContext(ctx).Create(review).Error
}

// ListTopicEnrichmentReviewsByTopic returns all reviews for a topic, newest first.
func (r *Repository) ListTopicEnrichmentReviewsByTopic(ctx context.Context, topicID uint) ([]TopicEnrichmentReview, error) {
	var list []TopicEnrichmentReview
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ?", topicID).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list enrichment reviews: %w", err)
	}
	return list, nil
}

// ListAppliedTopicEnrichmentReviews returns reviews with applied=true for a topic.
func (r *Repository) ListAppliedTopicEnrichmentReviews(ctx context.Context, topicID uint) ([]TopicEnrichmentReview, error) {
	var list []TopicEnrichmentReview
	err := r.db.WithContext(ctx).
		Where("persistent_topic_id = ? AND applied = ?", topicID, true).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list applied reviews: %w", err)
	}
	return list, nil
}

// UpdateTopicEnrichmentReviewDeviation updates the deviation_summary of a review.
func (r *Repository) UpdateTopicEnrichmentReviewDeviation(ctx context.Context, id uint, summary string) error {
	return r.db.WithContext(ctx).
		Model(&TopicEnrichmentReview{}).
		Where("id = ?", id).
		Update("deviation_summary", summary).Error
}

// ApplyTopicEnrichmentReview sets applied=true on a review.
func (r *Repository) ApplyTopicEnrichmentReview(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&TopicEnrichmentReview{}).
		Where("id = ?", id).
		Update("applied", true).Error
}

// GetTopicEnrichmentReviewByID fetches a single review by ID.
func (r *Repository) GetTopicEnrichmentReviewByID(ctx context.Context, id uint) (*TopicEnrichmentReview, error) {
	var review TopicEnrichmentReview
	err := r.db.WithContext(ctx).First(&review, id).Error
	if err != nil {
		return nil, fmt.Errorf("get enrichment review: %w", err)
	}
	return &review, nil
}

// ── StockDebateResult ─────────────────────────────────────────────────────

// CreateStockDebateResult inserts a new debate result.
func (r *Repository) CreateStockDebateResult(ctx context.Context, result *StockDebateResult) error {
	return r.db.WithContext(ctx).Create(result).Error
}

// ListStockDebateResultsByResult returns all debate results for a result ID, newest first.
func (r *Repository) ListStockDebateResultsByResult(ctx context.Context, resultID uint) ([]StockDebateResult, error) {
	var list []StockDebateResult
	err := r.db.WithContext(ctx).
		Where("topic_enrichment_result_id = ?", resultID).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list debate results: %w", err)
	}
	return list, nil
}
