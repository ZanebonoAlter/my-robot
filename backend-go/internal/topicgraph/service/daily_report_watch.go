package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm/clause"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/topicgraph/repository"
)

// GenerateAndSaveReport is the unified entry point for daily report generation.
// It replaces the older pattern: GenerateDailyReport → SaveReport.
// After saving the report (and its sections), it runs EvaluateWatchHits
// OUTSIDE the SaveReport transaction as a read-only overlay.
func GenerateAndSaveReport(ctx context.Context, boardID uint, date time.Time) (*repository.BoardDailyReport, error) {
	report, sections, threadBatches, err := GenerateDailyReport(ctx, boardID, date)
	if err != nil {
		return nil, fmt.Errorf("generate daily report: %w", err)
	}
	if report == nil {
		return nil, nil
	}

	if err := repository.Repo.SaveReport(report, sections, threadBatches); err != nil {
		return nil, fmt.Errorf("save daily report: %w", err)
	}

	// EvaluateWatchHits runs AFTER SaveReport (outside its transaction).
	// Failure is swallowed — hits are a read-only overlay that SHALL NOT
	// block daily report generation.
	if watchErr := EvaluateWatchHits(ctx, boardID, report, sections); watchErr != nil {
		logging.Warnf("daily-report: watch hit evaluation failed for board %d report %d: %v",
			boardID, report.ID, watchErr)
	}

	return report, nil
}

// watchChatFunc mirrors airouter.Router.Chat so tests can inject a mock
// instead of hitting the real AI provider pipeline.
type watchChatFunc func(ctx context.Context, req airouter.ChatRequest) (*airouter.ChatResult, error)

// EvaluateWatchHits evaluates every active watch against all sections of a
// newly-saved daily report via a single batch AI call. Detected matches are
// written as TopicWatchHit rows. This function SHALL NOT change any section's
// persistent_topic_id or any topic's consecutive_hits. On failure it returns
// an error that the caller SHOULD swallow (log.Warnf + continue).
func EvaluateWatchHits(ctx context.Context, boardID uint, report *repository.BoardDailyReport, sections []repository.DailyReportSection) error {
	return evaluateWatchHitsWithChat(ctx, boardID, report, sections, airouter.NewRouter().Chat)
}

// evaluateWatchHitsWithChat is the testable core of EvaluateWatchHits.
// It accepts a chat function so tests can inject a mock instead of hitting
// the real AI provider.
func evaluateWatchHitsWithChat(
	ctx context.Context,
	boardID uint,
	report *repository.BoardDailyReport,
	sections []repository.DailyReportSection,
	chat watchChatFunc,
) error {
	watches, err := repository.Repo.ListActiveWatchesByBoard(boardID)
	if err != nil {
		return fmt.Errorf("list active watches: %w", err)
	}
	if len(watches) == 0 {
		return nil // nothing to evaluate
	}
	if len(sections) == 0 {
		return nil
	}

	// Build a set of valid IDs to filter AI hallucinations.
	validWatchIDs := make(map[uint]bool, len(watches))
	for _, w := range watches {
		validWatchIDs[w.ID] = true
	}
	validSectionIDs := make(map[uint]bool, len(sections))
	for _, s := range sections {
		validSectionIDs[s.ID] = true
	}

	prompt := buildWatchHitPrompt(watches, sections)

	temperature := 0.1
	maxTokens := 4096
	result, err := chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityDigestPolish,
		Messages: []airouter.Message{
			{Role: "system", Content: watchHitSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		JSONMode:    true,
		JSONSchema: &airouter.JSONSchema{
			Type: "object",
			Properties: map[string]airouter.SchemaProperty{
				"hits": {
					Type: "array",
					Items: &airouter.SchemaProperty{
						Type: "object",
						Properties: map[string]airouter.SchemaProperty{
							"watch_id":   {Type: "integer", Description: "关注标记 ID"},
							"section_id": {Type: "integer", Description: "日报节 ID"},
							"reason":     {Type: "string", Description: "一句话命中理由"},
						},
						Required: []string{"watch_id", "section_id", "reason"},
					},
				},
			},
			Required: []string{"hits"},
		},
		Metadata: map[string]any{
			"operation":     "daily_report_watch_hit",
			"watch_count":   len(watches),
			"section_count": len(sections),
		},
	})
	if err != nil {
		return fmt.Errorf("AI watch hit call failed: %w", err)
	}

	logging.Infof("daily-report: watch hit LLM response length=%d for board %d report %d",
		len(result.Content), boardID, report.ID)

	hits, err := parseWatchHitResponse(result.Content, validWatchIDs, validSectionIDs, report)
	if err != nil {
		return fmt.Errorf("parse watch hit response: %w", err)
	}

	if len(hits) == 0 {
		return nil
	}

	// Batch upsert hits — silently skip duplicates on (watch_id, section_id, report_id)
	// so the daily report is never blocked by a duplicate AI response.
	if err := repository.Repo.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "watch_id"}, {Name: "section_id"}, {Name: "report_id"}},
		DoNothing: true,
	}).Create(&hits).Error; err != nil {
		return fmt.Errorf("write watch hits: %w", err)
	}

	logging.Infof("daily-report: wrote %d watch hits for board %d report %d",
		len(hits), boardID, report.ID)
	return nil
}

func watchHitSystemPrompt() string {
	return `你是一名新闻分析助手。你的任务是：给定若干个"关注标记"和一期日报的所有"节(section)"，判断哪些节与哪些关注标记内容相关。

规则：
1. 每个关注标记代表用户关心的一个话题方向
2. 对每个关注标记，判断哪些节的内容与之相关（节可能匹配多个关注）
3. 只记录确实相关的命中，不要强凑
4. 每个命中需要给出一句话的理由，说明为什么这个节与这个关注相关`
}

func buildWatchHitPrompt(watches []repository.BoardTopicWatch, sections []repository.DailyReportSection) string {
	var sb strings.Builder
	sb.WriteString("## 关注标记\n\n")
	for _, w := range watches {
		fmt.Fprintf(&sb, "- [id:%d] %s\n", w.ID, w.Label)
	}
	sb.WriteString("\n## 日报节列表\n\n")
	for _, s := range sections {
		fmt.Fprintf(&sb, "- [section_id:%d] %s\n", s.ID, s.ClusterLabel)
	}
	sb.WriteString("\n请输出命中列表，格式：{\"hits\":[{\"watch_id\":1,\"section_id\":101,\"reason\":\"原因\"},...]}\n")
	return sb.String()
}

type rawWatchHit struct {
	WatchID   uint   `json:"watch_id"`
	SectionID uint   `json:"section_id"`
	Reason    string `json:"reason"`
}

type rawWatchHitResponse struct {
	Hits []rawWatchHit `json:"hits"`
}

func parseWatchHitResponse(content string, validWatchIDs, validSectionIDs map[uint]bool, report *repository.BoardDailyReport) ([]repository.TopicWatchHit, error) {
	content = jsonutil.SanitizeLLMJSON(content)

	var raw rawWatchHitResponse
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse watch hit JSON: %w", err)
	}

	var result []repository.TopicWatchHit
	for _, h := range raw.Hits {
		if !validWatchIDs[h.WatchID] || !validSectionIDs[h.SectionID] {
			continue // filter AI hallucinations
		}
		result = append(result, repository.TopicWatchHit{
			WatchID:    h.WatchID,
			SectionID:  h.SectionID,
			ReportID:   report.ID,
			PeriodDate: report.PeriodDate,
			Reason:     strings.TrimSpace(h.Reason),
		})
	}
	return result, nil
}
