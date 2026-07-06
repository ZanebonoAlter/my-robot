package service

import (
	"fmt"
	"sort"
	"time"
)

// ── Types (local interfaces for decoupling from topicgraph package) ────────

// LifelineReader abstracts the data source for topic lifeline data.
// Production implementation will delegate to topicgraph repository.
type LifelineReader interface {
	GetTopicLifeline(topicID uint) (SectionTimelineData, error)
}

// ThreadTitleReader abstracts loading thread titles for a section.
// When not available (nil), thread titles from the node are used directly.
// Production implementation will query daily_report_threads.
type ThreadTitleReader interface {
	GetThreadTitles(sectionID uint) ([]string, error)
}

// SectionTimelineData mirrors topicgraph's SectionTimelineResponse for decoupling.
type SectionTimelineData struct {
	Topic    TopicBrief            `json:"topic"`
	Sections []TimelineSectionNode `json:"sections"`
	// Relations are read but not rendered into agent text; kept for future use.
}

// TopicBrief carries the topic metadata needed by the renderer.
type TopicBrief struct {
	ID              uint      `json:"id"`
	Label           string    `json:"label"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	Source          string    `json:"source"`
	FirstSeenDate   time.Time `json:"first_seen_date"`
	LastSeenDate    time.Time `json:"last_seen_date"`
	HitCount        int       `json:"hit_count"`
	ConsecutiveHits int       `json:"consecutive_hits"`
}

// TimelineSectionNode carries one day's section data for the renderer.
type TimelineSectionNode struct {
	SectionID            uint      `json:"section_id"`
	PeriodDate           time.Time `json:"period_date"`
	ClusterLabel         string    `json:"cluster_label"`
	Status               string    `json:"status"`
	TopicMatchConfidence string    `json:"topic_match_confidence"`
	ArticleCount         int       `json:"article_count"`
	ThreadCount          int       `json:"thread_count"`
	ThreadTitles         []string  `json:"thread_titles"`
}

// ── LifelineRenderer ────────────────────────────────────────────────────────

// LifelineRenderer renders a topic lifeline as agent-readable text.
type LifelineRenderer struct {
	threadReader ThreadTitleReader
}

// NewLifelineRenderer creates a new renderer. threadReader may be nil.
func NewLifelineRenderer(threadReader ...ThreadTitleReader) *LifelineRenderer {
	var tr ThreadTitleReader
	if len(threadReader) > 0 {
		tr = threadReader[0]
	}
	return &LifelineRenderer{threadReader: tr}
}

// statusChinese maps English status values to Chinese display text.
var statusChinese = map[string]string{
	"emerging":   "涌现",
	"continuing": "延续",
	"split":      "分叉",
	"merge":      "合并",
	"ending":     "结束",
}

// confidenceChinese maps confidence values to Chinese display text.
var confidenceChinese = map[string]string{
	"anchor_hit": "稳接",
	"auto_new":   "新开",
	"unmatched":  "断链",
	"manual":     "人工接",
}

// RenderLifelineForAgent renders a topic lifeline as markdown text for agent consumption.
// Format strictly matches the PoC render_lifeline_for_agent output:
//
//	# 持久话题演进脉络
//	## 话题本体
//	## 逐日演进(按时间正序)
func (r *LifelineRenderer) RenderLifelineForAgent(reader LifelineReader, topicID uint, windowDays int) (string, error) {
	data, err := reader.GetTopicLifeline(topicID)
	if err != nil {
		return "", fmt.Errorf("render lifeline: %w", err)
	}

	t := data.Topic
	lines := []string{
		"# 持久话题演进脉络",
		"",
		"## 话题本体",
		fmt.Sprintf("- 名称: %s", t.Label),
	}
	if t.Description != "" {
		lines = append(lines, fmt.Sprintf("- 演进概述: %s", t.Description))
	}
	lines = append(lines, fmt.Sprintf("- 状态: %s | 首次出现: %s | 最近: %s | 累计命中: %d天 | 连续: %d天",
		t.Status, formatDate(t.FirstSeenDate), formatDate(t.LastSeenDate), t.HitCount, t.ConsecutiveHits))
	lines = append(lines, "", "## 逐日演进(按时间正序)")

	// Sort sections by period_date ASC.
	sort.Slice(data.Sections, func(i, j int) bool {
		return data.Sections[i].PeriodDate.Before(data.Sections[j].PeriodDate)
	})

	for _, s := range data.Sections {
		statusCN := statusChinese[s.Status]
		if statusCN == "" {
			statusCN = s.Status
		}
		confCN := confidenceChinese[s.TopicMatchConfidence]
		if confCN == "" {
			confCN = s.TopicMatchConfidence
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("### %s [%s/%s] %s",
			formatDate(s.PeriodDate), statusCN, confCN, s.ClusterLabel))
		lines = append(lines, fmt.Sprintf("  文章数: %d | 线索数: %d", s.ArticleCount, s.ThreadCount))

		titles := s.ThreadTitles
		if r.threadReader != nil {
			loaded, err := r.threadReader.GetThreadTitles(s.SectionID)
			if err == nil && len(loaded) > 0 {
				titles = loaded
			}
		}
		for _, title := range titles {
			lines = append(lines, fmt.Sprintf("  - %s", title))
		}
	}

	return joinLines(lines), nil
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
