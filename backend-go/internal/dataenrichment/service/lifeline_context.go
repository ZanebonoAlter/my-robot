package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/airouter"
)

// LifelineContextService manages topic lifeline context generation for cycle A.
// It produces week/month/year/all granularity news summaries via LLM.
type LifelineContextService struct {
	airouter      AirRouter
	repo          *repository.Repository
	sectionReader SectionReader
	capability    airouter.Capability
}

// NewLifelineContextService creates a new LifelineContextService.
func NewLifelineContextService(
	airouter AirRouter,
	repo *repository.Repository,
	sectionReader SectionReader,
	capability airouter.Capability,
) *LifelineContextService {
	return &LifelineContextService{
		airouter:      airouter,
		repo:          repo,
		sectionReader: sectionReader,
		capability:    capability,
	}
}

// ── Public methods ──────────────────────────────────────────────────────────

// RefreshWeek directly summarizes the current week's sections without relying on old context.
// as_of_date is set to the exclusive boundary (next Monday) of the current week.
func (s *LifelineContextService) RefreshWeek(ctx context.Context, topicID uint, now time.Time) error {
	gran := string(repository.GranularityWeek)
	from, to := weekRange(now)
	sectionsText, err := s.sectionReader.ReadSections(ctx, topicID, from, to)
	if err != nil {
		return fmt.Errorf("refresh week: read sections: %w", err)
	}
	content, err := s.summarizeWeek(ctx, topicID, sectionsText)
	if err != nil {
		return fmt.Errorf("refresh week: summarize: %w", err)
	}
	return s.repo.UpsertTopicLifelineContext(ctx, &repository.TopicLifelineContext{
		PersistentTopicID: topicID,
		Granularity:       gran,
		Content:           content,
		AsOfDate:          to,
		Source:            "llm_assisted",
	})
}

// RefreshMonth incrementally merges new sections since as_of_date with the old month context.
// as_of_date is advanced to the exclusive boundary (next month 1st) of the current month.
func (s *LifelineContextService) RefreshMonth(ctx context.Context, topicID uint, now time.Time) error {
	gran := string(repository.GranularityMonth)
	return s.refreshIncremental(ctx, topicID, gran, now)
}

// RefreshYear incrementally merges new sections since as_of_date with the old year context.
func (s *LifelineContextService) RefreshYear(ctx context.Context, topicID uint, now time.Time) error {
	gran := string(repository.GranularityYear)
	return s.refreshIncremental(ctx, topicID, gran, now)
}

// RefreshAll incrementally merges new sections since as_of_date with the old 'all' context.
func (s *LifelineContextService) RefreshAll(ctx context.Context, topicID uint, now time.Time) error {
	gran := string(repository.GranularityAll)
	return s.refreshIncremental(ctx, topicID, gran, now)
}

// RefreshGranularity dispatches to the appropriate refresh method based on granularity.
func (s *LifelineContextService) RefreshGranularity(ctx context.Context, topicID uint, granularity string, now time.Time) error {
	switch granularity {
	case string(repository.GranularityWeek):
		return s.RefreshWeek(ctx, topicID, now)
	case string(repository.GranularityMonth):
		return s.RefreshMonth(ctx, topicID, now)
	case string(repository.GranularityYear):
		return s.RefreshYear(ctx, topicID, now)
	case string(repository.GranularityAll):
		return s.RefreshAll(ctx, topicID, now)
	default:
		return fmt.Errorf("unknown granularity: %s", granularity)
	}
}

// HealStale scans for topics with stale lifeline contexts and patches them cycle by cycle.
// It reads stale contexts via ListStaleTopicLifelineContexts, then for each stale context:
// from as_of_date (exclusive boundary of last patched period), iterates forward by
// granularity period blocks, merging incremental sections + old context, advancing
// as_of_date sequentially until it reaches the current period's exclusive boundary.
func (s *LifelineContextService) HealStale(ctx context.Context, granularity string, now time.Time) error {
	sinceDays := staleSinceDays(granularity)
	staleList, err := s.repo.ListStaleTopicLifelineContexts(ctx, granularity, sinceDays)
	if err != nil {
		return fmt.Errorf("heal stale: list: %w", err)
	}

	currentEnd := currentExclusiveEnd(now, granularity)

	for _, staleCtx := range staleList {
		topicID := staleCtx.PersistentTopicID
		start := staleCtx.AsOfDate

		for start.Before(currentEnd) {
			end := periodEnd(start, granularity)
			// Don't go past current time for 'all' granularity.
			if end.After(currentEnd) {
				end = currentEnd
			}

			if err := s.refreshIncrementalPeriod(ctx, topicID, granularity, start, end); err != nil {
				return fmt.Errorf("heal stale: topic %d patch [%s, %s): %w",
					topicID, start.Format("2006-01-02"), end.Format("2006-01-02"), err)
			}
			start = end
		}
	}
	return nil
}

// ── Private helpers ─────────────────────────────────────────────────────────

// refreshIncremental reads sections since as_of_date, merges with old context, upserts.
func (s *LifelineContextService) refreshIncremental(ctx context.Context, topicID uint, granularity string, now time.Time) error {
	from, to := granularityRange(now, granularity)

	// Check if there's an old context.
	oldCtx, _ := s.repo.GetTopicLifelineContext(ctx, topicID, granularity)
	var oldAsOf time.Time
	if oldCtx != nil {
		oldAsOf = oldCtx.AsOfDate
	}
	// If no old context or old as_of_date is before from, use from as the start.
	if oldAsOf.IsZero() || oldAsOf.Before(from) {
		oldAsOf = from
	}

	sectionsText, err := s.sectionReader.ReadSections(ctx, topicID, oldAsOf, to)
	if err != nil {
		return fmt.Errorf("refresh incremental: read sections: %w", err)
	}

	oldContent := ""
	if oldCtx != nil {
		oldContent = oldCtx.Content
	}

	content, err := s.summarizeIncremental(ctx, topicID, granularity, sectionsText, oldContent)
	if err != nil {
		return fmt.Errorf("refresh incremental: summarize: %w", err)
	}

	return s.repo.UpsertTopicLifelineContext(ctx, &repository.TopicLifelineContext{
		PersistentTopicID: topicID,
		Granularity:       granularity,
		Content:           content,
		AsOfDate:          to,
		Source:            "llm_assisted",
	})
}

// refreshIncrementalPeriod patches one specific period block for self-healing.
func (s *LifelineContextService) refreshIncrementalPeriod(ctx context.Context, topicID uint, granularity string, from, to time.Time) error {
	sectionsText, err := s.sectionReader.ReadSections(ctx, topicID, from, to)
	if err != nil {
		return fmt.Errorf("read sections: %w", err)
	}

	oldCtx, _ := s.repo.GetTopicLifelineContext(ctx, topicID, granularity)
	oldContent := ""
	if oldCtx != nil {
		oldContent = oldCtx.Content
	}

	content, err := s.summarizeIncremental(ctx, topicID, granularity, sectionsText, oldContent)
	if err != nil {
		return fmt.Errorf("summarize: %w", err)
	}

	return s.repo.UpsertTopicLifelineContext(ctx, &repository.TopicLifelineContext{
		PersistentTopicID: topicID,
		Granularity:       granularity,
		Content:           content,
		AsOfDate:          to,
		Source:            "llm_assisted",
	})
}

// summarizeWeek calls the LLM for a direct week summary (no old context).
func (s *LifelineContextService) summarizeWeek(ctx context.Context, topicID uint, sectionsText string) (string, error) {
	prompt := fmt.Sprintf(
		"你是话题新闻汇总助手。下面是一个话题在过去一周的新闻内容，请用中文总结：\n"+
			"1. 本周主要发生了哪些事件（按时间线）\n"+
			"2. 数据上有什么显著波动\n"+
			"只陈述客观事实，不做分析判断。\n\n"+
			"--- 新闻内容 ---\n%s", sectionsText)

	return s.callLLM(ctx, topicID, string(repository.GranularityWeek), prompt)
}

// summarizeIncremental merges new sections with old context via LLM.
func (s *LifelineContextService) summarizeIncremental(ctx context.Context, topicID uint, granularity string, sectionsText, oldContent string) (string, error) {
	var prompt string
	if oldContent != "" {
		prompt = fmt.Sprintf(
			"你是话题新闻汇总助手。下面是已有历史汇总 + 新的增量新闻，请将它们合并成新的汇总。\n\n"+
				"--- 已有汇总 ---\n%s\n\n--- 增量新闻 ---\n%s\n\n"+
				"请合并输出：\n1. 话题整体发展脉络（新进展融入历史背景）\n2. 本期增量亮点\n3. 相关数据波动\n"+
				"只陈述客观事实，不做分析判断。", oldContent, sectionsText)
	} else {
		prompt = fmt.Sprintf(
			"你是话题新闻汇总助手。下面是增量新闻内容，请总结：\n\n"+
				"--- 新闻内容 ---\n%s\n\n"+
				"请输出：\n1. 话题发展脉络\n2. 主要事件与数据波动\n"+
				"只陈述客观事实，不做分析判断。", sectionsText)
	}

	return s.callLLM(ctx, topicID, granularity, prompt)
}

// callLLM invokes airouter.Chat with the proper capability, operation, and session_id.
func (s *LifelineContextService) callLLM(ctx context.Context, topicID uint, granularity, prompt string) (string, error) {
	sessionID := fmt.Sprintf("lifeline_context_%d_%s_%s", topicID, granularity, uuid8())
	temp := 0.3
	result, err := s.airouter.Chat(ctx, airouter.ChatRequest{
		Capability: s.capability,
		Operation:  "data_enrichment.summarize_context",
		SessionID:  sessionID,
		Messages: []airouter.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: &temp,
	})
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}
	return result.Content, nil
}

// uuid8 returns 8 hex characters suitable for session ID uniqueness.
func uuid8() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Granularity period helpers ──────────────────────────────────────────────

// weekRange returns the [Monday 00:00, next Monday 00:00) window containing t.
func weekRange(t time.Time) (from, to time.Time) {
	weekday := t.Weekday()
	daysFromMonday := int(weekday) - int(time.Monday)
	if daysFromMonday < 0 {
		daysFromMonday += 7
	}
	monday := t.AddDate(0, 0, -daysFromMonday)
	from = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	to = from.AddDate(0, 0, 7)
	return
}

// monthRange returns the [1st of month 00:00, 1st of next month 00:00) window.
func monthRange(t time.Time) (from, to time.Time) {
	from = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	to = from.AddDate(0, 1, 0)
	return
}

// yearRange returns the [Jan 1 00:00, next Jan 1 00:00) window.
func yearRange(t time.Time) (from, to time.Time) {
	from = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	to = from.AddDate(1, 0, 0)
	return
}

// granularityRange returns the current period range for the given granularity.
// For 'all', returns [zero, now).
func granularityRange(now time.Time, granularity string) (from, to time.Time) {
	switch granularity {
	case string(repository.GranularityWeek):
		return weekRange(now)
	case string(repository.GranularityMonth):
		return monthRange(now)
	case string(repository.GranularityYear):
		return yearRange(now)
	case string(repository.GranularityAll):
		return time.Time{}, now
	default:
		return weekRange(now)
	}
}

// periodEnd returns the exclusive boundary after one period from start.
func periodEnd(start time.Time, granularity string) time.Time {
	switch granularity {
	case string(repository.GranularityWeek):
		return start.AddDate(0, 0, 7)
	case string(repository.GranularityMonth):
		return start.AddDate(0, 1, 0)
	case string(repository.GranularityYear):
		return start.AddDate(1, 0, 0)
	case string(repository.GranularityAll):
		return time.Now()
	default:
		return start.AddDate(0, 0, 7)
	}
}

// currentExclusiveEnd returns the exclusive boundary of the current period for heal detection.
func currentExclusiveEnd(now time.Time, granularity string) time.Time {
	switch granularity {
	case string(repository.GranularityWeek):
		_, to := weekRange(now)
		return to
	case string(repository.GranularityMonth):
		_, to := monthRange(now)
		return to
	case string(repository.GranularityYear):
		_, to := yearRange(now)
		return to
	case string(repository.GranularityAll):
		return now
	default:
		_, to := weekRange(now)
		return to
	}
}

// staleSinceDays returns the number of days used for ListStaleTopicLifelineContexts.
func staleSinceDays(granularity string) int {
	switch granularity {
	case string(repository.GranularityWeek):
		return 8
	case string(repository.GranularityMonth):
		return 32
	case string(repository.GranularityYear):
		return 366
	case string(repository.GranularityAll):
		return 30
	default:
		return 8
	}
}
