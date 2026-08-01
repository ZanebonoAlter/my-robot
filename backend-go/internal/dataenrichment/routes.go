package dataenrichment

import (
	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/dataenrichment/handler"
)

// RegisterRoutes registers data enrichment routes.
func RegisterRoutes(rg *gin.RouterGroup) {
	handler.RegisterRoutes(rg)
}
