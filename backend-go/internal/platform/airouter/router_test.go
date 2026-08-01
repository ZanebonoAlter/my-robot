package airouter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/models"
)

type fakeProviderClient struct {
	responses map[string]struct {
		content string
		usage   *TokenUsage
		err     error
	}
}

func (f *fakeProviderClient) Chat(_ context.Context, provider models.AIProvider, _ ChatRequest) (*ChatResponse, error) {
	res := f.responses[provider.Name]
	if res.err != nil {
		return nil, res.err
	}
	return &ChatResponse{Content: res.content, Usage: res.usage}, nil
}

func (f *fakeProviderClient) Embed(_ context.Context, provider models.AIProvider, _ EmbeddingRequest) (*EmbeddingResult, error) {
	res := f.responses[provider.Name]
	if res.err != nil {
		return nil, res.err
	}
	return &EmbeddingResult{Embeddings: [][]float64{{0.1, 0.2}}, Model: "test", Dimensions: 2, Provider: provider.Name}, nil
}

func TestRouterFallsBackOnRetryableProviderError(t *testing.T) {
	db := setupAIRouterTestDB(t)
	store := NewStore(db)

	p1 := models.AIProvider{Name: "primary", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://a.example/v1", APIKey: "a", Model: "m1", Enabled: true}
	p2 := models.AIProvider{Name: "backup", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://b.example/v1", APIKey: "b", Model: "m2", Enabled: true}
	require.NoError(t, db.Create(&p1).Error)
	require.NoError(t, db.Create(&p2).Error)
	route := models.AIRoute{Name: DefaultRouteName, Capability: string(CapabilitySummary), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, db.Create(&route).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p1.ID, Priority: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p2.ID, Priority: 2, Enabled: true}).Error)

	router := NewRouterWithStore(store)
	router.RegisterClient(ProviderTypeOpenAICompatible, &fakeProviderClient{responses: map[string]struct {
		content string
		usage   *TokenUsage
		err     error
	}{
		"primary": {err: &ProviderError{Message: "rate limited", Code: "rate_limit", Retryable: true}},
		"backup":  {content: "ok from backup"},
	}})

	result, err := router.Chat(context.Background(), ChatRequest{
		Operation:  "test.op",
		Capability: CapabilitySummary,
		Messages:   []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "ok from backup", result.Content)
	require.True(t, result.UsedFallback)
	require.Equal(t, 2, result.AttemptCount)
}

func TestRouterFallsBackOnTerminalError(t *testing.T) {
	db := setupAIRouterTestDB(t)
	store := NewStore(db)

	p1 := models.AIProvider{Name: "primary-terminal", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://a.example/v1", APIKey: "a", Model: "m1", Enabled: true}
	p2 := models.AIProvider{Name: "backup", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://b.example/v1", APIKey: "b", Model: "m2", Enabled: true}
	require.NoError(t, db.Create(&p1).Error)
	require.NoError(t, db.Create(&p2).Error)
	route := models.AIRoute{Name: DefaultRouteName, Capability: string(CapabilitySummary), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, db.Create(&route).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p1.ID, Priority: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p2.ID, Priority: 2, Enabled: true}).Error)

	router := NewRouterWithStore(store)
	router.RegisterClient(ProviderTypeOpenAICompatible, &fakeProviderClient{responses: map[string]struct {
		content string
		usage   *TokenUsage
		err     error
	}{
		"primary-terminal": {err: &ProviderError{Message: "invalid key", Code: "unauthorized", Retryable: false}},
		"backup":           {content: "ok from backup"},
	}})

	result, err := router.Chat(context.Background(), ChatRequest{
		Operation:  "test.op",
		Capability: CapabilitySummary,
		Messages:   []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "ok from backup", result.Content)
	require.True(t, result.UsedFallback)
	require.Equal(t, 2, result.AttemptCount)
}

func TestRouterChatRejectsEmptyOperation(t *testing.T) {
	db := setupAIRouterTestDB(t)
	store := NewStore(db)

	p := models.AIProvider{Name: "p", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://a.example/v1", APIKey: "k", Model: "m", Enabled: true}
	require.NoError(t, db.Create(&p).Error)
	route := models.AIRoute{Name: DefaultRouteName, Capability: string(CapabilitySummary), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, db.Create(&route).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p.ID, Priority: 1, Enabled: true}).Error)

	router := NewRouterWithStore(store)
	router.RegisterClient(ProviderTypeOpenAICompatible, &fakeProviderClient{responses: map[string]struct {
		content string
		usage   *TokenUsage
		err     error
	}{
		"p": {content: "ok"},
	}})

	result, err := router.Chat(context.Background(), ChatRequest{
		Capability: CapabilitySummary,
		Messages:   []Message{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Operation is required")
	require.Nil(t, result)
}

func TestRouterEmbedRejectsEmptyOperation(t *testing.T) {
	db := setupAIRouterTestDB(t)
	store := NewStore(db)

	p := models.AIProvider{Name: "emb-p", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://a.example/v1", APIKey: "k", Model: "m", Enabled: true}
	require.NoError(t, db.Create(&p).Error)
	route := models.AIRoute{Name: DefaultRouteName, Capability: string(CapabilityEmbedding), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, db.Create(&route).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p.ID, Priority: 1, Enabled: true}).Error)

	router := NewRouterWithStore(store)
	router.RegisterClient(ProviderTypeOpenAICompatible, &fakeProviderClient{responses: map[string]struct {
		content string
		usage   *TokenUsage
		err     error
	}{
		"emb-p": {content: "ok"},
	}})

	result, err := router.Embed(context.Background(), EmbeddingRequest{Input: []string{"test"}}, CapabilityEmbedding)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Operation is required")
	require.Nil(t, result)
}

func TestFormatMessages(t *testing.T) {
	t.Run("simple messages", func(t *testing.T) {
		result := formatMessages([]Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		})
		require.Equal(t, "[system]\nYou are helpful.\n\n[user]\nHello", result)
	})

	t.Run("single message", func(t *testing.T) {
		result := formatMessages([]Message{
			{Role: "user", Content: "hi"},
		})
		require.Equal(t, "[user]\nhi", result)
	})

	t.Run("truncation over 20000 runes", func(t *testing.T) {
		bigContent := make([]rune, 21000)
		for i := range bigContent {
			bigContent[i] = 'a'
		}
		result := formatMessages([]Message{
			{Role: "user", Content: string(bigContent)},
		})
		require.Contains(t, result, "[truncated")
		require.Contains(t, result, "[user]\n")
		// Result should be shorter than raw input (21000 + marker overhead),
		// but can be slightly above 20000 due to truncation marker.
		runes := []rune(result)
		require.Less(t, len(runes), 21000, "truncated result should be shorter than raw input")
	})

	t.Run("under limit no truncation", func(t *testing.T) {
		result := formatMessages([]Message{
			{Role: "user", Content: "short"},
		})
		require.NotContains(t, result, "[truncated")
	})
}

func TestTokenUsageInChatResult(t *testing.T) {
	db := setupAIRouterTestDB(t)
	store := NewStore(db)

	usage := &TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}

	p := models.AIProvider{Name: "p-usage", ProviderType: ProviderTypeOpenAICompatible, BaseURL: "https://a.example/v1", APIKey: "k", Model: "m", Enabled: true}
	require.NoError(t, db.Create(&p).Error)
	route := models.AIRoute{Name: DefaultRouteName, Capability: string(CapabilitySummary), Enabled: true, Strategy: "ordered_failover"}
	require.NoError(t, db.Create(&route).Error)
	require.NoError(t, db.Create(&models.AIRouteProvider{RouteID: route.ID, ProviderID: p.ID, Priority: 1, Enabled: true}).Error)

	router := NewRouterWithStore(store)
	router.RegisterClient(ProviderTypeOpenAICompatible, &fakeProviderClient{responses: map[string]struct {
		content string
		usage   *TokenUsage
		err     error
	}{
		"p-usage": {content: "response", usage: usage},
	}})

	result, err := router.Chat(context.Background(), ChatRequest{
		Operation:  "test.op",
		Capability: CapabilitySummary,
		Messages:   []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Usage)
	require.Equal(t, 100, result.Usage.PromptTokens)
	require.Equal(t, 50, result.Usage.CompletionTokens)
	require.Equal(t, 150, result.Usage.TotalTokens)
}

func TestEncodeTokenUsage(t *testing.T) {
	require.Equal(t, "", encodeTokenUsage(nil))

	u := &TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	encoded := encodeTokenUsage(u)
	require.Contains(t, encoded, "\"prompt\":10")
	require.Contains(t, encoded, "\"completion\":20")
	require.Contains(t, encoded, "\"total\":30")
}
