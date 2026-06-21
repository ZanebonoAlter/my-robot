package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/ws"
	"syntopica-backend/internal/topicgraph/repository"
	"syntopica-backend/internal/topicgraph/service"
)

// RegisterDailyReportRoutes registers all daily report routes.
func RegisterDailyReportRoutes(api *gin.RouterGroup) {
	// POST /api/daily-reports/generate
	api.POST("/daily-reports/generate", triggerGenerateDailyReport)

	// GET /api/daily-reports/:id
	api.GET("/daily-reports/:id", getDailyReport)

	// GET /api/semantic-boards/:id/daily-reports
	api.GET("/semantic-boards/:id/daily-reports", listBoardDailyReports)

	// GET /api/semantic-boards/:id/section-timeline
	api.GET("/semantic-boards/:id/section-timeline", getBoardSectionTimeline)

	// GET /api/daily-reports/sections/:id/lifecycle
	api.GET("/daily-reports/sections/:id/lifecycle", getSectionLifecycle)

	// GET /api/daily-reports/topics/:id/lifeline — all sections of one persistent
	// topic (no day limit), aggregated by persistent_topic_id.
	api.GET("/daily-reports/topics/:id/lifeline", getTopicLifeline)

	// PATCH /api/daily-reports/topics/:id — manual rename / archive / reactivate.
	api.PATCH("/daily-reports/topics/:id", updateTopic)
	// POST /api/daily-reports/topics/:id/merge — merge source topics into :id.
	api.POST("/daily-reports/topics/:id/merge", mergeTopic)
	// POST /api/daily-reports/topics/:id/split — carve sections into a new topic.
	api.POST("/daily-reports/topics/:id/split", splitTopic)

	// POST /api/daily-reports/backfill-embeddings
	api.POST("/daily-reports/backfill-embeddings", triggerBackfillEmbeddings)

	// POST /api/daily-reports/backfill-relations
	api.POST("/daily-reports/backfill-relations", triggerBackfillRelations)

	// POST /api/daily-reports/backfill-topics — reconstruct persistent topics
	// from historical sections. Optional ?board_id scopes to one board.
	api.POST("/daily-reports/backfill-topics", triggerBackfillTopics)
}

// triggerGenerateDailyReport handles POST /api/daily-reports/generate
func triggerGenerateDailyReport(c *gin.Context) {
	var req struct {
		Date    string `json:"date"`
		BoardID *uint  `json:"board_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Date is optional, defaults to today
		req.Date = ""
	}

	var date time.Time
	if req.Date != "" {
		parsed, err := time.ParseInLocation("2006-01-02", req.Date, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid date format, use YYYY-MM-DD"})
			return
		}
		date = parsed
	} else {
		date = time.Now()
	}

	jobID := uuid.New().String()

	if req.BoardID != nil {
		// Generate for single board
		go generateSingleBoard(*req.BoardID, date, jobID)
	} else {
		// Generate for all boards
		go generateAllBoards(date, jobID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"job_id": jobID,
			"status": "processing",
		},
	})
}

func generateSingleBoard(boardID uint, date time.Time, jobID string) {
	ctx, cancel := timeoutCtx(10 * time.Minute)
	defer cancel()

	boardName := dailyReportBoardName(boardID)
	broadcastProgress(jobID, "generating", boardID, boardName, 0, "0/1")

	report, sections, threadBatches, err := service.GenerateDailyReport(ctx, boardID, date)
	if err != nil {
		logging.Errorf("daily-report: generate failed for board %d: %v", boardID, err)
		broadcastProgress(jobID, "failed", boardID, boardName, 0, "1/1")
		broadcastDone(jobID, 0, 1)
		return
	}
	if report == nil {
		broadcastProgress(jobID, "completed", boardID, boardName, 0, "1/1")
		broadcastDone(jobID, 0, 1)
		return
	}

	if err := repository.Repo.SaveReport(report, sections, threadBatches); err != nil {
		logging.Errorf("daily-report: save failed for board %d: %v", boardID, err)
		broadcastProgress(jobID, "failed", boardID, boardName, 0, "1/1")
		broadcastDone(jobID, 0, 1)
		return
	}

	broadcastProgress(jobID, "completed", boardID, boardName, 1, "1/1")
	broadcastDone(jobID, 1, 1)
}

func generateAllBoards(date time.Time, jobID string) {
	ctx, cancel := timeoutCtx(30 * time.Minute)
	defer cancel()

	boardIDs, err := repository.Repo.CollectBoardIDsForDate(date)
	if err != nil {
		logging.Errorf("daily-report: collect boards failed: %v", err)
		broadcastProgress(jobID, "failed", 0, "All boards", 0, "0/0")
		broadcastDone(jobID, 0, 0)
		return
	}

	totalBoards := len(boardIDs)
	if totalBoards == 0 {
		broadcastDone(jobID, 0, 0)
		return
	}

	savedCount := 0
	for idx, boardID := range boardIDs {
		boardName := dailyReportBoardName(boardID)
		broadcastProgress(jobID, "generating", boardID, boardName, savedCount, fmt.Sprintf("%d/%d", idx, totalBoards))

		report, sections, threadBatches, genErr := service.GenerateDailyReport(ctx, boardID, date)
		if genErr != nil {
			logging.Warnf("daily-report: generate failed for board %d: %v", boardID, genErr)
			broadcastProgress(jobID, "failed", boardID, boardName, savedCount, fmt.Sprintf("%d/%d", idx+1, totalBoards))
			continue
		}
		if report == nil {
			broadcastProgress(jobID, "completed", boardID, boardName, savedCount, fmt.Sprintf("%d/%d", idx+1, totalBoards))
			continue
		}

		if saveErr := repository.Repo.SaveReport(report, sections, threadBatches); saveErr != nil {
			logging.Warnf("daily-report: save failed for board %d: %v", boardID, saveErr)
			broadcastProgress(jobID, "failed", boardID, boardName, savedCount, fmt.Sprintf("%d/%d", idx+1, totalBoards))
			continue
		}
		savedCount++
		broadcastProgress(jobID, "completed", boardID, boardName, savedCount, fmt.Sprintf("%d/%d", idx+1, totalBoards))
	}

	broadcastDone(jobID, savedCount, totalBoards)
}

// listBoardDailyReports handles GET /api/semantic-boards/:id/daily-reports
func listBoardDailyReports(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid board id"})
		return
	}

	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 7
	}

	reports, err := repository.Repo.ListReports(uint(boardID), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	if reports == nil {
		reports = []repository.ReportListItem{}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"reports": reports}})
}

// getDailyReport handles GET /api/daily-reports/:id
func getDailyReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid report id"})
		return
	}

	report, err := repository.Repo.GetReportByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "report not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"report": report}})
}

// getBoardSectionTimeline handles GET /api/semantic-boards/:id/section-timeline
func getBoardSectionTimeline(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid board id"})
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "14"))

	resp, err := repository.Repo.GetBoardSectionTimeline(uint(boardID), days)
	if err != nil {
		logging.Errorf("get board section timeline: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to get section timeline"})
		return
	}
	if resp.Sections == nil {
		resp.Sections = []repository.SectionTimelineNode{}
	}
	if resp.Relations == nil {
		resp.Relations = []repository.SectionRelationResult{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"sections":  resp.Sections,
		"relations": resp.Relations,
	}})
}

// getSectionLifecycle handles GET /api/daily-reports/sections/:id/lifecycle
func getSectionLifecycle(c *gin.Context) {
	sectionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid section id"})
		return
	}

	resp, err := repository.Repo.GetSectionLifecycle(uint(sectionID))
	if err != nil {
		logging.Errorf("get section lifecycle: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to get section lifecycle"})
		return
	}
	if resp.Sections == nil {
		resp.Sections = []repository.SectionTimelineNode{}
	}
	if resp.Relations == nil {
		resp.Relations = []repository.SectionRelationResult{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"sections":  resp.Sections,
		"relations": resp.Relations,
	}})
}

// getTopicLifeline handles GET /api/daily-reports/topics/:id/lifeline.
// Returns all sections of a persistent topic aggregated by topic id, with the
// internal relations among them. Unlike section lifecycle (embedding-graph
// based), this is identity-key based and survives label drift.
func getTopicLifeline(c *gin.Context) {
	topicIDStr := c.Param("id")
	topicID, err := strconv.ParseUint(topicIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid topic id"})
		return
	}

	resp, err := repository.Repo.GetTopicLifeline(uint(topicID))
	if err != nil {
		logging.Errorf("get topic lifeline: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to get topic lifeline"})
		return
	}
	if resp.Sections == nil {
		resp.Sections = []repository.SectionTimelineNode{}
	}
	if resp.Relations == nil {
		resp.Relations = []repository.SectionRelationResult{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"sections":  resp.Sections,
		"relations": resp.Relations,
	}})
}

// parseTopicID extracts and validates the :id path param as a topic id. Writes
// the error response when invalid, so callers return immediately on err.
func parseTopicID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid topic id"})
		return 0, err
	}
	return uint(id), nil
}

// updateTopic handles PATCH /api/daily-reports/topics/:id.
// Body: {"label": "...", "status": "active|archived"} — either field optional;
// omit a field to leave it unchanged. status is restricted to active/archived.
func updateTopic(c *gin.Context) {
	topicID, err := parseTopicID(c)
	if err != nil {
		return
	}
	var req struct {
		Label  *string `json:"label"`
		Status *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid body"})
		return
	}
	topic, err := repository.Repo.UpdateTopic(topicID, req.Label, req.Status)
	if err != nil {
		logging.Errorf("update topic %d: %v", topicID, err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": topic})
}

// mergeTopic handles POST /api/daily-reports/topics/:id/merge.
// Body: {"source_topic_ids": [2, 3]} — every section on those topics is
// reassigned to :id and the sources are archived.
func mergeTopic(c *gin.Context) {
	targetID, err := parseTopicID(c)
	if err != nil {
		return
	}
	var req struct {
		SourceTopicIDs []uint `json:"source_topic_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid body"})
		return
	}
	topic, err := repository.Repo.MergeTopics(targetID, req.SourceTopicIDs)
	if err != nil {
		logging.Errorf("merge into topic %d: %v", targetID, err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": topic})
}

// splitTopic handles POST /api/daily-reports/topics/:id/split.
// Body: {"section_ids": [10, 11], "label": "..."} — the listed sections are
// carved out of :id into a freshly created topic.
func splitTopic(c *gin.Context) {
	sourceID, err := parseTopicID(c)
	if err != nil {
		return
	}
	var req struct {
		SectionIDs []uint `json:"section_ids"`
		Label      string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid body"})
		return
	}
	topic, err := repository.Repo.SplitTopic(sourceID, req.SectionIDs, req.Label)
	if err != nil {
		logging.Errorf("split topic %d: %v", sourceID, err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": topic})
}

// broadcastProgress sends a WebSocket progress message.
func broadcastProgress(jobID string, status string, boardID uint, boardName string, saved int, progress string) {
	msg := buildProgressMessage(jobID, status, boardID, boardName, saved, progress)
	data, _ := json.Marshal(msg)
	ws.GetHub().BroadcastRaw(data)
}

func broadcastDone(jobID string, totalSaved int, totalBoards int) {
	msg := buildDoneMessage(jobID, totalSaved, totalBoards)
	data, _ := json.Marshal(msg)
	ws.GetHub().BroadcastRaw(data)
}

func buildProgressMessage(jobID string, status string, boardID uint, boardName string, saved int, progress string) map[string]interface{} {
	return map[string]interface{}{
		"type":       "daily_report_progress",
		"job_id":     jobID,
		"status":     status,
		"board_id":   boardID,
		"board_name": boardName,
		"saved":      saved,
		"progress":   progress,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
}

func buildDoneMessage(jobID string, totalSaved int, totalBoards int) map[string]interface{} {
	return map[string]interface{}{
		"type":         "daily_report_done",
		"job_id":       jobID,
		"total_saved":  totalSaved,
		"total_boards": totalBoards,
		"timestamp":    time.Now().Format(time.RFC3339),
	}
}

// triggerBackfillEmbeddings handles POST /api/daily-reports/backfill-embeddings
func triggerBackfillEmbeddings(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	go func() {
		embedded, matched, err := repository.Repo.BackfillSectionEmbeddings(ctx)
		if err != nil {
			logging.Errorf("daily-report: backfill failed: %v", err)
			return
		}
		logging.Infof("daily-report: backfill complete: %d sections embedded, %d sections matched", embedded, matched)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status": "processing",
		},
	})
}

// triggerBackfillRelations handles POST /api/daily-reports/backfill-relations
func triggerBackfillRelations(c *gin.Context) {
	boardIDStr := c.Query("board_id")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if boardIDStr != "" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board_id"})
			return
		}
		go func() {
			<-ctx.Done() // wait for response to be sent
			rebuilt, err := repository.Repo.BackfillRelations(uint(boardID))
			if err != nil {
				logging.Errorf("daily-report: backfill relations for board %d failed: %v", boardID, err)
				return
			}
			logging.Infof("daily-report: backfill relations for board %d complete: %d relations rebuilt", boardID, rebuilt)
		}()
	} else {
		go func() {
			<-ctx.Done()
			results, err := repository.Repo.BackfillAllRelations()
			if err != nil {
				logging.Errorf("daily-report: backfill all relations failed: %v", err)
				return
			}
			for bid, cnt := range results {
				logging.Infof("daily-report: board %d: %d relations rebuilt", bid, cnt)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "processing"}})
}

// triggerBackfillTopics handles POST /api/daily-reports/backfill-topics.
// Reconstructs persistent topics from historical sections lacking a topic
// assignment. Optional ?board_id scopes the run to one board; without it, all
// boards with unassigned sections are processed.
func triggerBackfillTopics(c *gin.Context) {
	boardIDStr := c.Query("board_id")
	_, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if boardIDStr != "" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board_id"})
			return
		}
		go func() {
			created, err := repository.Repo.BackfillPersistentTopics(uint(boardID))
			if err != nil {
				logging.Errorf("daily-report: backfill topics for board %d failed: %v", boardID, err)
				return
			}
			logging.Infof("daily-report: backfill topics for board %d complete: %d topics created", boardID, created)
		}()
	} else {
		go func() {
			results, err := repository.Repo.BackfillAllPersistentTopics()
			if err != nil {
				logging.Errorf("daily-report: backfill all topics failed: %v", err)
				return
			}
			for bid, cnt := range results {
				logging.Infof("daily-report: board %d: %d topics created", bid, cnt)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "processing"}})
}

func dailyReportBoardName(boardID uint) string {
	if boardID == 0 {
		return "All boards"
	}
	var board models.SemanticLabel
	if err := repository.Repo.DB().Select("label").Where("id = ?", boardID).First(&board).Error; err != nil {
		return fmt.Sprintf("Board #%d", boardID)
	}
	return board.Label
}

func timeoutCtx(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
