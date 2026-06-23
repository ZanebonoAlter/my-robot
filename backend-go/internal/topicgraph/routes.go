package topicgraph

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all topicgraph module routes under the given router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	RegisterDailyReportRoutes(rg)
}
