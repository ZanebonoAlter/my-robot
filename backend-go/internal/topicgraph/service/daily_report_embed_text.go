package service

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"syntopica-backend/internal/topicgraph/repository"
)

// Content-embedding text assembly (fix-section-embedding-content-based).
//
// Section embeddings SHALL represent the section's actual aggregated content
// (its tags' label/description/representative-article excerpts), NOT the
// cluster_label title text. The old title-based embedding froze L1/L2-hit
// sections onto the topic label (cluster_label = topic label for lane hits),
// which made topic_match_distance an ≈0 echo and pinned centroids to the
// first-day title forever — the "attraction black hole" loop.
//
// Tags (deterministic article facts) are chosen over LLM-generated thread
// text so the matching geometry stays anchored to fact inputs, consistent
// with the L2-prompt history-isolation principle, and stays same-modality
// with the tag semantic embeddings the lane router compares against.

const (
	// maxArticleContextRunes caps the ArticleContext excerpt per tag.
	maxArticleContextRunes = 100
	// maxSectionEmbedRunes caps the assembled section embedding text.
	// Calibrated against the embedding gateway's 512-token per-input limit
	// (observed 2026-08-22: a 545-token text was rejected and killed its whole
	// batch). 480 runes ≈ 330-480 tokens at typical CJK ratios, with headroom.
	maxSectionEmbedRunes = 480
)

// buildSectionEmbedText assembles the embedding input text for one section:
//
//	per tag: label + "：" + description + "；代表文章：" + excerpt(ArticleContext, 200 runes)
//	tag fragments joined with "\n", total truncated to 2000 runes
//
// Fallback chain (per spec): no tags → concatenated thread titles →
// cluster_label. Returns "" when nothing usable exists (caller skips embed).
func buildSectionEmbedText(clusterTags []repository.TagInput, threads []repository.DailyReportThread, fallbackLabel string) string {
	var sb strings.Builder
	for _, t := range clusterTags {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.TrimSpace(t.Label))
		if d := strings.TrimSpace(t.Description); d != "" {
			sb.WriteString("：")
			sb.WriteString(d)
		}
		if a := strings.TrimSpace(t.ArticleContext); a != "" {
			sb.WriteString("；代表文章：")
			if utf8.RuneCountInString(a) > maxArticleContextRunes {
				a = string([]rune(a)[:maxArticleContextRunes])
			}
			sb.WriteString(a)
		}
		if text := sb.String(); utf8.RuneCountInString(text) >= maxSectionEmbedRunes {
			break
		}
	}
	if text := sb.String(); strings.TrimSpace(text) != "" {
		return truncateRunesMax(text, maxSectionEmbedRunes)
	}

	// Fallback 1: thread titles.
	var titles []string
	for _, th := range threads {
		if title := strings.TrimSpace(th.Title); title != "" {
			titles = append(titles, title)
		}
	}
	if len(titles) > 0 {
		return truncateRunesMax(strings.Join(titles, "\n"), maxSectionEmbedRunes)
	}

	// Fallback 2: cluster label.
	return strings.TrimSpace(fallbackLabel)
}

// truncateRunesMax returns the first max runes of s (full s when shorter).
func truncateRunesMax(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// mustUnmarshalTagIDs decodes a section's ClusterTagIDs JSON column into a
// []uint. Malformed JSON (not producible through the pipeline) yields nil.
func mustUnmarshalTagIDs(raw repository.JSON) []uint {
	if len(raw) == 0 {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil
	}
	return ids
}
