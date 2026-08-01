package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/platform/testutil"
)

// ── Pure unit tests (no DB, run under -short) ───────────────────────────────

func TestDeriveTopicStance_AllBranches(t *testing.T) {
	upgrade := 3
	window := topicLandscapeActiveWindowDays
	cases := []struct {
		name    string
		topic   BoardPersistentTopic
		days    int
		stance  string
	}{
		{
			name:   "archived regardless of recency",
			topic:  BoardPersistentTopic{Status: TopicStatusArchived, HitCount: 99, ConsecutiveHits: 99},
			days:   0,
			stance: StanceArchived,
		},
		{
			name:   "candidate at threshold is pending",
			topic:  BoardPersistentTopic{Status: TopicStatusCandidate, HitCount: upgrade},
			stance: StancePending,
		},
		{
			name:   "candidate above threshold is pending",
			topic:  BoardPersistentTopic{Status: TopicStatusCandidate, HitCount: upgrade + 5},
			stance: StancePending,
		},
		{
			name:   "candidate below threshold is emerging",
			topic:  BoardPersistentTopic{Status: TopicStatusCandidate, HitCount: 1},
			stance: StanceEmerging,
		},
		{
			name:   "active with consecutive hits and fresh is active",
			topic:  BoardPersistentTopic{Status: TopicStatusActive, ConsecutiveHits: 2, HitCount: 5},
			days:   window,
			stance: StanceActive,
		},
		{
			name:   "active fresh but zero consecutive is stalled",
			topic:  BoardPersistentTopic{Status: TopicStatusActive, ConsecutiveHits: 0, HitCount: 5},
			days:   0,
			stance: StanceStalled,
		},
		{
			name:   "active with consecutive but stale is stalled",
			topic:  BoardPersistentTopic{Status: TopicStatusActive, ConsecutiveHits: 5, HitCount: 5},
			days:   window + 1,
			stance: StanceStalled,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveTopicStance(tc.topic, tc.days, upgrade, window)
			assert.Equal(t, tc.stance, got)
		})
	}
}

func TestDeriveTopicStance_VacuumDoesNotChangeStance(t *testing.T) {
	// is_vacuum is an orthogonal overlay; it must not flip the main stance.
	t1 := BoardPersistentTopic{Status: TopicStatusActive, ConsecutiveHits: 3, HitCount: 10, IsVacuum: true, VacuumStrong: 53}
	assert.Equal(t, StanceActive, deriveTopicStance(t1, 0, 3, topicLandscapeActiveWindowDays))
}

func TestFilterLandscapeVisible_KeepsEmergingDropsOrphans(t *testing.T) {
	// The landscape filter keeps anything hit >= 1 (including emerging
	// sub-threshold candidates) and drops only pure orphans (hit == 0),
	// regardless of status.
	topics := []BoardPersistentTopic{
		{Status: TopicStatusCandidate, HitCount: 1}, // emerging-tier → keep
		{Status: TopicStatusCandidate, HitCount: 0}, // orphan → drop
		{Status: TopicStatusCandidate, HitCount: 5}, // pending-tier → keep
		{Status: TopicStatusActive, HitCount: 0},    // hit-less → drop
		{Status: TopicStatusActive, HitCount: 3},    // active → keep
		{Status: TopicStatusArchived, HitCount: 2},  // archived → keep
		{Status: TopicStatusArchived, HitCount: 0},  // hit-less → drop
	}
	got := filterLandscapeVisible(topics)
	require.Len(t, got, 4, "only the four topics with hit >= 1 survive")
	assert.Equal(t, 1, got[0].HitCount)
	assert.Equal(t, 5, got[1].HitCount)
	assert.Equal(t, 3, got[2].HitCount)
	assert.Equal(t, 2, got[3].HitCount)
}

func TestClampTopicLandscapeDays(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", topicLandscapeDefaultDays},   // missing → default
		{"0", topicLandscapeDefaultDays},  // non-positive → default
		{"-5", topicLandscapeDefaultDays}, // negative → default
		{"abc", topicLandscapeDefaultDays}, // non-numeric → default
		{"7", 7},  // exact
		{"14", 14}, // exact
		{"30", 30}, // exact
		{"90", 90}, // exact
		{"10", 7},  // nearest: |10-7|=3 < |10-14|=4
		{"20", 14}, // nearest: |20-14|=6 < |20-30|=10
		{"60", 30}, // tie |60-30|=30 == |60-90|=30 → lower (30) wins
		{"100", 90}, // nearest above 90 snaps down to 90
		{"8", 7},   // nearest
		{"12", 14}, // |12-7|=5, |12-14|=2 → 14
	}
	for _, tc := range cases {
		t.Run("days="+tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, ClampTopicLandscapeDays(tc.in))
		})
	}
}

func TestLandscapeDateAxis_LengthAndBounds(t *testing.T) {
	axis := landscapeDateAxis(7)
	require.Len(t, axis, 8) // days+1 inclusive
	// First entry is 7 days ago, last entry is today (server-local).
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	start := today.AddDate(0, 0, -7)
	assert.Equal(t, start.Format("2006-01-02"), axis[0])
	assert.Equal(t, today.Format("2006-01-02"), axis[len(axis)-1])
	// Consecutive days, no gaps.
	for i := 1; i < len(axis); i++ {
		prev, err := time.Parse("2006-01-02", axis[i-1])
		require.NoError(t, err)
		cur, err := time.Parse("2006-01-02", axis[i])
		require.NoError(t, err)
		assert.Equal(t, 24*time.Hour, cur.Sub(prev))
	}
}

// ── Integration tests (Postgres via testcontainers) ─────────────────────────

// seedLandscapeTopic creates a persistent topic with the given lifecycle fields
// and a 1-dim embedding (test-DB vector column accepts it).
func seedLandscapeTopic(t *testing.T, db *gorm.DB, boardID uint, label, status string, hitCount, cons int, lastSeen time.Time) BoardPersistentTopic {
	t.Helper()
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           label,
		Embedding:       FloatsToPgVector([]float64{0}),
		Status:          status,
		Source:          TopicSourceAuto,
		FirstSeenDate:   NormalizeReportDate(lastSeen),
		LastSeenDate:    NormalizeReportDate(lastSeen),
		HitCount:        hitCount,
		ConsecutiveHits: cons,
	}
	require.NoError(t, db.Create(&topic).Error)
	return topic
}

// assignSection links a section to a topic (mirrors the assignment write that
// production performs inside SaveReport).
func assignSection(t *testing.T, db *gorm.DB, sectionID, topicID uint) {
	t.Helper()
	require.NoError(t, db.Model(&DailyReportSection{}).
		Where("id = ?", sectionID).
		Update("persistent_topic_id", topicID).Error)
}

func TestGetBoardTopicLandscape_MultiStance_EmptyDayFill_Filtering(t *testing.T) {
	db := setupLandscapeTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := time.Now()

	// Reports: today and 3 days ago (days -1 and -2 deliberately absent so the
	// axis has zero-filled gap days).
	reportToday := seedTestReport(t, db, boardID, now)
	report3 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -3))

	// T1 active+fresh: section today and 3 days ago.
	t1 := seedLandscapeTopic(t, db, boardID, "芯片战", TopicStatusActive, 47, 22, now)
	assignSection(t, db, seedTestSection(t, db, reportToday, "chips-today"), t1.ID)
	assignSection(t, db, seedTestSection(t, db, report3, "chips-3d"), t1.ID)

	// T2 active but zero consecutive → stalled (last seen today so recency
	// alone is not enough; consecutive_hits==0 forces stalled).
	t2 := seedLandscapeTopic(t, db, boardID, "停滞话题", TopicStatusActive, 9, 0, now)
	assignSection(t, db, seedTestSection(t, db, reportToday, "stall-today"), t2.ID)

	// T3 candidate at threshold → pending.
	seedLandscapeTopic(t, db, boardID, "待激活", TopicStatusCandidate, 3, 0, now)

	// T4 archived.
	seedLandscapeTopic(t, db, boardID, "已归档", TopicStatusArchived, 12, 0, now.AddDate(0, 0, -40))

	// T5 candidate below threshold → emerging (🌱). Unlike the /topics filter,
	// the landscape keeps sub-threshold candidates that have been hit >= 1.
	seedLandscapeTopic(t, db, boardID, "观察中", TopicStatusCandidate, 1, 0, now)

	// T6 candidate with zero hits (pure orphan) → filtered out by
	// filterLandscapeVisible (hit == 0 is the only thing the landscape drops).
	// The model is gorm:"not null;default:1", so Create stored hit_count=1;
	// force the true orphan state the way a real anomaly row would look.
	t6 := seedLandscapeTopic(t, db, boardID, "幽灵苗头", TopicStatusCandidate, 0, 0, now)
	require.NoError(t, db.Model(&BoardPersistentTopic{}).
		Where("id = ?", t6.ID).
		Update("hit_count", 0).Error)

	resp, err := repo.GetBoardTopicLandscape(boardID, 7)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 5 visible topics: the emerging candidate (T5, hit=1) is now kept; the
	// pure orphan (T6, hit=0) is still dropped.
	require.Len(t, resp.Topics, 5)
	byLabel := make(map[string]TopicLandscapeTopic, len(resp.Topics))
	for _, tp := range resp.Topics {
		byLabel[tp.Label] = tp
	}
	assert.Contains(t, byLabel, "芯片战")
	assert.Contains(t, byLabel, "停滞话题")
	assert.Contains(t, byLabel, "待激活")
	assert.Contains(t, byLabel, "已归档")
	assert.Contains(t, byLabel, "观察中")
	assert.NotContains(t, byLabel, "幽灵苗头")

	// Stances.
	assert.Equal(t, StanceActive, byLabel["芯片战"].Stance)
	assert.Equal(t, StanceStalled, byLabel["停滞话题"].Stance)
	assert.Equal(t, StancePending, byLabel["待激活"].Stance, "candidate at threshold stays pending (no regression)")
	assert.Equal(t, StanceArchived, byLabel["已归档"].Stance)
	assert.Equal(t, StanceEmerging, byLabel["观察中"].Stance, "sub-threshold candidate surfaces as emerging")

	// can_activate only true for pending (emerging must NOT be activatable).
	assert.True(t, byLabel["待激活"].CanActivate)
	assert.False(t, byLabel["观察中"].CanActivate, "emerging is not pending → cannot activate")
	assert.False(t, byLabel["芯片战"].CanActivate)
	assert.False(t, byLabel["已归档"].CanActivate)

	// days_since_last for the active topic (seen today) is 0.
	assert.Equal(t, 0, byLabel["芯片战"].DaysSinceLast)

	// T1 lifeline: 8 points (days=7), today and -3 non-zero, gap days 0.
	chips := byLabel["芯片战"]
	require.Len(t, chips.Lifeline, 8)
	dayToCount := make(map[string]int, len(chips.Lifeline))
	for _, p := range chips.Lifeline {
		dayToCount[p.Date] = p.SectionCount
	}
	todayStr := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
	day3Str := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -3).Format("2006-01-02")
	day1Str := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -1).Format("2006-01-02")
	assert.Equal(t, 1, dayToCount[todayStr], "section today counted")
	assert.Equal(t, 1, dayToCount[day3Str], "section 3 days ago counted")
	assert.Equal(t, 0, dayToCount[day1Str], "gap day zero-filled")

	// Vitality.
	assert.Equal(t, 7, resp.Vitality.Days)
	require.Len(t, resp.Vitality.Trend, 8)
	assert.Equal(t, 1, resp.Vitality.ActiveTopicCount, "only T1 is active")
	// 3 sections total in window (chips x2 + stall x1); T3/T4 have no sections.
	assert.Equal(t, 3, resp.Vitality.SectionCount)
	assert.Nil(t, resp.Vitality.FeedActive, "MVP feed_active is null")
}

func TestGetBoardTopicLandscape_EmptyBoardReturnsEmptyArrays(t *testing.T) {
	db := setupLandscapeTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	// No reports, no topics.

	resp, err := repo.GetBoardTopicLandscape(boardID, 30)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Topics, 0, "topics is empty slice, not nil")
	require.Len(t, resp.Vitality.Trend, 0, "trend is empty slice, not nil")
	assert.Equal(t, 30, resp.Vitality.Days)
	assert.Nil(t, resp.Vitality.FeedActive)
}

func TestGetBoardTopicLandscape_DaysClampedToDefaultWhenNonPositive(t *testing.T) {
	db := setupLandscapeTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)

	// days=0 → default 30 → empty-board payload still echoes the clamped days.
	resp, err := repo.GetBoardTopicLandscape(boardID, 0)
	require.NoError(t, err)
	assert.Equal(t, topicLandscapeDefaultDays, resp.Vitality.Days)
}

// setupLandscapeTestDB mirrors the package's standard integration-test bootstrap
// (testutil.SetupTestDB) but is local so these tests stay self-contained.
func setupLandscapeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.SetupTestDB(t)
}
