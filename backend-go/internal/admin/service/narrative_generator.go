package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
)

type NarrativeOutput struct {
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	Status          string  `json:"status"`
	RelatedTagIDs   []uint  `json:"related_tag_ids"`
	ParentIDs       []uint  `json:"parent_ids"`
	ConfidenceScore float64 `json:"confidence_score"`
}

func parseNarrativeResponse(content string) ([]NarrativeOutput, error) {
	content = jsonutil.SanitizeLLMJSON(content)

	var raw struct {
		Narratives []NarrativeOutput `json:"narratives"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err == nil {
		return raw.Narratives, nil
	}

	var direct []NarrativeOutput
	if err := json.Unmarshal([]byte(content), &direct); err != nil {
		return nil, fmt.Errorf("failed to parse narrative JSON: %w", err)
	}
	return direct, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var validNarrativeStatuses = map[string]bool{
	"emerging": true, "continuing": true, "splitting": true, "merging": true, "ending": true,
}

func validateNarrativeOutputs(outputs []NarrativeOutput, tagInputs []TagInput, prevNarratives []PreviousNarrative) []NarrativeOutput {
	validTagIDs := make(map[uint]bool, len(tagInputs))
	for _, t := range tagInputs {
		validTagIDs[t.ID] = true
	}

	validParentIDs := make(map[uint64]bool, len(prevNarratives))
	for _, p := range prevNarratives {
		validParentIDs[p.ID] = true
	}

	var valid []NarrativeOutput
	for _, out := range outputs {
		if strings.TrimSpace(out.Title) == "" || strings.TrimSpace(out.Summary) == "" {
			logging.Warnf("narrative: skipping output with empty title or summary")
			continue
		}

		if !validNarrativeStatuses[out.Status] {
			logging.Warnf("narrative: fixing invalid status '%s' to 'emerging' for '%s'", out.Status, out.Title)
			out.Status = "emerging"
		}

		if len(prevNarratives) == 0 && len(out.ParentIDs) > 0 {
			logging.Warnf("narrative: clearing parent_ids for '%s' — no previous narratives exist", out.Title)
			out.ParentIDs = nil
		}

		filteredTagIDs := filterValidIDs(out.RelatedTagIDs, validTagIDs, "related_tag_id", out.Title)
		if len(filteredTagIDs) == 0 {
			logging.Warnf("narrative: skipping '%s' — no valid related_tag_ids after filtering", out.Title)
			continue
		}
		out.RelatedTagIDs = filteredTagIDs

		if len(out.ParentIDs) > 0 {
			out.ParentIDs = filterValidParentIDs(out.ParentIDs, validParentIDs, out.Title)
		}

		if out.ParentIDs == nil {
			out.ParentIDs = []uint{}
		}
		if out.RelatedTagIDs == nil {
			out.RelatedTagIDs = []uint{}
		}

		valid = append(valid, out)
	}
	return valid
}

func filterValidIDs(ids []uint, validSet map[uint]bool, label, title string) []uint {
	var filtered []uint
	for _, id := range ids {
		if validSet[id] {
			filtered = append(filtered, id)
		} else {
			logging.Warnf("narrative: dropping invalid %s %d in '%s'", label, id, title)
		}
	}
	return filtered
}

func filterValidParentIDs(ids []uint, validSet map[uint64]bool, title string) []uint {
	var filtered []uint
	for _, id := range ids {
		if validSet[uint64(id)] {
			filtered = append(filtered, id)
		} else {
			logging.Warnf("narrative: dropping invalid parent_id %d in '%s'", id, title)
		}
	}
	return filtered
}
