package airouter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// TestUpsertRouteModelKindValidation exercises the UpsertRoute model_kind
// binding check against a testcontainer Postgres. The check reads ai_providers
// (a real DB lookup), so per testing.md it must run on real PG, not SQLite.
// Docker required. Skipped under -short.
func TestUpsertRouteModelKindValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.SetupTestDB(t)
	store := NewStore(db)

	// One llm provider, one embedding provider.
	llmProv := models.AIProvider{Name: "mk-route-llm", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://llm.example/v1", Model: "gpt", ModelKind: "llm", Enabled: true, TimeoutSeconds: 120}
	embProv := models.AIProvider{Name: "mk-route-emb", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://emb.example/v1", Model: "bge-m3", ModelKind: "embedding", Enabled: true, TimeoutSeconds: 120}
	require.NoError(t, db.Create(&llmProv).Error)
	require.NoError(t, db.Create(&embProv).Error)

	// embedding route + llm provider → rejected.
	embRouteLLM := &models.AIRoute{Name: "default", Capability: string(CapabilityEmbedding), Enabled: true, Strategy: "ordered_failover"}
	err := store.UpsertRoute(embRouteLLM, []uint{llmProv.ID})
	require.Error(t, err, "embedding route must reject an llm provider")
	require.Contains(t, err.Error(), "embedding 路由不能挂 llm 模型")

	// summary (llm) route + embedding provider → rejected.
	sumRouteEmb := &models.AIRoute{Name: "default", Capability: string(CapabilitySummary), Enabled: true, Strategy: "ordered_failover"}
	err = store.UpsertRoute(sumRouteEmb, []uint{embProv.ID})
	require.Error(t, err, "llm route must reject an embedding provider")
	require.Contains(t, err.Error(), "llm 路由不能挂 embedding 模型")

	// embedding route + embedding provider → accepted.
	embRouteOK := &models.AIRoute{Name: "emb-ok", Capability: string(CapabilityEmbedding), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, store.UpsertRoute(embRouteOK, []uint{embProv.ID}), "embedding route must accept an embedding provider")

	// summary (llm) route + llm provider → accepted.
	sumRouteOK := &models.AIRoute{Name: "sum-ok", Capability: string(CapabilitySummary), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, store.UpsertRoute(sumRouteOK, []uint{llmProv.ID}), "llm route must accept an llm provider")

	// empty providerIDs → accepted (clears bindings, no type check).
	embRouteClear := &models.AIRoute{Name: "emb-clear", Capability: string(CapabilityEmbedding), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, store.UpsertRoute(embRouteClear, nil), "empty providerIDs must skip validation (clear bindings)")

	// non-existent provider → ErrProviderNotFound (not a generic DB error).
	embRouteMissing := &models.AIRoute{Name: "missing", Capability: string(CapabilityEmbedding), Enabled: true, Strategy: "ordered_failover"}
	err = store.UpsertRoute(embRouteMissing, []uint{9999999})
	require.ErrorIs(t, err, ErrProviderNotFound)
}
