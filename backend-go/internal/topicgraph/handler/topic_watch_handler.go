package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/topicgraph/repository"
	"syntopica-backend/internal/topicgraph/service"
)

// topicWatchCreateData augments a keyword-watch creation response with the
// synchronous history-rescan count while preserving the normal watch shape.
type topicWatchCreateData struct {
	repository.BoardTopicWatch
	InstantHitCount int `json:"instant_hit_count"`
}

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

// createTopicWatch handles POST /api/semantic-boards/:id/topic-watches.
// Body: {"label": "...", "type": "label"|"keyword"|"keyword_topic"|"sentence_topic", "query": "..."}
// — type is optional and defaults to "label" (backward compatible with old
// clients). keyword / keyword_topic additionally require a parseable
// expression (at least one valid OR group, e.g. "ASML|镓锗 出口"); a keyword
// watch (hint track) once created synchronously triggers the instant match
// over the last 14 days — instant-match failure is swallowed (logged) so the
// watch itself always survives. sentence_topic carries an optional query
// (retrieval sentence, falls back to label); its embedding cache is filled
// lazily at the next daily-report generation when omitted/failed.
func createTopicWatch(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid board id"})
		return
	}

	var req struct {
		Label string `json:"label"`
		Type  string `json:"type"`
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Label == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "label is required"})
		return
	}

	watchType := req.Type
	if watchType == "" {
		watchType = repository.WatchTypeLabel
	}
	switch watchType {
	case repository.WatchTypeLabel:
		// no extra validation
	case repository.WatchTypeKeyword, repository.WatchTypeKeywordTopic:
		if !service.ValidateKeywordExpr(req.Label) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "keyword expression is invalid: need at least one non-empty term (space=AND, '|'=OR)"})
			return
		}
	case repository.WatchTypeSentenceTopic:
		if strings.TrimSpace(req.Query) == "" && strings.TrimSpace(req.Label) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "sentence_topic requires a non-empty query (or label as fallback)"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "type must be 'label', 'keyword', 'keyword_topic' or 'sentence_topic'"})
		return
	}

	watch, err := repository.Repo.CreateWatch(repository.CreateWatchInput{
		SemanticBoardID: uint(boardID),
		Label:           req.Label,
		Type:            watchType,
		Query:           req.Query,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	if watchType == repository.WatchTypeKeyword {
		// Instant match over the last 14 days so the user sees historical
		// hits without waiting for the next daily report. Non-fatal by
		// design: a failure is logged and swallowed — the watch row above
		// MUST survive (design §4.4 / migration plan step 8).
		instantHits, instantErr := service.MatchKeywordInstant(c.Request.Context(), uint(boardID), watch.ID, service.KeywordInstantWindowDays)
		if instantErr != nil {
			logging.Warnf("topic-watch: instant keyword match failed for board %d watch %d: %v", boardID, watch.ID, instantErr)
			instantHits = 0
		}
		// instant_hit_count belongs inside data so the API normalizer can
		// deserialize it together with the watch row.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": topicWatchCreateData{
			BoardTopicWatch: *watch,
			InstantHitCount: instantHits,
		}})
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
		Query  *string `json:"query"`
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

	watch, err := repository.Repo.UpdateWatch(uint(watchID), req.Label, req.Query, req.Status)
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

	watch, err := repository.Repo.GetWatchByID(uint(watchID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	// sentence_topic: archive the dedicated topic first — an explicit user
	// confirmation is required (spec: 删除一句话轨确认归档，不得静默归档).
	// Historical materialized sections keep their assignment (snapshots are
	// immutable per the topic-graph invariants).
	if watch.Type == repository.WatchTypeSentenceTopic {
		if v := c.Query("confirm_archive_topic"); v != "true" && v != "1" {
			topicLabel := watch.Label
			if watch.PersistentTopicID != nil {
				topicLabel = fmt.Sprintf("%s（话题 #%d）", watch.Label, *watch.PersistentTopicID)
			}
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("删除该关注需确认同时归档其专属话题「%s」：请携带 confirm_archive_topic=true 重试", topicLabel)})
			return
		}
		if watch.PersistentTopicID != nil {
			if err := service.ArchiveWatchTopic(c.Request.Context(), uint(watchID)); err != nil {
				logging.Warnf("topic-watch: archive watch topic failed for watch %d: %v", watchID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
				return
			}
		}
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
