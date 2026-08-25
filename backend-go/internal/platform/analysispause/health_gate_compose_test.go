// Package analysispause_test contains the end-to-end compose proof for the AI
// model health gate (openspec change ai-model-health-gate, task 3.6):
//
//	probe 通过 ⇒ aihealth.Healthy()==true ⇒ analysispause.IsPaused()==false ⇒
//	scheduler.PauseAware 放行（job 实际执行）
//	probe 失败 ⇒ aihealth.Healthy()==false ⇒ analysispause.IsPaused()==true ⇒
//	scheduler.PauseAware 跳过（job 不跑，摘要含 model_unhealthy）
//
// It lives in the external test package so the test binary can import
// aihealth/airouter/scheduler without creating an import cycle with
// analysispause's production import of aihealth.
//
// NOTE on the probe seam: aihealth.probeFn is an unexported package variable
// with no exported setter, so an external test package cannot swap in a fake
// probe. Instead these tests drive RunStartupProbe through its REAL default
// probe (airouter.TestConnection) against local httptest mock provider
// endpoints — exactly the "mock provider 端点" wording of task 3.6 — which
// exercises TestConnection itself and requires no product-code change.
package analysispause_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/admin/scheduler"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/aihealth"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/analysispause"
	"syntopica-backend/internal/platform/testutil"
)

// resetHealthForTest forces the process-global aihealth snapshot back to the
// not-ready state and restores it on cleanup, so the snapshot never leaks into
// a sibling test (tests in this package and in gate_test.go all share the
// process-global).
func resetHealthForTest(t *testing.T) {
	t.Helper()
	aihealth.SetSnapshotForTest(aihealth.Snapshot{})
	t.Cleanup(func() { aihealth.SetSnapshotForTest(aihealth.Snapshot{}) })
}

// mockProviderServer returns an httptest server answering GET /models with a
// valid OpenAI-style model list — enough for airouter.TestConnection (the real
// probe) to report reachable.
func mockProviderServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// closedProviderURL returns a base URL that refuses connections: a server is
// started and immediately closed, so the port is guaranteed not listening and
// TestConnection fails fast with connection refused.
func closedProviderURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// seedEnabledRoute upserts an enabled provider of the given model_kind bound as
// the primary (priority 1) link of an enabled route with the given capability.
// UpsertProvider/UpsertRoute are the production paths, so link priority and the
// model_kind binding check are exercised for real.
func seedEnabledRoute(t *testing.T, store *airouter.Store, capability, providerName, baseURL, modelKind string) {
	t.Helper()
	p := models.AIProvider{
		Name:         providerName,
		ProviderType: airouter.ProviderTypeOpenAICompatible,
		BaseURL:      baseURL,
		Model:        providerName + "-model",
		APIKey:       "test-key",
		Enabled:      true,
		ModelKind:    modelKind,
	}
	require.NoError(t, store.UpsertProvider(&p))
	require.NoError(t, store.UpsertRoute(&models.AIRoute{
		Name:       "default",
		Capability: capability,
		Enabled:    true,
	}, []uint{p.ID}))
}

// TestHealthGate_ProbeComposesWithPauseGate is the happy-path compose proof:
// the startup race (snapshot not ready) pauses analysis; after RunStartupProbe
// finds both an embedding and an llm route reachable (real HTTP against local
// mock endpoints), the gate opens and PauseAware passes the job through.
func TestHealthGate_ProbeComposesWithPauseGate(t *testing.T) {
	resetHealthForTest(t)
	db := testutil.SetupTestDB(t)
	store := airouter.NewStore(db)

	// analysis_paused is not seeded, so the user switch defaults to false.
	require.False(t, analysispause.UserPaused())

	seedEnabledRoute(t, store, string(airouter.CapabilityEmbedding), "emb-test", mockProviderServer(t).URL, "embedding")
	seedEnabledRoute(t, store, string(airouter.CapabilitySummary), "llm-test", mockProviderServer(t).URL, "llm")

	// Startup race: before the first probe completes the snapshot is not ready,
	// so Healthy()==false and the effective pause is ON (fail-closed).
	require.False(t, aihealth.Healthy())
	require.True(t, analysispause.IsPaused())

	// Probe both providers (both reachable) and let the verdict land in the
	// in-memory snapshot.
	aihealth.RunStartupProbe(context.Background(), store, false)

	require.True(t, aihealth.Healthy())
	require.False(t, analysispause.IsPaused())
	require.Equal(t, "", analysispause.PauseReason())

	// PauseAware must run the wrapped job unchanged.
	var ran int32
	result, err := scheduler.PauseAware(func(ctx context.Context) (*scheduler.JobResult, error) {
		atomic.AddInt32(&ran, 1)
		return &scheduler.JobResult{Summary: "real job ran"}, nil
	})(context.Background())

	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&ran), "job must run when the health gate is open")
	require.Equal(t, "real job ran", result.Summary)
}

// TestHealthGate_ProbeUnhealthy_PauseAwareSkips is the closed-gate compose
// proof: with both providers unreachable, RunStartupProbe marks the snapshot
// unhealthy, IsPaused flips to true with reason model_unhealthy, and
// PauseAware skips the wrapped job.
func TestHealthGate_ProbeUnhealthy_PauseAwareSkips(t *testing.T) {
	resetHealthForTest(t)
	db := testutil.SetupTestDB(t)
	store := airouter.NewStore(db)

	require.False(t, analysispause.UserPaused())

	unreachable := closedProviderURL(t)
	seedEnabledRoute(t, store, string(airouter.CapabilityEmbedding), "emb-test", unreachable, "embedding")
	seedEnabledRoute(t, store, string(airouter.CapabilitySummary), "llm-test", unreachable, "llm")

	aihealth.RunStartupProbe(context.Background(), store, false)

	require.False(t, aihealth.Healthy())
	require.True(t, analysispause.IsPaused())
	require.Equal(t, "model_unhealthy", analysispause.PauseReason())

	// PauseAware must skip the wrapped job: it never runs, the result is a
	// benign success whose summary carries model_unhealthy.
	var ran int32
	result, err := scheduler.PauseAware(func(ctx context.Context) (*scheduler.JobResult, error) {
		atomic.AddInt32(&ran, 1)
		return &scheduler.JobResult{Summary: "real job ran"}, nil
	})(context.Background())

	require.NoError(t, err, "skipped result must be a success (err=nil)")
	require.EqualValues(t, 0, atomic.LoadInt32(&ran), "real job must NOT run while the health gate is closed")
	require.NotNil(t, result)
	require.Contains(t, result.Summary, "skipped")
	require.Contains(t, result.Summary, "model_unhealthy", "summary should flag the health reason")
	require.Equal(t, "paused", result.Data["skipped"])
}

// TestHealthGate_EmbeddingUp_LLMDown_NotHealthy pins the lenient-health
// boundary in the compose chain: embedding reachable alone is NOT healthy
// (缺 llm → Healthy=false), so the pause gate stays closed.
func TestHealthGate_EmbeddingUp_LLMDown_NotHealthy(t *testing.T) {
	resetHealthForTest(t)
	db := testutil.SetupTestDB(t)
	store := airouter.NewStore(db)

	require.False(t, analysispause.UserPaused())

	seedEnabledRoute(t, store, string(airouter.CapabilityEmbedding), "emb-test", mockProviderServer(t).URL, "embedding")
	seedEnabledRoute(t, store, string(airouter.CapabilitySummary), "llm-test", closedProviderURL(t), "llm")

	aihealth.RunStartupProbe(context.Background(), store, false)

	snap := aihealth.GetSnapshot()
	require.False(t, snap.Healthy, "missing a reachable llm route -> not healthy")
	require.False(t, aihealth.Healthy())
	require.True(t, analysispause.IsPaused())

	var ran int32
	result, err := scheduler.PauseAware(func(ctx context.Context) (*scheduler.JobResult, error) {
		atomic.AddInt32(&ran, 1)
		return &scheduler.JobResult{Summary: "real job ran"}, nil
	})(context.Background())

	require.NoError(t, err, "skipped result must be a success (err=nil)")
	require.EqualValues(t, 0, atomic.LoadInt32(&ran), "real job must NOT run while the health gate is closed")
	require.Contains(t, result.Summary, "model_unhealthy")
	require.Equal(t, "paused", result.Data["skipped"])
}
