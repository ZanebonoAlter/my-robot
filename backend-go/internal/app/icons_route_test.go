package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// iconTestEngine registers the /icons route exactly as SetupRoutes does, backed
// by a temp directory, so route-level behavior is testable without a database.
func iconTestEngine(dir string) *gin.Engine {
	r := gin.New()
	r.GET("/icons/*filepath", iconsFileHandler(dir))
	return r
}

func TestIconsRoute_ServesFileWithSecurityHeaders(t *testing.T) {
	dir := t.TempDir()
	feedDir := filepath.Join(dir, "feeds")
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	iconBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(filepath.Join(feedDir, "42.png"), iconBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	r := iconTestEngine(dir)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/icons/feeds/42.png", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want sandbox", got)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if w.Body.String() != string(iconBytes) {
		t.Errorf("body = %q, want the stored icon bytes", w.Body.String())
	}
}

func TestIconsRoute_MissingFileReturns404(t *testing.T) {
	r := iconTestEngine(t.TempDir())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/icons/feeds/999.png", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// TestSpaFallback_ReleasesExactIconsPath locks in that the exact path /icons
// (not just /icons/...) passes through the SPA fallback instead of being
// swallowed by index.html — it must reach the /icons route (here: 404 since no
// route/file backs it).
func TestSpaFallback_ReleasesExactIconsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(spaFallback(dir))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/icons", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("status = %d, want pass-through (not SPA index.html)", w.Code)
	}
}
