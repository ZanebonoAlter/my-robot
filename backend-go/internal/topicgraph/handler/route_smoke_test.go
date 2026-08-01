package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRoutesRegisterWithoutPanic guards against Gin radix-tree panics caused by
// conflicting param names under the same prefix — e.g. registering both
// "/semantic-boards/:id/..." and "/semantic-boards/:boardId/...". Gin's trie
// forbids two different param names at the same tree position and panics at
// registration time.
//
// Such a panic only surfaces at startup when the full route tree is built.
// Handler unit tests call the handler functions directly and never register
// routes, so without this smoke test a param-name mismatch compiles fine, passes
// all unit tests, and crashes the server the moment it boots. This was a real
// incident: manual-topic-lane initially registered :boardId alongside the
// existing :id routes (see the warning comment in RegisterTopicWatchRoutes).
func TestRoutesRegisterWithoutPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterDailyReportRoutes panicked — likely a route-param "+
				"conflict under a shared prefix (e.g. :id vs :boardId under "+
				"/semantic-boards/): %v", r)
		}
	}()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// RegisterDailyReportRoutes also calls RegisterTopicWatchRoutes, so this
	// exercises every route this package registers.
	RegisterDailyReportRoutes(engine.Group("/api"))
}
