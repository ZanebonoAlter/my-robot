package airouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	otelCodes "go.opentelemetry.io/otel/codes"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/tracing"
)

const maxResponseSnippet = 10000
const maxPromptRunes = 20000

var defaultConcurrency = map[Capability]int{
	CapabilityTopicTagging: 3,
	CapabilityDigestPolish: 2,
	CapabilityOpenNotebook: 2,
	CapabilityEmbedding:    5,
	// CapabilityFeedDiscovery: 推荐精排 LLM，低频突发（手动刷新/问答），与总结同档并发。
	CapabilityFeedDiscovery: 2,
}

func truncateSnippet(s string) string {
	runes := []rune(s)
	if len(runes) > maxResponseSnippet {
		return string(runes[:maxResponseSnippet]) + "..."
	}
	return s
}

// formatMessages joins ChatRequest messages into a human-readable prompt string
// for logging. Format: [role]\n{content}\n\n[role]\n{content}. If the result
// exceeds 20000 runes, the first 18000 and last 2000 are kept with a truncation
// marker in between.
func formatMessages(messages []Message) string {
	var b strings.Builder
	for i, m := range messages {
		b.WriteString("[")
		b.WriteString(m.Role)
		b.WriteString("]\n")
		b.WriteString(m.Content)
		if i < len(messages)-1 {
			b.WriteString("\n\n")
		}
	}
	raw := b.String()
	runes := []rune(raw)
	if len(runes) <= maxPromptRunes {
		return raw
	}
	keptHead := runes[:18000]
	keptTail := runes[len(runes)-2000:]
	truncated := len(runes) - 18000 - 2000
	return fmt.Sprintf("%s\n...[truncated %d runes]...\n%s", string(keptHead), truncated, string(keptTail))
}

// encodeTokenUsage marshals a TokenUsage pointer into a JSON string suitable for
// a jsonb column. Returns empty string when usage is nil.
func encodeTokenUsage(u *TokenUsage) string {
	if u == nil {
		return ""
	}
	b, _ := json.Marshal(u)
	return string(b)
}

type Router struct {
	store   *Store
	clients map[string]ProviderClient
	semMap  sync.Map // map[Capability]chan struct{}
}

func NewRouter() *Router {
	store := NewStore(database.DB)
	return &Router{
		store: store,
		clients: map[string]ProviderClient{
			ProviderTypeOpenAICompatible: NewOpenAICompatibleClient(),
			ProviderTypeOllama:           NewOpenAICompatibleClient(),
		},
	}
}

func NewRouterWithStore(store *Store) *Router {
	return &Router{
		store: store,
		clients: map[string]ProviderClient{
			ProviderTypeOpenAICompatible: NewOpenAICompatibleClient(),
			ProviderTypeOllama:           NewOpenAICompatibleClient(),
		},
	}
}

func (r *Router) RegisterClient(providerType string, client ProviderClient) {
	if client == nil {
		return
	}
	r.clients[providerType] = client
}

func (r *Router) resolveConcurrency(capability Capability, route *models.AIRoute) int {
	if route != nil && route.MaxConcurrency > 0 {
		return route.MaxConcurrency
	}
	if n, ok := defaultConcurrency[capability]; ok {
		return n
	}
	return 3
}

func (r *Router) getSemaphore(capability Capability, route *models.AIRoute) chan struct{} {
	n := r.resolveConcurrency(capability, route)
	ch, _ := r.semMap.LoadOrStore(capability, make(chan struct{}, n))
	return ch.(chan struct{})
}

func (r *Router) acquireSem(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Router) releaseSem(sem chan struct{}) {
	select {
	case <-sem:
	default:
	}
}

func (r *Router) Chat(ctx context.Context, req ChatRequest) (result *ChatResult, err error) {
	if req.Operation == "" {
		return nil, errors.New("airouter: Operation is required")
	}

	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "Router.Chat")
	defer span.End()
	defer func() {
		if err != nil {
			span.SetStatus(otelCodes.Error, "error")
			span.RecordError(err)
		}
	}()
	span.SetAttributes(attribute.String("ai.capability", string(req.Capability)))
	span.SetAttributes(attribute.String("ai.operation", req.Operation))
	if req.SessionID != "" {
		span.SetAttributes(attribute.String("ai.session_id", req.SessionID))
	}
	if b := baggage.FromContext(ctx); b.Len() > 0 {
		for _, m := range b.Members() {
			span.SetAttributes(attribute.String("baggage."+m.Key(), m.Value()))
		}
	}
	route, providers, err := r.store.LoadRouteWithProviders(req.Capability)
	if err != nil {
		return nil, err
	}

	sem := r.getSemaphore(req.Capability, route)
	if err := r.acquireSem(ctx, sem); err != nil {
		return nil, err
	}
	defer r.releaseSem(sem)

	var attemptErrors []error
	for idx, provider := range providers {
		client := r.clients[provider.ProviderType]
		if client == nil {
			attemptErrors = append(attemptErrors, fmt.Errorf("provider type %s unsupported", provider.ProviderType))
			continue
		}

		start := time.Now()
		chatResp, callErr := client.Chat(ctx, provider, req)
		latencyMs := int(time.Since(start).Milliseconds())
		if callErr == nil {
			r.store.LogCall(ctx, &models.AICallLog{
				Operation:       req.Operation,
				Capability:      string(req.Capability),
				RouteName:       route.Name,
				ProviderName:    provider.Name,
				Model:           provider.Model,
				Success:         true,
				IsFallback:      idx > 0,
				LatencyMs:       latencyMs,
				Prompt:          formatMessages(req.Messages),
				RequestMeta:     encodeMeta(req.Metadata),
				ResponseSnippet: truncateSnippet(chatResp.Content),
				TokenUsage:      encodeTokenUsage(chatResp.Usage),
				SessionID:       req.SessionID,
			})

			return &ChatResult{
				Content:      chatResp.Content,
				ProviderID:   provider.ID,
				ProviderName: provider.Name,
				RouteName:    route.Name,
				UsedFallback: idx > 0,
				AttemptCount: idx + 1,
				Usage:        chatResp.Usage,
			}, nil
		}

		providerErr := &ProviderError{}
		code := "provider_error"
		if errors.As(callErr, &providerErr) {
			if providerErr.Code != "" {
				code = providerErr.Code
			}
		}
		r.store.LogCall(ctx, &models.AICallLog{
			Operation:       req.Operation,
			Capability:      string(req.Capability),
			RouteName:       route.Name,
			ProviderName:    provider.Name,
			Model:           provider.Model,
			Success:         false,
			IsFallback:      idx > 0,
			LatencyMs:       latencyMs,
			ErrorCode:       code,
			ErrorMessage:    callErr.Error(),
			Prompt:          formatMessages(req.Messages),
			RequestMeta:     encodeMeta(req.Metadata),
			ResponseSnippet: truncateSnippet(callErr.Error()),
			SessionID:       req.SessionID,
		})
		attemptErrors = append(attemptErrors, fmt.Errorf("%s: %w", provider.Name, callErr))
	}

	finalErr := errors.Join(attemptErrors...)

	return nil, finalErr
}

func (r *Router) ResolvePrimaryProvider(capability Capability) (*models.AIProvider, *models.AIRoute, error) {
	return r.store.ResolvePrimaryProvider(capability)
}

func (r *Router) Embed(ctx context.Context, req EmbeddingRequest, capability Capability) (result *EmbeddingResult, err error) {
	if req.Operation == "" {
		return nil, errors.New("airouter: Operation is required")
	}

	_, span := otel.Tracer(tracing.ServiceName).Start(ctx, "Router.Embed")
	defer span.End()
	span.SetAttributes(attribute.String("ai.capability", string(capability)))
	span.SetAttributes(attribute.String("ai.operation", req.Operation))
	if req.SessionID != "" {
		span.SetAttributes(attribute.String("ai.session_id", req.SessionID))
	}

	route, providers, err := r.store.LoadRouteWithProviders(capability)
	if err != nil {
		return nil, err
	}

	sem := r.getSemaphore(capability, route)
	if err := r.acquireSem(ctx, sem); err != nil {
		return nil, err
	}
	defer r.releaseSem(sem)

	var attemptErrors []error
	for idx, provider := range providers {
		client := r.clients[provider.ProviderType]
		if client == nil {
			attemptErrors = append(attemptErrors, fmt.Errorf("provider type %s unsupported", provider.ProviderType))
			continue
		}

		start := time.Now()
		res, callErr := client.Embed(ctx, provider, req)
		latencyMs := int(time.Since(start).Milliseconds())
		if callErr == nil {
			r.store.LogCall(ctx, &models.AICallLog{
				Operation:    req.Operation,
				Capability:   string(capability),
				RouteName:    route.Name,
				ProviderName: provider.Name,
				Model:        provider.Model,
				Success:      true,
				IsFallback:   idx > 0,
				LatencyMs:    latencyMs,
				RequestMeta:  encodeMeta(req.Metadata),
				SessionID:    req.SessionID,
			})
			return res, nil
		}

		providerErr := &ProviderError{}
		code := "provider_error"
		if errors.As(callErr, &providerErr) {
			if providerErr.Code != "" {
				code = providerErr.Code
			}
		}
		r.store.LogCall(ctx, &models.AICallLog{
			Operation:    req.Operation,
			Capability:   string(capability),
			RouteName:    route.Name,
			ProviderName: provider.Name,
			Model:        provider.Model,
			Success:      false,
			IsFallback:   idx > 0,
			LatencyMs:    latencyMs,
			ErrorCode:    code,
			ErrorMessage: callErr.Error(),
			RequestMeta:  encodeMeta(req.Metadata),
			SessionID:    req.SessionID,
		})
		attemptErrors = append(attemptErrors, fmt.Errorf("%s: %w", provider.Name, callErr))
	}

	return nil, errors.Join(attemptErrors...)
}
