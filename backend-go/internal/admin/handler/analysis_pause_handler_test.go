package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
)

// setupAnalysisPauseTestDB mirrors setupAIAdminTestDB (sqlite in-memory +
// database.DB swap). ai_settings is what the handler itself needs; the AI
// route tables exist so the resume-triggered RunStartupProbe goroutine sees an
// empty route list and finishes immediately instead of retrying ListRoutes
// for ~4s while holding the probe lock (which would make the later
// resume-probe test's TryLock skip).
func setupAnalysisPauseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.AISettings{}, &models.AIProvider{}, &models.AIRoute{},
		&models.AIRouteProvider{}, &models.AICallLog{},
	))
	database.DB = db
	return db
}

// newAnalysisPauseGinContext builds a gin test context the same way the
// existing handler tests do (gin.CreateTestContext + httptest.NewRecorder).
func newAnalysisPauseGinContext(t *testing.T, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
	return ctx, recorder
}

func decodeAnalysisPauseBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

// TestGetAnalysisPause covers GET: default state is paused=false with an empty
// paused_at string.
func TestGetAnalysisPause(t *testing.T) {
	setupAnalysisPauseTestDB(t)

	ctx, recorder := newAnalysisPauseGinContext(t, http.MethodGet, "")
	GetAnalysisPause(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeAnalysisPauseBody(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "response should carry a data object")
	require.Equal(t, false, data["paused"])
	require.Equal(t, "", data["paused_at"])
}

// TestSetAnalysisPause_Engage covers POST {paused:true}: the pause is engaged
// and the response reports paused=true, a stamped paused_at and the "分析已暂停"
// message.
func TestSetAnalysisPause_Engage(t *testing.T) {
	setupAnalysisPauseTestDB(t)

	ctx, recorder := newAnalysisPauseGinContext(t, http.MethodPost, `{"paused":true}`)
	SetAnalysisPause(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeAnalysisPauseBody(t, recorder)
	require.Equal(t, true, body["success"])
	require.Equal(t, "分析已暂停", body["message"])
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "response should carry a data object")
	require.Equal(t, true, data["paused"])
	require.NotEmpty(t, data["paused_at"])
}

// TestSetAnalysisPause_Release covers POST {paused:false}: the pause is
// released, paused_at cleared, and the response reports the "分析已恢复" message.
func TestSetAnalysisPause_Release(t *testing.T) {
	setupAnalysisPauseTestDB(t)

	ctx, recorder := newAnalysisPauseGinContext(t, http.MethodPost, `{"paused":false}`)
	SetAnalysisPause(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeAnalysisPauseBody(t, recorder)
	require.Equal(t, true, body["success"])
	require.Equal(t, "分析已恢复", body["message"])
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "response should carry a data object")
	require.Equal(t, false, data["paused"])
	require.Equal(t, "", data["paused_at"])
}

// TestSetAnalysisPause_BadRequest covers the bind failure path: a body without
// a valid {paused} field returns 400 with success=false.
func TestSetAnalysisPause_BadRequest(t *testing.T) {
	setupAnalysisPauseTestDB(t)

	ctx, recorder := newAnalysisPauseGinContext(t, http.MethodPost, "")
	SetAnalysisPause(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	body := decodeAnalysisPauseBody(t, recorder)
	require.Equal(t, false, body["success"])
	require.NotEmpty(t, body["error"])
}
