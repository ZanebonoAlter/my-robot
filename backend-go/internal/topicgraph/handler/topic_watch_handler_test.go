package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
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
	_, err := repo.CreateWatch(1, "watch-a")
	require.NoError(t, err)
	_, err = repo.CreateWatch(1, "watch-b")
	require.NoError(t, err)
	_, err = repo.CreateWatch(2, "watch-c")
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

	watch, err := repo.CreateWatch(1, "original")
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

	watch, err := repo.CreateWatch(1, "test")
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

	watch, err := repo.CreateWatch(1, "to delete")
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
	watch, err := repo.CreateWatch(1, "test watch")
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
