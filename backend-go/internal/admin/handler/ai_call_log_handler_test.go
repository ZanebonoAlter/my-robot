package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
)

func setupCallLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AICallLog{}))
	database.DB = db
	repository.InitRepository(database.DB)
	return db
}

func TestListCallLogs_EmptyResult(t *testing.T) {
	setupCallLogTestDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?operation=nonexistent", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	require.Empty(t, data)
}

func TestListCallLogs_FilterByOperation(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()

	require.NoError(t, db.Create(&models.AICallLog{
		Operation:    "test.op1",
		Capability:   "summary",
		RouteName:    "default",
		ProviderName: "p1",
		Success:      true,
		LatencyMs:    100,
		CreatedAt:    now,
	}).Error)
	require.NoError(t, db.Create(&models.AICallLog{
		Operation:    "test.op2",
		Capability:   "summary",
		RouteName:    "default",
		ProviderName: "p1",
		Success:      true,
		LatencyMs:    200,
		CreatedAt:    now.Add(time.Second),
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?operation=test.op1", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	require.Equal(t, "test.op1", item["operation"])
}

func TestListCallLogs_FilterBySessionID_AscOrder(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()
	sessionID := "session-test-1"

	// Insert in reverse chronological order (3, 2, 1 seconds ago)
	// Query should return them in ascending created_at order (1, 2, 3)
	for i := 3; i > 0; i-- {
		require.NoError(t, db.Create(&models.AICallLog{
			Operation:    "test.op",
			SessionID:    sessionID,
			Capability:   "summary",
			RouteName:    "default",
			ProviderName: "p1",
			Success:      true,
			LatencyMs:    i * 100,
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}).Error)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?session_id=%s", sessionID), nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 3)

	// Verify ascending order by checking latency_ms (proportional to created_at)
	for i := 1; i < len(data); i++ {
		prev := data[i-1].(map[string]interface{})
		curr := data[i].(map[string]interface{})
		prevLatency := prev["latency_ms"].(float64)
		currLatency := curr["latency_ms"].(float64)
		require.True(t, prevLatency < currLatency,
			"expected ascending order by created_at, but latency_ms[%d]=%v >= latency_ms[%d]=%v",
			i-1, prevLatency, i, currLatency)
	}
}

func TestListCallLogs_DefaultLimitIs50(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()

	// Insert 55 records (exceeds default limit of 50)
	for i := 0; i < 55; i++ {
		require.NoError(t, db.Create(&models.AICallLog{
			Operation:    "test.op",
			Capability:   "summary",
			RouteName:    "default",
			ProviderName: "p1",
			Success:      true,
			LatencyMs:    i,
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}).Error)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 50)
}

func TestListCallLogs_LimitExceeds200_Truncated(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()

	// Insert 210 records (exceeds max limit of 200)
	for i := 0; i < 210; i++ {
		require.NoError(t, db.Create(&models.AICallLog{
			Operation:    "test.op",
			Capability:   "summary",
			RouteName:    "default",
			ProviderName: "p1",
			Success:      true,
			LatencyMs:    i,
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}).Error)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?limit=500", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 200)
}

func TestListCallLogs_DefaultDescOrder(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()

	// Insert records with increasing created_at
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Create(&models.AICallLog{
			Operation:    "test.op",
			Capability:   "summary",
			RouteName:    "default",
			ProviderName: "p1",
			Success:      true,
			LatencyMs:    i * 100,
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}).Error)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 5)

	// Verify descending order (most recent first)
	for i := 1; i < len(data); i++ {
		prev := data[i-1].(map[string]interface{})
		curr := data[i].(map[string]interface{})
		prevLatency := prev["latency_ms"].(float64)
		currLatency := curr["latency_ms"].(float64)
		require.True(t, prevLatency > currLatency,
			"expected descending order by created_at, but latency_ms[%d]=%v <= latency_ms[%d]=%v",
			i-1, prevLatency, i, currLatency)
	}
}

func TestListCallLogs_FilterByCapability(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()

	require.NoError(t, db.Create(&models.AICallLog{
		Operation:    "test.op",
		Capability:   "summary",
		RouteName:    "default",
		ProviderName: "p1",
		Success:      true,
		LatencyMs:    100,
		CreatedAt:    now,
	}).Error)
	require.NoError(t, db.Create(&models.AICallLog{
		Operation:    "test.op",
		Capability:   "embedding",
		RouteName:    "default",
		ProviderName: "p1",
		Success:      true,
		LatencyMs:    200,
		CreatedAt:    now.Add(time.Second),
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?capability=embedding", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	require.Equal(t, "embedding", item["capability"])
}

func TestListCallLogs_Offset(t *testing.T) {
	db := setupCallLogTestDB(t)
	now := time.Now()

	// Insert 10 records
	for i := 0; i < 10; i++ {
		require.NoError(t, db.Create(&models.AICallLog{
			Operation:    "test.op",
			Capability:   "summary",
			RouteName:    "default",
			ProviderName: "p1",
			Success:      true,
			LatencyMs:    i,
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}).Error)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?limit=5&offset=5", nil)

	ListCallLogs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data := body["data"].([]interface{})
	require.Len(t, data, 5)
}
