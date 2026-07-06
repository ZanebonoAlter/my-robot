package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
)

// LifelineService abstracts the methods of service.LifelineContextService used by handlers.
// This allows tests to mock the LLM-dependent regenerate operation.
type LifelineService interface {
	RefreshGranularity(ctx context.Context, topicID uint, granularity string, now time.Time) error
}

// Orchestrator abstracts the methods of service.OrchestratorService used by handlers.
// This allows tests to mock the LLM-dependent trigger operation.
type Orchestrator interface {
	EnrichTopic(ctx context.Context, topicID uint) (*service.EnrichmentOutput, error)
}

// EnrichmentHandler serves the data enrichment CRUD API.
// Dependencies are injected via InitHandler (called from runtime.go).
type EnrichmentHandler struct {
	repo              *repository.Repository
	lifelineSvc       LifelineService
	orchestrator      Orchestrator
	boardConfigReader service.BoardConfigReader
	db                *gorm.DB
}

var instance *EnrichmentHandler

// InitHandler stores the singleton handler. Call once at startup.
func InitHandler(
	repo *repository.Repository,
	lifelineSvc LifelineService,
	orchestrator Orchestrator,
	boardConfigReader service.BoardConfigReader,
	db *gorm.DB,
) {
	instance = &EnrichmentHandler{
		repo:              repo,
		lifelineSvc:       lifelineSvc,
		orchestrator:      orchestrator,
		boardConfigReader: boardConfigReader,
		db:                db,
	}
}

// NewHandler creates a standalone EnrichmentHandler for testing without touching the singleton.
func NewHandler(
	repo *repository.Repository,
	lifelineSvc LifelineService,
	orchestrator Orchestrator,
	boardConfigReader service.BoardConfigReader,
	db *gorm.DB,
) *EnrichmentHandler {
	return &EnrichmentHandler{
		repo:              repo,
		lifelineSvc:       lifelineSvc,
		orchestrator:      orchestrator,
		boardConfigReader: boardConfigReader,
		db:                db,
	}
}

// RegisterRoutes registers all data enrichment HTTP routes under rg using the
// package-level singleton handler.
func RegisterRoutes(rg *gin.RouterGroup) {
	if instance == nil {
		panic("handler.InitHandler must be called before RegisterRoutes")
	}
	instance.RegisterRoutes(rg)
}

// RegisterRoutes registers all routes on the given router group, accepting an
// explicit handler so tests can wire their own dependencies.
func (h *EnrichmentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// ── Topic enrichment (topic dimension) ──────────────────────────────────
	enrichment := rg.Group("/persistent-topics/:topicId/enrichment")

	// Table 1: topic_lifeline_context
	contexts := enrichment.Group("/contexts")
	{
		contexts.GET("", h.listContexts)
		contexts.GET("/:granularity", h.getContext)
		contexts.PUT("/:granularity", h.updateContext)
		contexts.POST("/:granularity/regenerate", h.regenerateContext)
	}

	// Table 2: topic_enrichment_result
	results := enrichment.Group("/results")
	{
		results.GET("", h.listResults)
		results.GET("/:id", h.getResult)
		results.POST("/trigger", h.triggerEnrichment)
	}

	// Table 3: topic_enrichment_review
	reviews := enrichment.Group("/reviews")
	{
		reviews.GET("", h.listReviews)
		reviews.POST("", h.createReview)
		reviews.PUT("/:id", h.updateReviewDeviation)
		reviews.POST("/:id/apply", h.applyReview)
	}

	// ── Board data source bindings (board dimension) ────────────────────────
	dataSources := rg.Group("/semantic-boards/:id/data-sources")
	{
		dataSources.GET("", h.listDataSources)
		dataSources.PUT("", h.upsertDataSource)
		dataSources.DELETE("/:sourceType", h.deleteDataSource)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

func parseTopicID(c *gin.Context) (uint, bool) {
	parsed, err := strconv.ParseUint(c.Param("topicId"), 10, 64)
	if err != nil || parsed == 0 {
		respondError(c, http.StatusBadRequest, "invalid topicId")
		return 0, false
	}
	return uint(parsed), true
}

func parseIDParam(c *gin.Context, name string) (uint, bool) {
	parsed, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || parsed == 0 {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return 0, false
	}
	return uint(parsed), true
}

// ── Table 1: topic_lifeline_context ─────────────────────────────────────────

// listContexts returns all granularity contexts for a topic.
// GET /persistent-topics/:topicId/enrichment/contexts
func (h *EnrichmentHandler) listContexts(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}

	list, err := h.repo.ListTopicLifelineContextsByTopic(c.Request.Context(), topicID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, list)
}

// getContext returns a single granularity context.
// GET /persistent-topics/:topicId/enrichment/contexts/:granularity
func (h *EnrichmentHandler) getContext(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}
	granularity := c.Param("granularity")
	if !isValidGranularity(granularity) {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid granularity: %s", granularity))
		return
	}

	lc, err := h.repo.GetTopicLifelineContext(c.Request.Context(), topicID, granularity)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	respondOK(c, lc)
}

// updateContext allows manual editing of a context's content.
// PUT /persistent-topics/:topicId/enrichment/contexts/:granularity
// Body: { "content": "..." }
func (h *EnrichmentHandler) updateContext(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}
	granularity := c.Param("granularity")
	if !isValidGranularity(granularity) {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid granularity: %s", granularity))
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "content is required")
		return
	}

	// Fetch existing context to get the ID.
	existing, err := h.repo.GetTopicLifelineContext(c.Request.Context(), topicID, granularity)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	existing.Content = req.Content
	existing.Source = "manual"

	if err := h.repo.UpsertTopicLifelineContext(c.Request.Context(), existing); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, existing)
}

// regenerateContext triggers manual re-generation of a granularity context.
// POST /persistent-topics/:topicId/enrichment/contexts/:granularity/regenerate
func (h *EnrichmentHandler) regenerateContext(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}
	granularity := c.Param("granularity")
	if !isValidGranularity(granularity) {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid granularity: %s", granularity))
		return
	}

	if err := h.lifelineSvc.RefreshGranularity(c.Request.Context(), topicID, granularity, time.Now()); err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("regenerate failed: %v", err))
		return
	}

	// Fetch the newly generated context.
	lc, err := h.repo.GetTopicLifelineContext(c.Request.Context(), topicID, granularity)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("context regenerated but failed to fetch: %v", err))
		return
	}

	respondOK(c, lc)
}

// ── Table 2: topic_enrichment_result ────────────────────────────────────────

// listResults returns all enrichment results for a topic, newest first.
// GET /persistent-topics/:topicId/enrichment/results
func (h *EnrichmentHandler) listResults(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}

	results, err := h.repo.ListTopicEnrichmentResultsByTopic(c.Request.Context(), topicID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Return slim summaries with sectors / tool_calls overview / input_snapshot / session_id.
	type resultSummary struct {
		ID                  uint   `json:"id"`
		EvolutionAssessment string `json:"evolution_assessment"`
		Sectors             any    `json:"sectors"`
		ToolCallsCount      int    `json:"tool_calls_count"`
		SessionID           string `json:"session_id"`
		CreatedAt           any    `json:"created_at"`
	}
	items := make([]resultSummary, 0, len(results))
	for _, r := range results {
		sectors := tryParseJSON(r.Sectors)
		tcCount := countJSONArray(r.ToolCalls)
		items = append(items, resultSummary{
			ID:                  r.ID,
			EvolutionAssessment: truncateStr(r.EvolutionAssessment, 200),
			Sectors:             sectors,
			ToolCallsCount:      tcCount,
			SessionID:           r.SessionID,
			CreatedAt:           r.CreatedAt,
		})
	}

	respondOK(c, items)
}

// getResult returns a single enrichment result with full detail.
// GET /persistent-topics/:topicId/enrichment/results/:id
func (h *EnrichmentHandler) getResult(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}

	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	result, err := h.repo.GetTopicEnrichmentResultByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	// Validate the result belongs to the requested topic (IDOR protection).
	if result.PersistentTopicID != topicID {
		respondError(c, http.StatusNotFound, "result not found for this topic")
		return
	}

	respondOK(c, gin.H{
		"id":                   result.ID,
		"persistent_topic_id":  result.PersistentTopicID,
		"evolution_assessment": result.EvolutionAssessment,
		"sectors":              tryParseJSON(result.Sectors),
		"causal_chain":         result.CausalChain,
		"tool_calls":           tryParseJSON(result.ToolCalls),
		"input_snapshot":       tryParseJSON(result.InputSnapshot),
		"session_id":           result.SessionID,
		"created_at":           result.CreatedAt,
	})
}

// triggerEnrichment manually triggers cycle-B enrichment for the topic.
// POST /persistent-topics/:topicId/enrichment/results/trigger
func (h *EnrichmentHandler) triggerEnrichment(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}

	// Check enrichment_enabled via board config.
	cfg, err := h.boardConfigReader.GetBoardConfig(c.Request.Context(), topicID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !cfg.EnrichmentEnabled {
		respondError(c, http.StatusBadRequest, "enrichment is not enabled for this topic's board")
		return
	}

	output, err := h.orchestrator.EnrichTopic(c.Request.Context(), topicID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("enrichment failed: %v", err))
		return
	}

	respondOK(c, gin.H{
		"result": gin.H{
			"id":                   output.Result.ID,
			"evolution_assessment": output.Result.EvolutionAssessment,
			"sectors":              tryParseJSON(output.Result.Sectors),
			"causal_chain":         output.Result.CausalChain,
			"tool_calls_count":     len(output.AgentLoops),
			"session_id":           output.Result.SessionID,
			"created_at":           output.Result.CreatedAt,
		},
		"review_generated": output.Review != nil,
	})
}

// ── Table 3: topic_enrichment_review ────────────────────────────────────────

// listReviews returns all reviews for a topic, newest first.
// GET /persistent-topics/:topicId/enrichment/reviews
func (h *EnrichmentHandler) listReviews(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}

	reviews, err := h.repo.ListTopicEnrichmentReviewsByTopic(c.Request.Context(), topicID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, reviews)
}

// updateReviewDeviation updates the deviation_summary of a review.
// PUT /persistent-topics/:topicId/enrichment/reviews/:id
// Body: { "deviation_summary": "..." }
func (h *EnrichmentHandler) updateReviewDeviation(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		DeviationSummary string `json:"deviation_summary" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "deviation_summary is required")
		return
	}

	if err := h.repo.UpdateTopicEnrichmentReviewDeviation(c.Request.Context(), id, req.DeviationSummary); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch the updated review.
	review, err := h.repo.GetTopicEnrichmentReviewByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, review)
}

// applyReview marks a review as applied (applied=true).
// Does NOT write back to table 1 (context) per design §4.3.
// POST /persistent-topics/:topicId/enrichment/reviews/:id/apply
func (h *EnrichmentHandler) applyReview(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.repo.ApplyTopicEnrichmentReview(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	review, err := h.repo.GetTopicEnrichmentReviewByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, review)
}

// createReview manually creates a review annotation.
// POST /persistent-topics/:topicId/enrichment/reviews
// Body: { "curr_result_id": N, "deviation_summary": "...", "prev_result_id": N? }
func (h *EnrichmentHandler) createReview(c *gin.Context) {
	topicID, ok := parseTopicID(c)
	if !ok {
		return
	}

	var req struct {
		CurrResultID     uint   `json:"curr_result_id" binding:"required"`
		DeviationSummary string `json:"deviation_summary" binding:"required"`
		PrevResultID     *uint  `json:"prev_result_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "curr_result_id and deviation_summary are required")
		return
	}

	review := &repository.TopicEnrichmentReview{
		PersistentTopicID: topicID,
		PrevResultID:      req.PrevResultID,
		CurrResultID:      req.CurrResultID,
		DeviationSummary:  req.DeviationSummary,
		Applied:           true,
		Source:            "manual",
	}

	if err := h.repo.CreateTopicEnrichmentReview(c.Request.Context(), review); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, review)
}

// ── Board data sources ──────────────────────────────────────────────────────

// listDataSources returns data sources bound to a board.
// GET /semantic-boards/:id/data-sources
func (h *EnrichmentHandler) listDataSources(c *gin.Context) {
	boardID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	list, err := h.repo.ListBoardDataSourcesByBoardID(c.Request.Context(), boardID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, list)
}

// upsertDataSource creates or updates a data source binding for a board.
// PUT /semantic-boards/:id/data-sources
// Body: { "source_type": "etf_quote", "config": {...}, "enabled": true }
func (h *EnrichmentHandler) upsertDataSource(c *gin.Context) {
	boardID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		SourceType string         `json:"source_type" binding:"required"`
		Config     map[string]any `json:"config"`
		Enabled    *bool          `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "source_type is required")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	ds := &repository.BoardDataSource{
		SemanticBoardID: boardID,
		SourceType:      req.SourceType,
		Config:          req.Config,
		Enabled:         enabled,
	}

	if err := h.repo.UpsertBoardDataSource(c.Request.Context(), ds); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondOK(c, ds)
}

// deleteDataSource removes a data source binding by source type.
// DELETE /semantic-boards/:id/data-sources/:sourceType
func (h *EnrichmentHandler) deleteDataSource(c *gin.Context) {
	boardID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	sourceType := c.Param("sourceType")

	ds, err := h.repo.GetBoardDataSourceByBoardAndType(c.Request.Context(), boardID, sourceType)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	if err := h.repo.DeleteBoardDataSource(c.Request.Context(), ds.ID); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, gin.H{"deleted": true})
}

// ── Private helpers ─────────────────────────────────────────────────────────

var validGranularities = map[string]bool{
	"week": true, "month": true, "year": true, "all": true,
}

func isValidGranularity(g string) bool {
	return validGranularities[g]
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func countJSONArray(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	count := 0
	for _, b := range raw {
		if b == '{' {
			count++
		}
	}
	return count
}

func tryParseJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}
