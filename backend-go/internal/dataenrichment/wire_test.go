package dataenrichment

import (
	"testing"

	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/platform/database"
)

// TestInitEnablesRegisterRoutes is a regression test for a production startup
// panic: handler.RegisterRoutes dereferences a package-level handler singleton
// and panics with "handler.InitHandler must be called before RegisterRoutes"
// when that singleton is nil.
//
// Root cause that this test guards against: the wiring (InitHandler + the
// service construction) was originally placed inside app.StartRuntime, which
// runs AFTER app.SetupRoutes. SetupRoutes calls dataenrichment.RegisterRoutes,
// so the singleton was still nil at registration time and the server panicked
// on boot — yet every unit test passed because handler tests build a standalone
// handler via NewHandler and never exercise the singleton path.
//
// Contract: dataenrichment.Init MUST wire the handler singleton so that
// RegisterRoutes is safe to call during app.SetupRoutes. Init runs in main.go
// before SetupRoutes, mirroring how the other domains call InitRepository.
func TestInitEnablesRegisterRoutes(t *testing.T) {
	setupDataEnrichmentTestDB(t)
	Init(database.DB) // wires repo + cycle-A/B services + handler singleton

	gin.SetMode(gin.TestMode)
	g := gin.New()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterRoutes panicked after Init (regression: handler "+
				"singleton must be wired before route registration, see main.go "+
				"startup order): %v", r)
		}
	}()
	RegisterRoutes(g.Group("/api"))
}
