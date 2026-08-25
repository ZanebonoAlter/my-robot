package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/topicgraph/repository"
)

func setupWatchHandlerTest(t *testing.T) (*gin.Engine, *repository.TopicGraphRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared&_foreign_keys=on", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	database.DB = db
	repository.InitRepository(db)
	require.NoError(t, db.AutoMigrate(&repository.BoardTopicWatch{}, &repository.TopicWatchHit{}), "auto-migrate watch tables")

	r := gin.New()
	api := r.Group("/api")
	RegisterTopicWatchRoutes(api)
	return r, repository.Repo
}

func TestCreateTopicWatch_Handler(t *testing.T) {
	r, _ := setupWatchHandlerTest(t)

	body := map[string]string{"label": "美伊会不会真打起来"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "美伊会不会真打起来", data["label"])
	assert.Equal(t, "active", data["status"])
}

func TestCreateTopicWatch_MissingLabel(t *testing.T) {
	r, _ := setupWatchHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches",
		bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
}

func TestListTopicWatches_Handler(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	// Create some watches
	_, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "watch-a", Type: ""})
	require.NoError(t, err)
	_, err = repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "watch-b", Type: ""})
	require.NoError(t, err)
	_, err = repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 2, Label: "watch-c", Type: ""})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/semantic-boards/1/topic-watches", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestUpdateTopicWatch_Handler(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	watch, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "original", Type: ""})
	require.NoError(t, err)

	// Update label
	body := map[string]string{"label": "updated label"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/topic-watches/%d", watch.ID), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "updated label", data["label"])
	assert.Equal(t, "active", data["status"])

	// Update status
	body2 := map[string]string{"status": "paused"}
	jsonBody2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/topic-watches/%d", watch.ID), bytes.NewReader(jsonBody2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	data2 := resp2["data"].(map[string]interface{})
	assert.Equal(t, "paused", data2["status"])
}

func TestUpdateTopicWatch_InvalidStatus(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	watch, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "test", Type: ""})
	require.NoError(t, err)

	body := map[string]string{"status": "candidate"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/topic-watches/%d", watch.ID), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteTopicWatch_Handler(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	watch, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "to delete", Type: ""})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/topic-watches/%d", watch.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])

	// Verify deleted
	var found repository.BoardTopicWatch
	err = repo.DB().First(&found, watch.ID).Error
	assert.Error(t, err)
}

func TestGetWatchHits_Handler(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	// Create a watch and some hits
	watch, err := repo.CreateWatch(repository.CreateWatchInput{SemanticBoardID: 1, Label: "test watch", Type: ""})
	require.NoError(t, err)

	repo.DB().Create(&repository.TopicWatchHit{WatchID: watch.ID, SectionID: 100, ReportID: 200, Reason: "hit 1"})
	repo.DB().Create(&repository.TopicWatchHit{WatchID: watch.ID, SectionID: 101, ReportID: 200, Reason: "hit 2"})

	req := httptest.NewRequest(http.MethodGet, "/api/daily-reports/200/watch-hits", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestGetWatchHits_EmptyReport(t *testing.T) {
	r, _ := setupWatchHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/daily-reports/999/watch-hits", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].([]interface{})
	assert.Empty(t, data)
}

// ── watch-keyword-and-quickadd: type param + keyword validation ──────────────

func TestCreateTopicWatch_KeywordType(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	body := map[string]string{"label": "ASML|镓锗 出口", "type": "keyword"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "ASML|镓锗 出口", data["label"])
	assert.Equal(t, "keyword", data["type"], "watch JSON carries the type field")
	assert.Equal(t, "active", data["status"])
	// Frozen API contract: keyword creations carry instant_hit_count inside
	// data, even when nothing matched (the frontend normalizes data only).
	assert.Equal(t, float64(0), data["instant_hit_count"], "no historical sections → zero instant hits")

	var found repository.BoardTopicWatch
	require.NoError(t, repo.DB().First(&found, "label = ?", "ASML|镓锗 出口").Error)
	assert.Equal(t, repository.WatchTypeKeyword, found.Type)
}

func TestCreateTopicWatch_TypeDefaultsToLabel(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	// Old-client payload without type.
	body := map[string]string{"label": "美伊会不会真打起来"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "label", data["type"], "omitted type defaults to label")
	assert.NotContains(t, resp, "instant_hit_count", "label creations never carry instant_hit_count")

	var found repository.BoardTopicWatch
	require.NoError(t, repo.DB().First(&found, "label = ?", "美伊会不会真打起来").Error)
	assert.Equal(t, repository.WatchTypeLabel, found.Type)

	// Explicit type=label behaves the same.
	body2 := map[string]string{"label": "explicit label watch", "type": "label"}
	jsonBody2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	data2 := resp2["data"].(map[string]interface{})
	assert.Equal(t, "label", data2["type"])
}

func TestCreateTopicWatch_InvalidKeywordExprRejected(t *testing.T) {
	r, repo := setupWatchHandlerTest(t)

	cases := []struct {
		name  string
		label string
	}{
		{"trailing separator", "ASML|"},
		{"pure whitespace fullwidth+tab", "　\t "},
		{"empty string", ""},
		{"bare separator", "|"},
		{"consecutive separators", "||"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"label": tc.label, "type": "keyword"}
			jsonBody, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, false, resp["success"])
		})
	}

	var count int64
	repo.DB().Model(&repository.BoardTopicWatch{}).Where("type = ?", "keyword").Count(&count)
	assert.Zero(t, count, "no keyword watch row may be created from invalid expressions")
}

func TestCreateTopicWatch_InvalidTypeRejected(t *testing.T) {
	r, _ := setupWatchHandlerTest(t)

	body := map[string]string{"label": "x", "type": "regex"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTopicWatch_KeywordInstantMatchErrorSwallowed(t *testing.T) {
	// The instant match must never block creation: even when its internals
	// fail (here: boards/sections tables dropped so the scan errors), the
	// watch row survives and creation returns 200 with instant_hit_count=0.
	r, repo := setupWatchHandlerTest(t)
	require.NoError(t, repo.DB().Migrator().DropTable(&repository.BoardDailyReport{}, &repository.DailyReportSection{}, &repository.DailyReportThread{}))

	body := map[string]string{"label": "ASML", "type": "keyword"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/semantic-boards/1/topic-watches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "instant-match failure must not block watch creation")
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["instant_hit_count"])

	var count int64
	repo.DB().Model(&repository.BoardTopicWatch{}).Where("type = ?", repository.WatchTypeKeyword).Count(&count)
	assert.Equal(t, int64(1), count, "the keyword watch row must exist despite instant-match error")
}
