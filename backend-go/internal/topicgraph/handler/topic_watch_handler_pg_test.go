package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/platform/testutil"
	"syntopica-backend/internal/topicgraph/repository"
)

func setupWatchHandlerPGTest(t *testing.T) (*gin.Engine, *repository.TopicGraphRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	repository.InitRepository(db)

	r := gin.New()
	api := r.Group("/api")
	RegisterTopicWatchRoutes(api)
	return r, repository.Repo
}

func TestGetWatchHits_HandlerPGReturnsOnlyActiveJoinedWatchData(t *testing.T) {
	r, repo := setupWatchHandlerPGTest(t)
	now := repository.NormalizeReportDate(time.Now())

	active, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "ASML", Type: repository.WatchTypeKeyword})
	require.NoError(t, err)
	paused, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "paused", Type: repository.WatchTypeLabel})
	require.NoError(t, err)
	pausedStatus := repository.WatchStatusPaused
	_, err = repo.UpdateWatch(paused.ID, nil, nil, &pausedStatus)
	require.NoError(t, err)

	for _, hit := range []repository.TopicWatchHit{
		{WatchID: active.ID, SectionID: 11, ReportID: 700, PeriodDate: now, Reason: "active"},
		{WatchID: paused.ID, SectionID: 12, ReportID: 700, PeriodDate: now, Reason: "paused"},
	} {
		require.NoError(t, repo.DB().Create(&hit).Error)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/daily-reports/700/watch-hits", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "ASML", resp.Data[0]["watch_label"])
	assert.Equal(t, repository.WatchTypeKeyword, resp.Data[0]["watch_type"])
	assert.Equal(t, float64(active.ID), resp.Data[0]["watch_id"])
}

// ── watch-materialized-topic: handler validation & delete-confirmation ──

// TestCreateTopicWatch_MaterializedTypes verifies the four-type creation
// contract: keyword_topic validates DNF, sentence_topic persists query,
// invalid type rejected.
func TestCreateTopicWatch_MaterializedTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	r, repo := setupWatchHandlerPGTest(t)

	doPost := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// keyword_topic with valid expression ⇒ created.
	w := doPost(`{"label":"harness","type":"keyword_topic"}`)
	require.Equal(t, http.StatusOK, w.Code)
	// keyword_topic with trailing '|' ⇒ rejected (invalid DNF).
	w = doPost(`{"label":"harness|","type":"keyword_topic"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	// sentence_topic with query ⇒ created with query persisted.
	w = doPost(`{"label":"AI 进展","type":"sentence_topic","query":"AI coding assistant 进展"}`)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data repository.BoardTopicWatch `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, repository.WatchTypeSentenceTopic, resp.Data.Type)
	assert.Equal(t, "AI coding assistant 进展", resp.Data.Query)
	assert.Equal(t, repository.WatchStatusActive, resp.Data.Status)
	// unknown type ⇒ rejected.
	w = doPost(`{"label":"x","type":"sentence"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Cleanup.
	watches, _ := repo.ListWatchesByBoard(1)
	for _, watch := range watches {
		_ = repo.DeleteWatch(watch.ID)
	}
}

// TestDeleteTopicWatch_SentenceRequiresConfirmation verifies the delete
// flow: sentence_topic without confirm ⇒ 400 with the topic label spelled
// out; with confirm_archive_topic=true ⇒ topic archived + watch deleted.
func TestDeleteTopicWatch_SentenceRequiresConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	r, repo := setupWatchHandlerPGTest(t)

	// sentence watch + its dedicated topic (simulating post-materialization).
	watch, err := repo.CreateWatch(repository.CreateWatchInput{
		SemanticBoardID: 1, Label: "AI 编程工具进展", Type: repository.WatchTypeSentenceTopic, Query: "AI 进展",
	})
	require.NoError(t, err)
	today := repository.NormalizeReportDate(time.Now())
	topicID, err := repo.CreateWatchTopic(1, "AI 编程工具进展", "[1,0,0]", today)
	require.NoError(t, err)
	require.NoError(t, repo.SetWatchPersistentTopic(watch.ID, topicID))

	doDelete := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/api/topic-watches/"+strconv.Itoa(int(watch.ID))+query, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// ① no confirm ⇒ 400, error names the topic.
	w := doDelete("")
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "AI 编程工具进展")
	// watch still alive.
	_, err = repo.GetWatchByID(watch.ID)
	require.NoError(t, err)

	// ② confirmed ⇒ topic archived + watch gone.
	w = doDelete("?confirm_archive_topic=true")
	require.Equal(t, http.StatusOK, w.Code)
	_, err = repo.GetWatchByID(watch.ID)
	assert.Error(t, err, "watch deleted after confirmation")
	var topic repository.BoardPersistentTopic
	require.NoError(t, repo.DB().First(&topic, topicID).Error)
	assert.Equal(t, repository.TopicStatusArchived, topic.Status, "dedicated topic archived with the watch")

	// keyword_topic deletes freely (no confirmation, no topic).
	kw, err := repo.CreateWatch(repository.CreateWatchInput{
		SemanticBoardID: 1, Label: "harness", Type: repository.WatchTypeKeywordTopic,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodDelete, "/api/topic-watches/"+strconv.Itoa(int(kw.ID)), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
