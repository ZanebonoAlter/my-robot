package service

import (
	"context"
	"time"
)

// SectionReader reads news content text for a topic within a date range.
// Production implementation queries daily_report_sections joined with
// daily_report_threads and renders section news as LLM-friendly text blocks.
type SectionReader interface {
	// ReadSections returns news content text for a topic within [from, to).
	// Each section rendered as: "日期 [cluster_label]: thread标题1; thread标题2; ..."
	ReadSections(ctx context.Context, topicID uint, from, to time.Time) (string, error)

	// SectionDates returns the distinct dates (from board_daily_reports.period_date)
	// that have ≥1 section for the topic, ascending. Used to know which periods have data.
	SectionDates(ctx context.Context, topicID uint) ([]time.Time, error)
}
