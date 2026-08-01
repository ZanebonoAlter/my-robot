package repository

import (
	"fmt"
	"strconv"
	"time"
)

// Stance values derived for the topic-landscape view (design §2). Mutually
// exclusive; derived in priority order inside deriveTopicStance.
const (
	StanceEmerging = "emerging" // 🌱 candidate, sub-threshold hits
	StancePending  = "pending"  // 🔴 candidate, meets upgrade threshold (can_activate)
	StanceActive   = "active"   // 🟢 active, fresh (seen within activeWindow days)
	StanceStalled  = "stalled"  // ⏸️ active, stale or no consecutive hits
	StanceArchived = "archived" // ⬛ archived
)

// TopicLandscapeLifelinePoint is one calendar day in a topic's mini-lifeline.
// Identity-track aggregated (section.persistent_topic_id); gap days are
// zero-filled so the date axis stays continuous (design §4).
type TopicLandscapeLifelinePoint struct {
	Date         string `json:"date"`          // YYYY-MM-DD, server-local day
	SectionCount int    `json:"section_count"` // sections of this topic that day (0 on gap days)
}

// TopicLandscapeTopic is one card in the topic-landscape wall.
type TopicLandscapeTopic struct {
	ID              uint                          `json:"id"`
	Label           string                        `json:"label"`
	Status          string                        `json:"status"`
	Source          string                        `json:"source"`
	Stance          string                        `json:"stance"`
	IsVacuum        bool                          `json:"is_vacuum"`
	VacuumStrong    int                           `json:"vacuum_strong"`
	HitCount        int                           `json:"hit_count"`
	ConsecutiveHits int                           `json:"consecutive_hits"`
	FirstSeenDate   string                        `json:"first_seen_date"`
	LastSeenDate    string                        `json:"last_seen_date"`
	DaysSinceLast   int                           `json:"days_since_last"`
	CanActivate     bool                          `json:"can_activate"`
	Lifeline        []TopicLandscapeLifelinePoint `json:"lifeline"`
}

// TopicLandscapeVitality is the top vitality bar of the landscape view.
type TopicLandscapeVitality struct {
	Days             int   `json:"days"`
	ArticleCount     int   `json:"article_count"`
	SectionCount     int   `json:"section_count"`
	ActiveTopicCount int   `json:"active_topic_count"`
	FeedActive       *int  `json:"feed_active"` // MVP: always null (cross-domain feed count, later)
	Trend            []int `json:"trend"`       // per-day board section count over the window
}

// TopicLandscapeResponse is the data payload of
// GET /api/semantic-boards/:id/topic-landscape (design §3 contract).
type TopicLandscapeResponse struct {
	Topics   []TopicLandscapeTopic  `json:"topics"`
	Vitality TopicLandscapeVitality `json:"vitality"`
}

// allowedTopicLandscapeDays is the legal lifeline window set (design §3).
var allowedTopicLandscapeDays = []int{7, 14, 30, 90}

// ClampTopicLandscapeDays parses the ?days= query value and clamps it to the
// allowed set {7,14,30,90}. Empty / non-numeric / non-positive → default 30;
// any other positive value snaps to the nearest allowed value (lower wins on
// a tie). Used by the landscape HTTP handler.
func ClampTopicLandscapeDays(raw string) int {
	if raw == "" {
		return topicLandscapeDefaultDays
	}
	d, err := strconv.Atoi(raw)
	if err != nil || d <= 0 {
		return topicLandscapeDefaultDays
	}
	for _, a := range allowedTopicLandscapeDays {
		if d == a {
			return d
		}
	}
	best := allowedTopicLandscapeDays[0]
	bestDelta := intAbs(d - best)
	for _, a := range allowedTopicLandscapeDays[1:] {
		delta := intAbs(d - a)
		if delta < bestDelta {
			best = a
			bestDelta = delta
		}
	}
	return best
}

func intAbs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// GetBoardTopicLandscape builds the topic-landscape view for a board over the
// given window (days). It reuses the same visible-topic filter as the /topics
// management endpoint, derives each topic's stance from identity-track fields
// (design §2, server-derived for a single source of truth), and aggregates a
// per-day section-count lifeline + board vitality from the identity track
// (section.persistent_topic_id). It is strictly READ-ONLY: it touches no
// assignment / matching / lane state.
//
// Empty-board semantics (design §6): when the board has no daily report at
// all, the payload is the empty state — topics=[] and vitality.trend=[].
func (r *TopicGraphRepository) GetBoardTopicLandscape(boardID uint, days int) (*TopicLandscapeResponse, error) {
	if days <= 0 {
		days = topicLandscapeDefaultDays
	}

	// Empty-board guard: no daily reports at all → empty-state payload.
	var reportCount int64
	if err := r.db.Model(&BoardDailyReport{}).
		Where("semantic_board_id = ?", boardID).
		Count(&reportCount).Error; err != nil {
		return nil, fmt.Errorf("count board reports: %w", err)
	}
	if reportCount == 0 {
		return emptyLandscapeResponse(days), nil
	}

	// Visible topics. Unlike the /topics management filter (FilterVisibleTopics,
	// which hides sub-threshold candidates), the landscape intentionally keeps
	// emerging candidates (🌱 hit < threshold) — surfacing new sprouts is its
	// core value — so we only drop pure orphans (hit == 0), regardless of status.
	topics, err := r.ListTopicsByBoardAll(boardID)
	if err != nil {
		return nil, fmt.Errorf("list board topics: %w", err)
	}
	// upgradeThreshold still feeds the pending vs emerging stance split in
	// deriveTopicStance below; it no longer gates visibility here.
	upgradeThreshold := LoadPersistentTopicConfig(r.db).UpgradeThreshold
	topics = filterLandscapeVisible(topics)

	// Date axis [today-days .. today] inclusive → days+1 points (mirrors the
	// generate_series bounds in design §3). Computed in server-local time.
	axis := landscapeDateAxis(days)
	start, today := axis[0], axis[len(axis)-1]

	// Vitality: per-day board section count + article sum, zero-filled across
	// the whole axis via generate_series LEFT JOIN.
	type vitalityRow struct {
		Date         string
		SectionCount int
		ArticleCount int
	}
	var vitRows []vitalityRow
	err = r.db.Raw(`
		SELECT (g.d)::date::text AS date,
		       COALESCE(t.sec_cnt, 0) AS section_count,
		       COALESCE(t.art_cnt, 0) AS article_count
		FROM generate_series(?::date, ?::date, INTERVAL '1 day') AS g(d)
		LEFT JOIN (
		  SELECT r.period_date AS rd,
		         COUNT(*) AS sec_cnt,
		         COALESCE(SUM(s.article_count), 0) AS art_cnt
		  FROM daily_report_sections s
		  JOIN board_daily_reports r ON r.id = s.report_id
		  WHERE r.semantic_board_id = ?
		    AND r.period_date BETWEEN ?::date AND ?::date
		  GROUP BY r.period_date
		) t ON t.rd = (g.d)::date
		ORDER BY g.d
	`, start, today, boardID, start, today).Scan(&vitRows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate board vitality: %w", err)
	}

	trend := make([]int, len(vitRows))
	totalSections := 0
	totalArticles := 0
	for i, row := range vitRows {
		trend[i] = row.SectionCount
		totalSections += row.SectionCount
		totalArticles += row.ArticleCount
	}

	// Per-topic daily section counts in the window (identity track). Grouped in
	// one query for all visible topics; assembled against the axis in Go so gap
	// days zero-fill without a second generate_series per topic.
	countByTopic := make(map[uint]map[string]int, len(topics))
	if len(topics) > 0 {
		ids := make([]uint, len(topics))
		for i, t := range topics {
			ids[i] = t.ID
		}
		type topicDayCount struct {
			TopicID      uint
			Date         string
			SectionCount int
		}
		var rows []topicDayCount
		err = r.db.Raw(`
			SELECT s.persistent_topic_id AS topic_id,
			       r.period_date::text AS date,
			       COUNT(*) AS section_count
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE s.persistent_topic_id IN ?
			  AND r.period_date BETWEEN ?::date AND ?::date
			GROUP BY s.persistent_topic_id, r.period_date
		`, ids, start, today).Scan(&rows).Error
		if err != nil {
			return nil, fmt.Errorf("aggregate topic lifelines: %w", err)
		}
		for _, row := range rows {
			m := countByTopic[row.TopicID]
			if m == nil {
				m = make(map[string]int)
				countByTopic[row.TopicID] = m
			}
			m[row.Date] = row.SectionCount
		}
	}

	// Assemble topic cards + count active stance for the vitality bar.
	out := make([]TopicLandscapeTopic, 0, len(topics))
	activeCount := 0
	for _, t := range topics {
		daysSince := daysSinceLastSeen(t.LastSeenDate)
		stance := deriveTopicStance(t, daysSince, upgradeThreshold, topicLandscapeActiveWindowDays)
		if stance == StanceActive {
			activeCount++
		}
		lifeline := make([]TopicLandscapeLifelinePoint, len(axis))
		dayMap := countByTopic[t.ID]
		for i, d := range axis {
			lifeline[i] = TopicLandscapeLifelinePoint{
				Date:         d,
				SectionCount: dayMap[d],
			}
		}
		out = append(out, TopicLandscapeTopic{
			ID:              t.ID,
			Label:           t.Label,
			Status:          t.Status,
			Source:          t.Source,
			Stance:          stance,
			IsVacuum:        t.IsVacuum,
			VacuumStrong:    t.VacuumStrong,
			HitCount:        t.HitCount,
			ConsecutiveHits: t.ConsecutiveHits,
			FirstSeenDate:   t.FirstSeenDate.Format("2006-01-02"),
			LastSeenDate:    t.LastSeenDate.Format("2006-01-02"),
			DaysSinceLast:   daysSince,
			CanActivate:     stance == StancePending,
			Lifeline:        lifeline,
		})
	}

	return &TopicLandscapeResponse{
		Topics: out,
		Vitality: TopicLandscapeVitality{
			Days:             days,
			ArticleCount:     totalArticles,
			SectionCount:     totalSections,
			ActiveTopicCount: activeCount,
			FeedActive:       nil,
			Trend:            trend,
		},
	}, nil
}

// emptyLandscapeResponse is the empty-board payload (design §6): no topics,
// empty trend. Slices are non-nil so the JSON serializes as [] not null.
func emptyLandscapeResponse(days int) *TopicLandscapeResponse {
	return &TopicLandscapeResponse{
		Topics: []TopicLandscapeTopic{},
		Vitality: TopicLandscapeVitality{
			Days:             days,
			ArticleCount:     0,
			SectionCount:     0,
			ActiveTopicCount: 0,
			FeedActive:       nil,
			Trend:            []int{},
		},
	}
}

// filterLandscapeVisible keeps topics observed at least once (HitCount >= 1),
// dropping only pure orphans (hit == 0). Unlike FilterVisibleTopics (the
// /topics management filter, which hides sub-threshold candidates), the
// landscape view intentionally surfaces emerging candidates (🌱 hit below the
// upgrade threshold) because spotting new sprouts is its core value. Status
// is not considered: active/archived/candidate are all kept once they have a
// single hit (in practice only orphan candidates ever have hit == 0).
func filterLandscapeVisible(topics []BoardPersistentTopic) []BoardPersistentTopic {
	result := make([]BoardPersistentTopic, 0, len(topics))
	for _, t := range topics {
		if t.HitCount >= 1 {
			result = append(result, t)
		}
	}
	return result
}

// deriveTopicStance maps identity-track fields to a single landscape stance
// (design §2, first-match order). Pure function — no DB — so it is unit
// testable in isolation. Vacuum (is_vacuum) is an orthogonal overlay and does
// NOT influence this derivation.
func deriveTopicStance(t BoardPersistentTopic, daysSinceLast int, upgradeThreshold int, activeWindow int) string {
	switch t.Status {
	case TopicStatusArchived:
		return StanceArchived
	case TopicStatusCandidate:
		if t.HitCount >= upgradeThreshold {
			return StancePending
		}
		return StanceEmerging
	case TopicStatusActive:
		if t.ConsecutiveHits > 0 && daysSinceLast <= activeWindow {
			return StanceActive
		}
		return StanceStalled
	default:
		return StanceStalled
	}
}

// daysSinceLastSeen returns the calendar-day difference between lastSeen and
// today in server-local time (matching the daily-report generation timezone).
// Both operands are reduced to local midnight so a few hours of timezone skew
// cannot flip the day count.
func daysSinceLastSeen(lastSeen time.Time) int {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	day := time.Date(lastSeen.Year(), lastSeen.Month(), lastSeen.Day(), 0, 0, 0, 0, time.Local)
	return int(today.Sub(day).Hours() / 24)
}

// landscapeDateAxis returns the inclusive [today-days, today] day list as
// YYYY-MM-DD strings in server-local time. Length = days+1 (mirrors the
// generate_series bounds in design §3).
func landscapeDateAxis(days int) []string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	start := today.AddDate(0, 0, -days)
	axis := make([]string, 0, days+1)
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		axis = append(axis, d.Format("2006-01-02"))
	}
	return axis
}
