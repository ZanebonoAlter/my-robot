package board

import (
	"context"
	"encoding/json"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
)

// defaultSemanticBoardUpgradeLLM is the production airouter-backed implementation
// of SemanticBoardUpgradeLLM. It lives in the service package (not the HTTP
// handler layer) so that both the handler factory and the scheduler job can share
// a single default without a layering inversion (scheduler → handler).
type defaultSemanticBoardUpgradeLLM struct{}

// NewDefaultSemanticBoardUpgradeLLM returns the production airouter-backed
// SemanticBoardUpgradeLLM. Used by the handler's semanticBoardUpgradeLLMFactory
// var and by the board-upgrade-suggest scheduler job.
func NewDefaultSemanticBoardUpgradeLLM() SemanticBoardUpgradeLLM {
	return defaultSemanticBoardUpgradeLLM{}
}

// SuggestSemanticBoardUpgrades calls the airouter with the board-upgrade system
// prompt (mode-scoped) and parses the JSON suggestions list. A malformed JSON
// response is logged with a preview and returned as an error so the caller can
// skip the round (no partial/best-effort suggestions are synthesized).
func (defaultSemanticBoardUpgradeLLM) SuggestSemanticBoardUpgrades(ctx context.Context, prompt string, mode string) ([]SemanticBoardUpgradeSuggestion, error) {
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Operation:  "tagmanagement.board_upgrade_suggest",
		Capability: airouter.CapabilityTopicTagging,
		Messages: []airouter.Message{
			{Role: "system", Content: BuildSemanticBoardUpgradeSystemPrompt(mode)},
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
			Decision          SemanticBoardUpgradeDecision `json:"decision"`
			BoardLabel        string                       `json:"board_label"`
			Description       string                       `json:"description"`
			AuxiliaryLabelIDs []uint                       `json:"auxiliary_label_ids"`
			Reason            string                       `json:"reason"`
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
	suggestions := make([]SemanticBoardUpgradeSuggestion, 0, len(parsed.Suggestions))
	for _, raw := range parsed.Suggestions {
		suggestions = append(suggestions, SemanticBoardUpgradeSuggestion{Decision: raw.Decision, BoardLabel: raw.BoardLabel, Description: raw.Description, AuxiliaryLabelIDs: raw.AuxiliaryLabelIDs, Reason: raw.Reason})
	}
	return suggestions, nil
}
