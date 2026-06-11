package tagging

import (
	"gorm.io/gorm"

	"syntopica-backend/internal/tagmanagement/handler"
	"syntopica-backend/internal/tagmanagement/repository"
	"syntopica-backend/internal/tagmanagement/service"
)

// ============================================================================
// Package-level wiring
// ============================================================================

var Repo *repository.TagManagementRepository

func InitRepository(db *gorm.DB) {
	repository.InitRepository(db)
	Repo = repository.Repo
}

func NewTagManagementRepository(db *gorm.DB) *repository.TagManagementRepository {
	return repository.NewTagManagementRepository(db)
}

// ============================================================================
// Type aliases from service/
// ============================================================================

type (
	TopicTag                 = service.TopicTag
	AggregatedTopicTag       = service.AggregatedTopicTag
	ExtractedTag             = service.ExtractedTag
	ExtractionInput          = service.ExtractionInput
	GraphNode                = service.GraphNode
	GraphEdge                = service.GraphEdge
	TopicArticleCard         = service.TopicArticleCard
	TopicTagSummary          = service.TopicTagSummary
	TopicHistoryPoint        = service.TopicHistoryPoint
	TopicDetail              = service.TopicDetail
	RelatedTag               = service.RelatedTag
	TopicsByCategoryResult   = service.TopicsByCategoryResult
	PendingArticle           = service.PendingArticle
	PendingArticlesResponse  = service.PendingArticlesResponse
	TopicGraphResponse       = service.TopicGraphResponse
	GetTopicArticlesParams   = service.GetTopicArticlesParams
	TagResolutionRequest     = service.TagResolutionRequest
	TagResolutionResponse    = service.TagResolutionResponse
	SimilarTagInfo           = service.SimilarTagInfo
	SimilarityEdge           = service.SimilarityEdge
	TagMatchResult           = service.TagMatchResult
	TagCandidate             = service.TagCandidate
	SemanticBoardMatchResult = service.SemanticBoardMatchResult
	AuxLabelGCMode           = service.AuxLabelGCMode
	AuxLabelGCRequest        = service.AuxLabelGCRequest
)

const (
	AuxLabelGCModeDryRun      = service.AuxLabelGCModeDryRun
	AuxLabelGCModeDisable     = service.AuxLabelGCModeDisable
	AuxLabelGCModeDelete      = service.AuxLabelGCModeDelete
	AuxLabelGCModeRecalculate = service.AuxLabelGCModeRecalculate
)

// ============================================================================
// Function / variable re-exports
// ============================================================================

var (
	ParseAnchorDate = service.ParseAnchorDate
	ResolveWindow   = service.ResolveWindow
	StartAllWorkers = service.StartAllWorkers
	StopAllWorkers  = service.StopAllWorkers
)

var (
	FeedCategoryName          = service.FeedCategoryName
	GetArticleTags            = service.GetArticleTags
	CleanupOrphanedTags       = service.CleanupOrphanedTags
	NormalizeDisplayCategory  = service.NormalizeDisplayCategory
	RegisterVectorDimEnsurer  = service.RegisterVectorDimEnsurer
	EnsureVectorDimensionOnce = service.EnsureVectorDimensionOnce
)

var (
	NewTagJobQueue = repository.NewTagJobQueue
)

// Re-export TagJobRequest from repository
type TagJobRequest = repository.TagJobRequest

var (
	NewAuxiliaryLabelService        = service.NewAuxiliaryLabelService
	NewSemanticBoardMatchingService = service.NewSemanticBoardMatchingService
	MatchTier                       = handler.MatchTier
)

// ============================================================================
// Handler route registrations
// ============================================================================

var (
	RegisterWatchedTagsRoutes           = handler.RegisterWatchedTagsRoutes
	RegisterTagManagementRoutes         = handler.RegisterTagManagementRoutes
	RegisterTagMergePreviewRoutes       = handler.RegisterTagMergePreviewRoutes
	RegisterTagQueueRoutes              = handler.RegisterTagQueueRoutes
	RegisterSemanticBoardRoutes         = handler.RegisterSemanticBoardRoutes
	RegisterEmbeddingConfigRoutes       = handler.RegisterEmbeddingConfigRoutes
	RegisterEmbeddingQueueRoutes        = handler.RegisterEmbeddingQueueRoutes
	RegisterMergeReembeddingQueueRoutes = handler.RegisterMergeReembeddingQueueRoutes
)
