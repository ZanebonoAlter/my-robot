package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// IsDemoReadOnly reports whether the server is running in the public, read-only
// demo mode (activated via DEMO_READ_ONLY=1). When false, ReadOnly is a no-op.
func IsDemoReadOnly() bool {
	return strings.TrimSpace(os.Getenv("DEMO_READ_ONLY")) == "1"
}

// ReadOnly returns a Gin middleware that enforces a read-only surface for the
// public demo instance. When DEMO_READ_ONLY is unset (production), it passes
// every request through unchanged.
//
// In demo mode it:
//   - allows CORS preflight (OPTIONS),
//   - allows GET (the only method needed to browse the product),
//   - rejects every other method with 405,
//   - additionally rejects two GET SSE endpoints that trigger background work
//     (tag merge scan/evaluate streams).
func ReadOnly() gin.HandlerFunc {
	if !IsDemoReadOnly() {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// Allow CORS preflight so browsers can fetch GET endpoints cross-origin.
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// Non-GET methods mutate state or trigger workloads; block them.
		if c.Request.Method != http.MethodGet {
			c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
				"error": "read-only demo",
			})
			return
		}

		// A handful of GET endpoints stream long-running side effects (embedding
		// generation + LLM calls). Block them explicitly.
		path := c.Request.URL.Path
		if strings.HasSuffix(path, "/merge-preview/scan/stream") ||
			strings.HasSuffix(path, "/merge-preview/evaluate/stream") {
			c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
				"error": "read-only demo",
			})
			return
		}

		c.Next()
	}
}
