package dataenrichment

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/models"
)

// dbBoardConfigReader implements service.BoardConfigReader by looking up the
// semantic_board_id from board_persistent_topics and then reading the enrichment
// config fields (EnrichmentEnabled / WindowDays / ContextLayers) from the
// corresponding semantic_labels row.
type dbBoardConfigReader struct {
	db *gorm.DB
}

// NewDBBoardConfigReader creates a production BoardConfigReader backed by gorm.DB.
func NewDBBoardConfigReader(db *gorm.DB) service.BoardConfigReader {
	return &dbBoardConfigReader{db: db}
}

// GetBoardConfig returns the enrichment configuration for the board that owns topicID.
// Falls back to DefaultBoardConfig (enrichment_enabled=false) when the topic or its
// board cannot be found.
func (r *dbBoardConfigReader) GetBoardConfig(ctx context.Context, topicID uint) (*service.BoardEnrichmentConfig, error) {
	// 1. Look up semantic_board_id from board_persistent_topics.
	var boardID uint
	err := r.db.WithContext(ctx).
		Table("board_persistent_topics").
		Select("semantic_board_id").
		Where("id = ?", topicID).
		Limit(1).
		Scan(&boardID).Error
	if err != nil {
		return nil, fmt.Errorf("find board for topic %d: %w", topicID, err)
	}
	if boardID == 0 {
		return service.DefaultBoardConfig(), nil
	}

	// 2. Read enrichment config from semantic_labels.
	var label models.SemanticLabel
	err = r.db.WithContext(ctx).
		Where("id = ? AND label_type = ?", boardID, "board").
		First(&label).Error
	if err != nil {
		return nil, fmt.Errorf("find semantic_label %d: %w", boardID, err)
	}

	// 3. Read enabled board_data_sources and collect allowed tools.
	var sourceTypes []string
	err = r.db.WithContext(ctx).
		Table("board_data_sources").
		Where("semantic_board_id = ? AND enabled = true", boardID).
		Pluck("source_type", &sourceTypes).Error
	if err != nil {
		return nil, fmt.Errorf("find board_data_sources for board %d: %w", boardID, err)
	}

	allowedTools := make([]string, 0)
	seen := make(map[string]bool)
	for _, st := range sourceTypes {
		for _, tool := range service.ToolsForSourceType(st) {
			if !seen[tool] {
				seen[tool] = true
				allowedTools = append(allowedTools, tool)
			}
		}
	}

	return &service.BoardEnrichmentConfig{
		EnrichmentEnabled: label.EnrichmentEnabled,
		WindowDays:        label.WindowDays,
		ContextLayers:     label.ContextLayers,
		AllowedTools:      allowedTools,
	}, nil
}
