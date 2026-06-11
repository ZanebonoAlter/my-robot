package tagging

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all tagmanagement module routes under the given router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	RegisterEmbeddingConfigRoutes(rg)
	RegisterEmbeddingQueueRoutes(rg)
	RegisterMergeReembeddingQueueRoutes(rg)
	RegisterTagQueueRoutes(rg)
	RegisterTagManagementRoutes(rg)
	RegisterWatchedTagsRoutes(rg)
	RegisterTagMergePreviewRoutes(rg)
	RegisterSemanticBoardRoutes(rg)
}
