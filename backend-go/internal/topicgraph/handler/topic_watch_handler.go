package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/topicgraph/repository"
)

// RegisterTopicWatchRoutes registers topic watch CRUD routes.
// Note: the board param must be named ":id" to match the existing
// "/api/semantic-boards/:id/*" routes — Gin's trie forbids two different
// param names at the same tree position.
func RegisterTopicWatchRoutes(api *gin.RouterGroup) {
	// POST /api/semantic-boards/:id/topic-watches
	api.POST("/semantic-boards/:id/topic-watches", createTopicWatch)
	// GET /api/semantic-boards/:id/topic-watches
	api.GET("/semantic-boards/:id/topic-watches", listTopicWatches)

	// PATCH /api/topic-watches/:id
	api.PATCH("/topic-watches/:id", updateTopicWatch)
	// DELETE /api/topic-watches/:id
	api.DELETE("/topic-watches/:id", deleteTopicWatch)

	// GET /api/daily-reports/:id/watch-hits
	api.GET("/daily-reports/:id/watch-hits", getWatchHits)
}

// createTopicWatch handles POST /api/semantic-boards/:id/topic-watches
func createTopicWatch(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid board id"})
		return
	}

	var req struct {
		Label string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Label == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "label is required"})
		return
	}

	watch, err := repository.Repo.CreateWatch(uint(boardID), req.Label)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": watch})
}

// listTopicWatches handles GET /api/semantic-boards/:id/topic-watches
func listTopicWatches(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid board id"})
		return
	}

	watches, err := repository.Repo.ListWatchesByBoard(uint(boardID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": watches})
}

// updateTopicWatch handles PATCH /api/topic-watches/:id
func updateTopicWatch(c *gin.Context) {
	watchID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid watch id"})
		return
	}

	var req struct {
		Label  *string `json:"label"`
		Status *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	if req.Status != nil && *req.Status != repository.WatchStatusActive && *req.Status != repository.WatchStatusPaused {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "status must be 'active' or 'paused'"})
		return
	}

	watch, err := repository.Repo.UpdateWatch(uint(watchID), req.Label, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": watch})
}

// deleteTopicWatch handles DELETE /api/topic-watches/:id
func deleteTopicWatch(c *gin.Context) {
	watchID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid watch id"})
		return
	}

	if err := repository.Repo.DeleteWatch(uint(watchID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}

// getWatchHits handles GET /api/daily-reports/:id/watch-hits
func getWatchHits(c *gin.Context) {
	reportID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid report id"})
		return
	}

	hits, err := repository.Repo.GetWatchHitsByReport(uint(reportID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": hits})
}
