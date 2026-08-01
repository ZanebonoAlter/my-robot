package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// embedQuery 的 HTTP 契约测试（task 3.10）。
// embedText（包级变量）默认调真实 airouter，这里整体替换为 mock，
// 覆盖空 query 拒绝 / body 解析失败 / 嵌入成功 / 嵌入失败降级 四个分支。
// 路由注册本身由 route_smoke_test.go 的 TestRoutesRegisterWithoutPanic 守护。

func newEmbedQueryEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/embed-query", embedQuery)
	return r
}

func postEmbedQuery(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/embed-query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newEmbedQueryEngine().ServeHTTP(w, req)
	return w
}

func TestEmbedQuery_RejectsEmptyOrBlankQuery(t *testing.T) {
	for _, body := range []string{`{"query":""}`, `{"query":"   "}`, `{}`} {
		w := postEmbedQuery(t, body)
		require.Equalf(t, http.StatusBadRequest, w.Code, "body %q should be rejected", body)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp["success"].(bool))
		assert.Contains(t, resp["error"], "query is required")
	}
}

func TestEmbedQuery_RejectsInvalidBody(t *testing.T) {
	w := postEmbedQuery(t, `not json`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEmbedQuery_ReturnsEmbedding(t *testing.T) {
	orig := embedText
	defer func() { embedText = orig }()
	embedText = func(ctx context.Context, q string) ([]float64, error) {
		assert.Equal(t, "半导体出口管制", q)
		return []float64{0.1, 0.2, 0.3}, nil
	}

	w := postEmbedQuery(t, `{"query":"半导体出口管制"}`)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, resp.Data.Embedding)
}

func TestEmbedQuery_FailureReturns500(t *testing.T) {
	orig := embedText
	defer func() { embedText = orig }()
	embedText = func(ctx context.Context, q string) ([]float64, error) {
		return nil, fmt.Errorf("model down")
	}

	w := postEmbedQuery(t, `{"query":"x"}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Contains(t, resp["error"], "failed to embed query")
}
