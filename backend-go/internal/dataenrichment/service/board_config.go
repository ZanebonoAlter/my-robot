package service

import (
	"context"
	"fmt"
)

// BoardEnrichmentConfig holds the cycle-B analysis configuration for a board.
// See design.md §11, decision ④ (fields live on SemanticLabel / semantic_labels table).
type BoardEnrichmentConfig struct {
	EnrichmentEnabled bool     `json:"enrichment_enabled"` // default false
	WindowDays        int      `json:"window_days"`        // default 14
	ContextLayers     []string `json:"context_layers"`     // default ["week","month","year","all"]
}

// BoardConfigReader looks up a board's enrichment configuration from a topic ID.
// The production implementation queries board_persistent_topics → semantic_labels.
// Mocks satisfy this interface for TDD without a database.
type BoardConfigReader interface {
	GetBoardConfig(ctx context.Context, topicID uint) (*BoardEnrichmentConfig, error)
}

// ── Default config ──────────────────────────────────────────────────────────

// DefaultBoardConfig returns the default enrichment config for a board.
func DefaultBoardConfig() *BoardEnrichmentConfig {
	return &BoardEnrichmentConfig{
		EnrichmentEnabled: false,
		WindowDays:        14,
		ContextLayers:     []string{"week", "month", "year", "all"},
	}
}

// Validate returns an error if the config is semantically invalid.
func (c *BoardEnrichmentConfig) Validate() error {
	if c.WindowDays < 1 || c.WindowDays > 365 {
		return fmt.Errorf("window_days must be 1-365, got %d", c.WindowDays)
	}
	for _, layer := range c.ContextLayers {
		switch layer {
		case "week", "month", "year", "all":
		default:
			return fmt.Errorf("unknown context_layer: %s", layer)
		}
	}
	return nil
}
