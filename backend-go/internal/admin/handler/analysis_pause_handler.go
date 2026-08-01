package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/platform/analysispause"
)

// GetAnalysisPause reports the current global analysis-pause state.
func GetAnalysisPause(c *gin.Context) {
	pausedAtStr := ""
	if pausedAt := analysispause.PausedAt(); pausedAt != nil {
		pausedAtStr = pausedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"paused":    analysispause.IsPaused(),
			"paused_at": pausedAtStr,
		},
	})
}

// SetAnalysisPauseRequest is the request body for SetAnalysisPause.
type SetAnalysisPauseRequest struct {
	Paused bool `json:"paused"`
}

// SetAnalysisPause engages (true) or releases (false) the global analysis
// pause, then returns the resulting state.
func SetAnalysisPause(c *gin.Context) {
	var req SetAnalysisPauseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := analysispause.SetPaused(req.Paused); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	message := "分析已恢复"
	if req.Paused {
		message = "分析已暂停"
	}

	pausedAtStr := ""
	if pausedAt := analysispause.PausedAt(); pausedAt != nil {
		pausedAtStr = pausedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"paused":    analysispause.IsPaused(),
			"paused_at": pausedAtStr,
		},
	})
}
