package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/dataenrichment/repository"
)

// ── 版块级分析 API（design D8，tasks 4.1 / M6）───────────────────────────────
//
// POST /semantic-boards/:id/enrichment/analysis/trigger —— 手动触发版块分析
// GET  /semantic-boards/:id/enrichment/analysis/results      —— 历史列表
// GET  /semantic-boards/:id/enrichment/analysis/results/:rid —— 单份详情
//
// 单泳道 trigger 的可选 body {prefill_lens} 见 handler.go triggerEnrichment。

// triggerBoardEnrichment starts the board-level cycle-B flow in the
// background and returns immediately (fix-board-analysis-material 8.x):
// the run survives client disconnects; poll analysis-status for progress.
func (h *EnrichmentHandler) triggerBoardEnrichment(c *gin.Context) {
	boardID, ok := parseBoardID(c)
	if !ok {
		return
	}

	err := h.orchestrator.BoardEnrichmentEnabled(c.Request.Context(), boardID)
	if err != nil {
		// Board-not-enabled is a client error (M6.1); everything else 500.
		// Message keeps the machine-matchable English prefix (containsNotEnabled,
		// tests) and appends a human hint — the flag lives in the board edit dialog
		// / panel quick-toggle, which was undiscoverable before.
		if containsNotEnabled(err.Error()) {
			respondError(c, http.StatusBadRequest,
				"enrichment not enabled for this board：请先开启「数据增强」开关（工作台分析区一键开启，或板块编辑弹窗→分析配置）")
			return
		}
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	err = h.analysis.Start(AnalysisScopeBoard, boardID, analysisJobTimeout, func(ctx context.Context) (uint, error) {
		output, err := h.orchestrator.EnrichBoard(ctx, boardID)
		if err != nil {
			return 0, err
		}
		return output.Result.ID, nil
	})
	if err != nil {
		if errors.Is(err, ErrAlreadyRunning) {
			respondError(c, http.StatusConflict, "board analysis already running")
			return
		}
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, gin.H{"status": "started", "scope": AnalysisScopeBoard, "target_id": boardID})
}

// listBoardResults returns the board-scope analysis history (newest first).
func (h *EnrichmentHandler) listBoardResults(c *gin.Context) {
	boardID, ok := parseBoardID(c)
	if !ok {
		return
	}
	results, err := h.repo.ListBoardEnrichmentResults(c.Request.Context(), boardID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	list := make([]gin.H, 0, len(results))
	for i := range results {
		list = append(list, serializeBoardResult(&results[i]))
	}
	respondOK(c, list)
}

// getBoardResult returns one board-scope result by id (board-owned only).
func (h *EnrichmentHandler) getBoardResult(c *gin.Context) {
	boardID, ok := parseBoardID(c)
	if !ok {
		return
	}
	rid, err := strconv.ParseUint(c.Param("rid"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid result id")
		return
	}
	result, err := h.repo.GetTopicEnrichmentResultByID(c.Request.Context(), uint(rid))
	if err != nil {
		respondError(c, http.StatusNotFound, "result not found")
		return
	}
	if !repository.BoardIDMatches(result.SemanticBoardID, boardID) {
		respondError(c, http.StatusNotFound, "result not found")
		return
	}
	respondOK(c, serializeBoardResult(result))
}

// serializeBoardResult shapes one board-scope result for the API (mirrors the
// topic trigger's shape; sectors stays raw JSON for the frontend scope branch).
func serializeBoardResult(r *repository.TopicEnrichmentResult) gin.H {
	return gin.H{
		"id":               r.ID,
		"analysis_scope":   r.AnalysisScope,
		"result_kind":      repository.EffectiveResultKind(r),
		"parent_result_id": r.ParentResultID,
		"question_key":     r.QuestionKey,
		"sectors":          tryParseJSON(r.Sectors),
		"tool_calls":       tryParseJSON(r.ToolCalls),
		"input_snapshot":   tryParseJSON(r.InputSnapshot),
		"session_id":       r.SessionID,
		"created_at":       r.CreatedAt,
	}
}

func parseBoardID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(c, http.StatusBadRequest, "invalid board id")
		return 0, false
	}
	return uint(id), true
}

// getAnalysisStatus reports the current (or last finished) async analysis job
// for a target. GET /enrichment/analysis-status?scope=board|topic&id=<n>
func (h *EnrichmentHandler) getAnalysisStatus(c *gin.Context) {
	scope := c.Query("scope")
	if scope != AnalysisScopeBoard && scope != AnalysisScopeTopic {
		respondError(c, http.StatusBadRequest, "scope must be board or topic")
		return
	}
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	st, ok := h.analysis.Status(scope, uint(id))
	if !ok {
		// Never triggered (or process restarted): idle, nothing running.
		respondOK(c, gin.H{"scope": scope, "target_id": id, "running": false, "finished": false})
		return
	}
	respondOK(c, st)
}

// containsNotEnabled keeps the 4xx mapping local (no strings import at call site).
func containsNotEnabled(msg string) bool {
	return strings.Contains(msg, "not enabled")
}
