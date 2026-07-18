package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/tagmanagement/service"
)

type confirmSemanticBoardUpgradeHTTPRequest struct {
	Decision          service.SemanticBoardUpgradeDecision `json:"decision"`
	BoardLabel        string                               `json:"board_label"`
	Description       string                               `json:"description"`
	AuxiliaryLabelIDs []uint                               `json:"auxiliary_label_ids"`
	TargetBoardID     *uint                                `json:"target_board_id"`
	SuggestionID      *uint                                `json:"suggestion_id,omitempty"`
}
type semanticBoardUpgradeSuggestionDTO struct {
	Decision          service.SemanticBoardUpgradeDecision `json:"decision"`
	BoardLabel        string                               `json:"board_label"`
	Description       string                               `json:"description"`
	AuxiliaryLabelIDs []uint                               `json:"auxiliary_label_ids"`
	AuxiliaryLabels   []struct {
		ID    uint   `json:"id"`
		Label string `json:"label"`
	} `json:"auxiliary_labels"`
	TargetBoardID    *uint              `json:"target_board_id,omitempty"`
	TargetBoardLabel string             `json:"target_board_label,omitempty"`
	Reason           string             `json:"reason"`
	BoardAffinities  []boardAffinityDTO `json:"board_affinities"`
}
type semanticBoardUpgradeCandidateDTO struct {
	ID       uint   `json:"id"`
	Label    string `json:"label"`
	Slug     string `json:"slug"`
	RefCount int    `json:"ref_count"`
}
type boardAffinityDTO struct {
	BoardID            uint    `json:"board_id"`
	BoardLabel         string  `json:"board_label"`
	BoardDescription   string  `json:"board_description"`
	MatchingCandidates int     `json:"matching_candidates"`
	AvgDistance        float64 `json:"avg_distance"`
}
type semanticBoardUpgradeClusterDTO struct {
	Candidates      []semanticBoardUpgradeCandidateDTO `json:"candidates"`
	BoardAffinities []boardAffinityDTO                 `json:"board_affinities"`
}
type airouterSemanticBoardUpgradeLLM struct{}

func (h *semanticBoardHandler) getUpgradeCandidates(c *gin.Context) {
	mode := strings.TrimSpace(c.Query("mode"))
	svc := service.NewSemanticBoardUpgradeService(h.db, nil, nil)
	config := svc.LoadUpgradeConfig(c.Request.Context())
	if mode != "" {
		config.Mode = mode
	}
	if config.Mode == "" {
		config.Mode = "discover_new"
	}
	candidates, err := svc.CollectCandidates(c.Request.Context(), config)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	clusters, err := svc.ClusterCandidates(c.Request.Context(), candidates, config)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	respondOK(c, gin.H{"candidates": upgradeCandidatesToDTO(candidates), "clusters": upgradeClustersToDTO(clusters), "config": semanticBoardUpgradeConfigToMap(config)})
}
func (h *semanticBoardHandler) suggestUpgrades(c *gin.Context) {
	mode := strings.TrimSpace(c.Query("mode"))
	svc := service.NewSemanticBoardUpgradeService(h.db, semanticBoardUpgradeLLMFactory(), nil)
	suggestions, clusters, err := svc.GenerateSuggestions(c.Request.Context(), mode)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	respondOK(c, gin.H{"suggestions": h.suggestionsToDTO(c.Request.Context(), suggestions, clusters)})
}
func (h *semanticBoardHandler) executeUpgrade(c *gin.Context) {
	var req confirmSemanticBoardUpgradeHTTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	result, err := service.NewSemanticBoardUpgradeService(h.db, nil, semanticBoardLabelEmbedder).ConfirmSuggestion(c.Request.Context(), service.ConfirmSemanticBoardUpgradeRequest(req))
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	respondOK(c, gin.H{"semantic_board_id": result.SemanticBoardID, "auxiliary_label_ids": result.AuxiliaryLabelIDs})
}
func (h *semanticBoardHandler) enqueueBackfill(c *gin.Context) {
	var req service.SemanticBoardBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	job, err := h.backfill.Enqueue(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	respondOK(c, job)
}
func (h *semanticBoardHandler) getBackfillJob(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("id"))
	job, ok := h.backfill.GetJob(jobID)
	if !ok {
		respondError(c, http.StatusNotFound, fmt.Errorf("backfill job not found"))
		return
	}
	respondOK(c, job)
}
func (h *semanticBoardHandler) backfillBoardEmbeddings(c *gin.Context) {
	ctx := c.Request.Context()
	var boards []models.SemanticLabel
	if err := h.db.WithContext(ctx).
		Where("label_type = ? AND embedding IS NULL", "board").
		Find(&boards).Error; err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	count := 0
	for _, board := range boards {
		input := semanticBoardEmbeddingInput(board.Label, board.Description)
		pgVector, _, err := semanticBoardLabelEmbedder(ctx, input, service.AuxiliaryLabelEmbeddingModeStorage)
		if err != nil {
			logging.Warnf("[backfill-embeddings] failed for board %d (%s): %v", board.ID, board.Label, err)
			continue
		}
		if err := h.db.WithContext(ctx).Model(&models.SemanticLabel{}).Where("id = ?", board.ID).Update("embedding", pgVector).Error; err != nil {
			logging.Warnf("[backfill-embeddings] db update failed for board %d: %v", board.ID, err)
			continue
		}
		count++
	}
	respondOK(c, gin.H{"backfilled": count, "total": len(boards)})
}
func (airouterSemanticBoardUpgradeLLM) SuggestSemanticBoardUpgrades(ctx context.Context, prompt string, mode string) ([]service.SemanticBoardUpgradeSuggestion, error) {
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Operation:  "tagmanagement.board_upgrade_suggest",
		Capability: airouter.CapabilityTopicTagging,
		Messages: []airouter.Message{
			{Role: "system", Content: service.BuildSemanticBoardUpgradeSystemPrompt(mode)},
			{Role: "user", Content: prompt},
		},
		JSONMode: true,
		Metadata: map[string]any{"operation": "semantic_board_upgrade_suggest"},
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Suggestions []struct {
			Decision          service.SemanticBoardUpgradeDecision `json:"decision"`
			BoardLabel        string                               `json:"board_label"`
			Description       string                               `json:"description"`
			AuxiliaryLabelIDs []uint                               `json:"auxiliary_label_ids"`
			Reason            string                               `json:"reason"`
		} `json:"suggestions"`
	}
	sanitized := jsonutil.SanitizeLLMJSON(result.Content)
	if err := json.Unmarshal([]byte(sanitized), &parsed); err != nil {
		rawPreview := sanitized
		if len(rawPreview) > 500 {
			rawPreview = rawPreview[:500] + "..."
		}
		logging.Warnf("[semantic-board-upgrade] LLM JSON parse failed: %v, raw=%d sanitized=%d preview=%s", err, len(result.Content), len(sanitized), rawPreview)
		return nil, err
	}
	suggestions := make([]service.SemanticBoardUpgradeSuggestion, 0, len(parsed.Suggestions))
	for _, raw := range parsed.Suggestions {
		suggestions = append(suggestions, service.SemanticBoardUpgradeSuggestion{Decision: raw.Decision, BoardLabel: raw.BoardLabel, Description: raw.Description, AuxiliaryLabelIDs: raw.AuxiliaryLabelIDs, Reason: raw.Reason})
	}
	return suggestions, nil
}
func newSemanticBoardUpgradeLLM() service.SemanticBoardUpgradeLLM {
	return airouterSemanticBoardUpgradeLLM{}
}
func upgradeCandidatesToDTO(candidates []service.SemanticBoardUpgradeCandidate) []semanticBoardUpgradeCandidateDTO {
	items := make([]semanticBoardUpgradeCandidateDTO, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, semanticBoardUpgradeCandidateDTO{ID: candidate.ID, Label: candidate.Label, Slug: candidate.Slug, RefCount: candidate.RefCount})
	}
	return items
}
func upgradeClustersToDTO(clusters []service.SemanticBoardUpgradeCluster) []semanticBoardUpgradeClusterDTO {
	items := make([]semanticBoardUpgradeClusterDTO, 0, len(clusters))
	for _, cluster := range clusters {
		affDTOs := make([]boardAffinityDTO, 0, len(cluster.BoardAffinities))
		for _, aff := range cluster.BoardAffinities {
			affDTOs = append(affDTOs, boardAffinityDTO(aff))
		}
		items = append(items, semanticBoardUpgradeClusterDTO{
			Candidates:      upgradeCandidatesToDTO(cluster.Candidates),
			BoardAffinities: affDTOs,
		})
	}
	return items
}
func (h *semanticBoardHandler) suggestionsToDTO(ctx context.Context, suggestions []service.SemanticBoardUpgradeSuggestion, clusters []service.SemanticBoardUpgradeCluster) []semanticBoardUpgradeSuggestionDTO {
	// Collect unique IDs for batch lookup
	labelIDSet := make(map[uint]struct{})
	boardIDSet := make(map[uint]struct{})
	for _, s := range suggestions {
		for _, id := range s.AuxiliaryLabelIDs {
			labelIDSet[id] = struct{}{}
		}
		if s.TargetBoardID != nil {
			boardIDSet[*s.TargetBoardID] = struct{}{}
		}
	}

	// Batch lookup auxiliary labels
	labelNames := make(map[uint]string)
	if len(labelIDSet) > 0 {
		ids := make([]uint, 0, len(labelIDSet))
		for id := range labelIDSet {
			ids = append(ids, id)
		}
		var labels []models.SemanticLabel
		if err := h.db.WithContext(ctx).Where("id IN ?", ids).Select("id, label").Find(&labels).Error; err == nil {
			for _, l := range labels {
				labelNames[l.ID] = l.Label
			}
		}
	}

	// Batch lookup board labels
	boardNames := make(map[uint]string)
	if len(boardIDSet) > 0 {
		ids := make([]uint, 0, len(boardIDSet))
		for id := range boardIDSet {
			ids = append(ids, id)
		}
		var labels []models.SemanticLabel
		if err := h.db.WithContext(ctx).Where("id IN ? AND label_type = ?", ids, "board").Select("id, label").Find(&labels).Error; err == nil {
			for _, l := range labels {
				boardNames[l.ID] = l.Label
			}
		}
	}

	// Build candidate ID → cluster index map for board_affinities lookup
	candidateToCluster := make(map[uint]int)
	for i, cluster := range clusters {
		for _, c := range cluster.Candidates {
			candidateToCluster[c.ID] = i
		}
	}

	items := make([]semanticBoardUpgradeSuggestionDTO, 0, len(suggestions))
	for _, s := range suggestions {
		dto := semanticBoardUpgradeSuggestionDTO{
			Decision:          s.Decision,
			BoardLabel:        s.BoardLabel,
			Description:       s.Description,
			AuxiliaryLabelIDs: s.AuxiliaryLabelIDs,
			TargetBoardID:     s.TargetBoardID,
			Reason:            s.Reason,
		}
		for _, id := range s.AuxiliaryLabelIDs {
			dto.AuxiliaryLabels = append(dto.AuxiliaryLabels, struct {
				ID    uint   `json:"id"`
				Label string `json:"label"`
			}{ID: id, Label: labelNames[id]})
		}
		if s.TargetBoardID != nil {
			if name, ok := boardNames[*s.TargetBoardID]; ok {
				dto.TargetBoardLabel = name
			}
		}
		// Embed board_affinities from matching cluster
		if len(s.AuxiliaryLabelIDs) > 0 {
			if clusterIdx, ok := candidateToCluster[s.AuxiliaryLabelIDs[0]]; ok {
				bas := clusters[clusterIdx].BoardAffinities
				dto.BoardAffinities = make([]boardAffinityDTO, 0, len(bas))
				for _, ba := range bas {
					dto.BoardAffinities = append(dto.BoardAffinities, boardAffinityDTO(ba))
				}
			}
		}
		items = append(items, dto)
	}
	return items
}
func semanticBoardUpgradeConfigToMap(config service.SemanticBoardUpgradeConfig) gin.H {
	return gin.H{
		"semantic_board_upgrade_ref_count_threshold":        config.RefCountThreshold,
		"semantic_board_upgrade_cluster_distance_threshold": config.ClusterDistanceThreshold,
		"semantic_board_upgrade_cotag_window_days":          config.CoTagWindowDays,
		"semantic_board_upgrade_cotag_top_n":                config.CoTagTopN,
		"semantic_board_upgrade_cotag_dedupe_sim_threshold": config.CoTagDedupeSimThreshold,
		"semantic_board_upgrade_cotag_hard_limit":           config.CoTagHardLimit,
		"semantic_board_upgrade_cluster_method":             config.ClusterMethod,
	}
}
