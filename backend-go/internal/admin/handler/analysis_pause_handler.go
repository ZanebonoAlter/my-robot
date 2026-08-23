package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/platform/aihealth"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/aisettings"
	"syntopica-backend/internal/platform/analysispause"
	"syntopica-backend/internal/platform/database"
)

// GetAnalysisPause reports the current global analysis-pause state. The
// reported `paused` reflects the USER intent only (the persisted switch), not
// the effective pause that also factors in AI model health. The frontend
// button/favicon must follow user intent so it never flips because of health.
func GetAnalysisPause(c *gin.Context) {
	pausedAtStr := ""
	if pausedAt := analysispause.PausedAt(); pausedAt != nil {
		pausedAtStr = pausedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"paused":    analysispause.UserPaused(),
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

	// On resume (user clicks start), asynchronously re-probe so the health
	// gate can self-heal: a transient startup-probe failure (or models that
	// came up after startup) is re-evaluated when the user explicitly resumes.
	// Pause does not trigger a reprobe. The response still reflects user
	// intent only. (spec: 恢复时重新探活)
	if !req.Paused {
		autoStart, _ := aisettings.LoadAutoStartModelsConfig()
		go aihealth.RunStartupProbe(context.Background(), airouter.NewStore(database.DB), autoStart)
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
			"paused":    analysispause.UserPaused(),
			"paused_at": pausedAtStr,
		},
	})
}
