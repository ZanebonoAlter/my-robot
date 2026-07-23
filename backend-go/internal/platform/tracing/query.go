package tracing

import (
	"time"

	"gorm.io/gorm"
)

type TraceSummary struct {
	TraceID      string `json:"trace_id"`
	RootSpanName string `json:"root_span_name"`
	SpanCount    int    `json:"span_count"`
	StartTime    int64  `json:"start_time_unix_nano"`
	DurationMs   int64  `json:"duration_ms"`
	StatusCode   int    `json:"status_code"`
	ServiceName  string `json:"service_name"`
}

type TraceDetail struct {
	OtelSpan
	Children []TraceDetail `json:"children,omitempty" gorm:"-"`
}

func QueryByTraceID(db *gorm.DB, traceID string) ([]OtelSpan, error) {
	var spans []OtelSpan
	err := db.Where("trace_id = ?", traceID).
		Order("start_time_unix_nano ASC").
		Find(&spans).Error
	return spans, err
}

// QuerySpansByTraceIDs returns all spans belonging to the given trace IDs,
// ordered by start time ascending. Used by the session aggregation endpoint
// to assemble a timeline across every trace that one orchestration session
// touched (trace_id is the join key written into ai_call_logs by LogCall).
func QuerySpansByTraceIDs(db *gorm.DB, traceIDs []string) ([]OtelSpan, error) {
	if len(traceIDs) == 0 {
		return nil, nil
	}
	var spans []OtelSpan
	err := db.Where("trace_id IN ?", traceIDs).
		Order("start_time_unix_nano ASC").
		Find(&spans).Error
	return spans, err
}

func QueryRecentTraces(db *gorm.DB, limit int) ([]TraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	var traces []TraceSummary
	err := db.Raw(`
		SELECT
			trace_id,
			(SELECT name FROM otel_spans s2 WHERE s2.trace_id = s.trace_id ORDER BY start_time_unix_nano ASC LIMIT 1) as root_span_name,
			COUNT(*) as span_count,
			MIN(start_time_unix_nano) as start_time_unix_nano,
			(MAX(end_time_unix_nano) - MIN(start_time_unix_nano)) / 1000000 as duration_ms,
			MAX(CASE WHEN status_code = 1 THEN 1 ELSE 0 END) as status_code,
			MAX(service_name) as service_name
		FROM otel_spans s
		WHERE parent_span_id = '' OR parent_span_id = '0000000000000000'
		GROUP BY trace_id
		ORDER BY start_time_unix_nano DESC
		LIMIT ?
	`, limit).Scan(&traces).Error
	return traces, err
}

func QueryTracesByOperation(db *gorm.DB, operation string, limit int) ([]TraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	var traces []TraceSummary
	err := db.Raw(`
		SELECT
			s.trace_id,
			s.name as root_span_name,
			(SELECT COUNT(*) FROM otel_spans s3 WHERE s3.trace_id = s.trace_id) as span_count,
			s.start_time_unix_nano,
			(SELECT (MAX(s4.end_time_unix_nano) - MIN(s4.start_time_unix_nano)) / 1000000 FROM otel_spans s4 WHERE s4.trace_id = s.trace_id) as duration_ms,
			s.status_code,
			s.service_name
		FROM otel_spans s
		WHERE s.name LIKE ? AND (s.parent_span_id = '' OR s.parent_span_id = '0000000000000000')
		GROUP BY s.trace_id
		ORDER BY s.start_time_unix_nano DESC
		LIMIT ?
	`, "%"+operation+"%", limit).Scan(&traces).Error
	return traces, err
}

func QuerySlowTraces(db *gorm.DB, minDurationMs int64, limit int) ([]TraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	var traces []TraceSummary
	err := db.Raw(`
		SELECT
			s.trace_id,
			s.name as root_span_name,
			(SELECT COUNT(*) FROM otel_spans s3 WHERE s3.trace_id = s.trace_id) as span_count,
			s.start_time_unix_nano,
			(SELECT (MAX(s4.end_time_unix_nano) - MIN(s4.start_time_unix_nano)) / 1000000 FROM otel_spans s4 WHERE s4.trace_id = s.trace_id) as duration_ms,
			s.status_code,
			s.service_name
		FROM otel_spans s
		WHERE (s.parent_span_id = '' OR s.parent_span_id = '0000000000000000')
		GROUP BY s.trace_id
		HAVING duration_ms >= ?
		ORDER BY duration_ms DESC
		LIMIT ?
	`, minDurationMs, limit).Scan(&traces).Error
	return traces, err
}

func QueryErrorTraces(db *gorm.DB, limit int) ([]TraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	var traces []TraceSummary
	err := db.Raw(`
		SELECT
			s.trace_id,
			s.name as root_span_name,
			(SELECT COUNT(*) FROM otel_spans s3 WHERE s3.trace_id = s.trace_id) as span_count,
			s.start_time_unix_nano,
			(SELECT (MAX(s4.end_time_unix_nano) - MIN(s4.start_time_unix_nano)) / 1000000 FROM otel_spans s4 WHERE s4.trace_id = s.trace_id) as duration_ms,
			1 as status_code,
			s.service_name
		FROM otel_spans s
		WHERE s.trace_id IN (
			SELECT DISTINCT trace_id FROM otel_spans WHERE status_code = 1
		) AND (s.parent_span_id = '' OR s.parent_span_id = '0000000000000000')
		GROUP BY s.trace_id
		ORDER BY s.start_time_unix_nano DESC
		LIMIT ?
	`, limit).Scan(&traces).Error
	return traces, err
}

func QueryStats(db *gorm.DB) (map[string]interface{}, error) {
	var totalTraces int64
	db.Raw("SELECT COUNT(DISTINCT trace_id) FROM otel_spans").Scan(&totalTraces)

	var totalSpans int64
	db.Raw("SELECT COUNT(*) FROM otel_spans").Scan(&totalSpans)

	var errorTraces int64
	db.Raw("SELECT COUNT(DISTINCT trace_id) FROM otel_spans WHERE status_code = 1").Scan(&errorTraces)

	var successRate float64
	if totalTraces > 0 {
		successRate = float64(totalTraces-errorTraces) / float64(totalTraces) * 100
	}

	var p50, p95, p99 int64
	db.Raw(`
		SELECT MAX(duration_ms) FROM (
			SELECT (MAX(end_time_unix_nano) - MIN(start_time_unix_nano)) / 1000000 as duration_ms
			FROM otel_spans GROUP BY trace_id
			ORDER BY duration_ms
			LIMIT 1 OFFSET (SELECT COUNT(DISTINCT trace_id) * 50 / 100 FROM otel_spans)
		)
	`).Scan(&p50)
	db.Raw(`
		SELECT MAX(duration_ms) FROM (
			SELECT (MAX(end_time_unix_nano) - MIN(start_time_unix_nano)) / 1000000 as duration_ms
			FROM otel_spans GROUP BY trace_id
			ORDER BY duration_ms
			LIMIT 1 OFFSET (SELECT COUNT(DISTINCT trace_id) * 95 / 100 FROM otel_spans)
		)
	`).Scan(&p95)
	db.Raw(`
		SELECT MAX(duration_ms) FROM (
			SELECT (MAX(end_time_unix_nano) - MIN(start_time_unix_nano)) / 1000000 as duration_ms
			FROM otel_spans GROUP BY trace_id
			ORDER BY duration_ms
			LIMIT 1 OFFSET (SELECT COUNT(DISTINCT trace_id) * 99 / 100 FROM otel_spans)
		)
	`).Scan(&p99)

	type TopOperation struct {
		Name      string  `json:"name"`
		Count     int64   `json:"count"`
		AvgMs     int64   `json:"avg_ms"`
		ErrorRate float64 `json:"error_rate"`
	}
	var topOperations []TopOperation
	db.Raw(`
		SELECT
			name,
			COUNT(*) as count,
			AVG(duration_ms) as avg_ms,
			SUM(CASE WHEN status_code = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as error_rate
		FROM otel_spans
		WHERE parent_span_id = '' OR parent_span_id = '0000000000000000'
		GROUP BY name
		ORDER BY count DESC
		LIMIT 10
	`).Scan(&topOperations)

	cutoff := time.Now().AddDate(0, 0, -1)
	var last24hCount int64
	db.Raw("SELECT COUNT(DISTINCT trace_id) FROM otel_spans WHERE created_at >= ?", cutoff).Scan(&last24hCount)

	return map[string]interface{}{
		"total_traces":    totalTraces,
		"total_spans":     totalSpans,
		"error_traces":    errorTraces,
		"success_rate":    successRate,
		"p50_ms":          p50,
		"p95_ms":          p95,
		"p99_ms":          p99,
		"top_operations":  topOperations,
		"last_24h_traces": last24hCount,
	}, nil
}

// BuildSpanTree assembles a flat span list into a parent-to-children tree.
// Spans whose ParentSpanID is empty, all-zero, or absent from the set become
// roots. Children are attached recursively so multi-level nesting is preserved.
//
// NOTE: an earlier value-copy version appended *detail into roots before
// walking children, which dropped any grandchild span (root.Children was
// captured before child spans were attached to it). The recursive build below
// fixes that.
func BuildSpanTree(spans []OtelSpan) []TraceDetail {
	indexOf := make(map[string]int, len(spans))
	for i, s := range spans {
		indexOf[s.SpanID] = i
	}
	childrenOf := make(map[string][]int)
	var rootIdx []int
	for i, s := range spans {
		pid := s.ParentSpanID
		if pid == "" || pid == "0000000000000000" {
			rootIdx = append(rootIdx, i)
			continue
		}
		if _, ok := indexOf[pid]; ok {
			childrenOf[pid] = append(childrenOf[pid], i)
		} else {
			rootIdx = append(rootIdx, i)
		}
	}

	var build func(idx int) TraceDetail
	build = func(idx int) TraceDetail {
		s := spans[idx]
		d := TraceDetail{OtelSpan: s, Children: []TraceDetail{}}
		for _, ci := range childrenOf[s.SpanID] {
			d.Children = append(d.Children, build(ci))
		}
		return d
	}

	roots := make([]TraceDetail, 0, len(rootIdx))
	for _, i := range rootIdx {
		roots = append(roots, build(i))
	}
	return roots
}
