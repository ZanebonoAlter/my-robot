package core

import (
	"context"

	"gorm.io/gorm"
)

// ============================================================================
// Cross-package function bridges.
// These function variables break import cycles between core/ and sub-packages
// (auxlabel, board, merge). Sub-packages set these during init().
// ============================================================================

// AuxServiceFactory creates an auxiliary label service.
// Set by the auxlabel package during init.
var AuxServiceFactory func(db *gorm.DB, embedder interface{}) AuxService

// AuxService is the interface that auxlabel.AuxiliaryLabelService satisfies.
type AuxService interface {
	AttachAuxiliaryLabels(ctx context.Context, tagID uint, labels []AuxiliaryLabel) error
	RecountRefs(ctx context.Context, auxLabelIDs []uint) error
}

// BoardMatcherFactory returns a board matching service singleton.
// Set by the board package during init.
var BoardMatcherFactory func() BoardMatcher

// BoardMatcher is the interface that board.SemanticBoardMatchingService satisfies.
type BoardMatcher interface {
	MatchTopicTag(ctx context.Context, topicTagID uint) (interface{}, error)
}
