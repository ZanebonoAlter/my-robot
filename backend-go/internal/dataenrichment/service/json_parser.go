package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseJSONResponse extracts a JSON object from LLM output text.
// Ported from PoC tests/data_enrichment_poc/llm_client.py:parse_json_response.
//
// Handles three cases:
//  1. Clean JSON — direct unmarshal.
//  2. Markdown code block wrapped — strips ```json / ``` fences.
//  3. Text with embedded JSON — finds first { and last }, extracts the substring.
//
// Returns nil, error when no valid JSON object can be found.
func ParseJSONResponse(text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("parse json response: empty text")
	}

	// Strip markdown code blocks.
	if strings.HasPrefix(text, "```") {
		// Remove opening fence line (e.g. ```json or just ```).
		if idx := strings.Index(text, "\n"); idx >= 0 {
			text = text[idx+1:]
		} else {
			text = text[3:]
		}
		// Remove closing fence.
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	// Try direct unmarshal.
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err == nil {
		return result, nil
	}

	// Find first { and last }, extract substring.
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		sub := text[start : end+1]
		if err := json.Unmarshal([]byte(sub), &result); err == nil {
			return result, nil
		}
	}

	preview := text
	if len(preview) > 200 {
		preview = preview[:200]
	}
	return nil, fmt.Errorf("parse json response: unable to extract valid JSON from: %s", preview)
}
