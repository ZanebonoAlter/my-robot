package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ReadOnly())
	r.GET("/api/categories", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/api/topic-tags/merge-preview/scan/stream", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.POST("/api/categories", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	return r
}

func do(t *testing.T, r http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestReadOnly_PassthroughWhenInactive(t *testing.T) {
	// DEMO_READ_ONLY unset → production path, everything passes through.
	withEnv(t, "DEMO_READ_ONLY", "")
	r := newTestRouter()

	// POST must reach the handler in production.
	w := do(t, r, http.MethodPost, "/api/categories")
	if w.Code != 200 {
		t.Fatalf("production POST should pass through, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReadOnly_AllowsGetInDemoMode(t *testing.T) {
	withEnv(t, "DEMO_READ_ONLY", "1")
	r := newTestRouter()

	// GET reads must be allowed.
	w := do(t, r, http.MethodGet, "/api/categories")
	if w.Code != 200 {
		t.Fatalf("GET should be allowed in demo mode, got %d", w.Code)
	}
}

func TestReadOnly_BlocksNonGetInDemoMode(t *testing.T) {
	withEnv(t, "DEMO_READ_ONLY", "1")
	r := newTestRouter()

	w := do(t, r, http.MethodPost, "/api/categories")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST should be blocked in demo mode, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "read-only demo") {
		t.Fatalf("expected read-only demo error body, got %s", w.Body.String())
	}

	// PUT and DELETE likewise.
	for _, m := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		w := do(t, r, m, "/api/categories")
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s should be blocked in demo mode, got %d", m, w.Code)
		}
	}
}

func TestReadOnly_AllowsOptionsInDemoMode(t *testing.T) {
	// CORS preflight must pass even in demo mode so browsers can issue GETs.
	withEnv(t, "DEMO_READ_ONLY", "1")
	r := newTestRouter()

	w := do(t, r, http.MethodOptions, "/api/categories")
	if w.Code == http.StatusMethodNotAllowed {
		t.Fatalf("OPTIONS (CORS preflight) must not be blocked in demo mode, got %d", w.Code)
	}
}

func TestReadOnly_BlocksSideEffectingGetStreams(t *testing.T) {
	withEnv(t, "DEMO_READ_ONLY", "1")
	r := newTestRouter()

	for _, path := range []string{
		"/api/topic-tags/merge-preview/scan/stream",
		"/api/topic-tags/merge-preview/evaluate/stream",
	} {
		w := do(t, r, http.MethodGet, path)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("side-effecting GET %s should be blocked, got %d", path, w.Code)
		}
	}
}

func TestIsDemoReadOnly(t *testing.T) {
	withEnv(t, "DEMO_READ_ONLY", "")
	if IsDemoReadOnly() {
		t.Fatal("expected false when env unset")
	}
	withEnv(t, "DEMO_READ_ONLY", "1")
	if !IsDemoReadOnly() {
		t.Fatal("expected true when env=1")
	}
	withEnv(t, "DEMO_READ_ONLY", " 1 ")
	if !IsDemoReadOnly() {
		t.Fatal("expected true when env has surrounding whitespace")
	}
}
