package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
	"syntopica-backend/internal/topicgraph/repository"
)

// setupLandscapeHandlerTest wires a Postgres-backed DB, the global repository
// singleton, and the full daily-report route tree (incl. topic-landscape) on a
// fresh gin engine. Each test seeds through repository.Repo.DB().
func setupLandscapeHandlerTest(t *testing.T) *gin.Engine {
	t.Helper()
	db := testutil.SetupTestDB(t)
	repository.InitRepository(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	RegisterDailyReportRoutes(api)
	return r
}

func getLandscape(t *testing.T, r *gin.Engine, boardID, days string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/api/semantic-boards/" + boardID + "/topic-landscape"
	if days != "" {
		path += "?days=" + days
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeLandscape(t *testing.T, w *httptest.ResponseRecorder) (topics []map[string]any, vitality map[string]any) {
	t.Helper()
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Topics   []map[string]any `json:"topics"`
			Vitality map[string]any   `json:"vitality"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Data.Topics, body.Data.Vitality
}

func TestGetBoardTopicLandscape_InvalidBoardID(t *testing.T) {
	r := setupLandscapeHandlerTest(t)
	w := getLandscape(t, r, "abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body["success"].(bool))
	assert.Contains(t, body["error"], "invalid board id")
}

func TestGetBoardTopicLandscape_HTTPClampsDays(t *testing.T) {
	r := setupLandscapeHandlerTest(t)
	// An empty/non-existent board returns the empty-state payload, whose
	// vitality.days echoes the clamped value — so we can assert the clamp
	// without seeding any data.
	cases := []struct{ in, want string }{
		{"", "30"},
		{"0", "30"},
		{"-5", "30"},
		{"abc", "30"},
		{"7", "7"},
		{"14", "14"},
		{"30", "30"},
		{"90", "90"},
		{"10", "7"},  // nearest
		{"20", "14"}, // nearest
		{"60", "30"}, // tie → lower
		{"100", "90"},
	}
	for _, tc := range cases {
		t.Run("days="+tc.in, func(t *testing.T) {
			w := getLandscape(t, r, "99999", tc.in)
			require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
			_, vitality := decodeLandscape(t, w)
			daysF, ok := vitality["days"].(float64)
			require.True(t, ok, "vitality.days missing: %+v", vitality)
			assert.Equal(t, tc.want, itoa(int(daysF)))
		})
	}
}

func TestGetBoardTopicLandscape_EmptyBoard(t *testing.T) {
	r := setupLandscapeHandlerTest(t)
	w := getLandscape(t, r, "99999", "30")
	require.Equal(t, http.StatusOK, w.Code)
	topics, vitality := decodeLandscape(t, w)
	// topics and trend must be [] (empty arrays), not null.
	require.NotNil(t, topics)
	assert.Len(t, topics, 0)
	trend, ok := vitality["trend"].([]any)
	require.True(t, ok, "trend must be an array, got %T", vitality["trend"])
	assert.Len(t, trend, 0)
	assert.Nil(t, vitality["feed_active"])
}

func TestGetBoardTopicLandscape_StancesAndLifeline(t *testing.T) {
	r := setupLandscapeHandlerTest(t)
	db := repository.Repo.DB()

	boardID := seedHandlerBoard(t, db)
	now := time.Now()
	reportToday := seedHandlerReport(t, db, boardID, now)
	report3 := seedHandlerReport(t, db, boardID, now.AddDate(0, 0, -3))

	// active+fresh
	tActive := seedHandlerTopic(t, db, boardID, "芯片战", repository.TopicStatusActive, 47, 22, now)
	seedHandlerSectionAssign(t, db, reportToday, tActive.ID)
	seedHandlerSectionAssign(t, db, report3, tActive.ID)
	// candidate at threshold → pending
	seedHandlerTopic(t, db, boardID, "待激活", repository.TopicStatusCandidate, 3, 0, now)
	// archived
	seedHandlerTopic(t, db, boardID, "已归档", repository.TopicStatusArchived, 12, 0, now.AddDate(0, 0, -40))
	// candidate below threshold → emerging (🌱); the landscape keeps it.
	seedHandlerTopic(t, db, boardID, "观察中", repository.TopicStatusCandidate, 1, 0, now)

	w := getLandscape(t, r, itoa(int(boardID)), "7")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	topics, vitality := decodeLandscape(t, w)

	require.Len(t, topics, 4, "emerging candidate surfaces alongside the others")
	byLabel := map[string]map[string]any{}
	for _, tp := range topics {
		byLabel[tp["label"].(string)] = tp
	}
	assert.Equal(t, "active", byLabel["芯片战"]["stance"])
	assert.Equal(t, "pending", byLabel["待激活"]["stance"])
	assert.Equal(t, "emerging", byLabel["观察中"]["stance"])
	assert.Equal(t, "archived", byLabel["已归档"]["stance"])

	// active topic lifeline is a non-empty array with zero-filled gap days.
	chips := byLabel["芯片战"]["lifeline"].([]any)
	assert.Len(t, chips, 8, "days=7 → 8 axis points")
	assert.Equal(t, true, byLabel["待激活"]["can_activate"])
	assert.Equal(t, false, byLabel["芯片战"]["can_activate"])
	assert.Equal(t, false, byLabel["观察中"]["can_activate"], "emerging is not activatable")

	// vitality
	assert.EqualValues(t, 7, vitality["days"])
	assert.EqualValues(t, 1, vitality["active_topic_count"])
	trend := vitality["trend"].([]any)
	assert.Len(t, trend, 8)
}

// ── handler-test seed helpers (handler package can't see repository_test.go's
// unexported helpers, so local copies live here). ──

func seedHandlerBoard(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	board := models.SemanticLabel{
		Label:     "landscape-board",
		Slug:      "landscape-board",
		LabelType: "board",
		Status:    "active",
	}
	require.NoError(t, db.Create(&board).Error)
	return board.ID
}

func seedHandlerReport(t *testing.T, db *gorm.DB, boardID uint, date time.Time) uint {
	t.Helper()
	rpt := repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      repository.NormalizeReportDate(date),
		Title:           "Test Report",
		Status:          "completed",
	}
	require.NoError(t, db.Create(&rpt).Error)
	return rpt.ID
}

func seedHandlerTopic(t *testing.T, db *gorm.DB, boardID uint, label, status string, hit, cons int, lastSeen time.Time) repository.BoardPersistentTopic {
	t.Helper()
	tp := repository.BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           label,
		Embedding:       repository.FloatsToPgVector([]float64{0}),
		Status:          status,
		Source:          repository.TopicSourceAuto,
		FirstSeenDate:   repository.NormalizeReportDate(lastSeen),
		LastSeenDate:    repository.NormalizeReportDate(lastSeen),
		HitCount:        hit,
		ConsecutiveHits: cons,
	}
	require.NoError(t, db.Create(&tp).Error)
	return tp
}

func seedHandlerSectionAssign(t *testing.T, db *gorm.DB, reportID, topicID uint) {
	t.Helper()
	sec := repository.DailyReportSection{
		ReportID:     reportID,
		ClusterLabel: "section",
		ArticleCount: 1,
		Embedding:    repository.FloatsToPgVector([]float64{0}),
	}
	require.NoError(t, db.Create(&sec).Error)
	require.NoError(t, db.Model(&repository.DailyReportSection{}).
		Where("id = ?", sec.ID).
		Update("persistent_topic_id", topicID).Error)
}

// itoa is a tiny strconv.Itoa wrapper to avoid pulling strconv into the test
// file's import list just for URL building.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
