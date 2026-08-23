package airouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/tracing"
)

type Capability string

const (
	CapabilitySummary            Capability = "summary"
	CapabilityTopicTagging       Capability = "topic_tagging"
	CapabilityDigestPolish       Capability = "digest_polish"
	CapabilityOpenNotebook       Capability = "open_notebook"
	CapabilityEmbedding          Capability = "embedding"
	CapabilityFeedDiscovery      Capability = "feed_discovery"
	DefaultRouteName             string     = "default"
	DefaultProviderName          string     = "default-primary"
	ProviderTypeOpenAICompatible string     = "openai_compatible"
	ProviderTypeOllama           string     = "ollama"
)

var (
	ErrRouteNotFound    = errors.New("ai route not found")
	ErrNoProviders      = errors.New("ai route has no enabled providers")
	ErrProviderNotFound = errors.New("ai provider not found")
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	if db == nil {
		db = database.DB
	}
	return &Store{db: db}
}

func (s *Store) ListProviders() ([]models.AIProvider, error) {
	var providers []models.AIProvider
	if err := s.db.Order("enabled DESC, name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

func (s *Store) ListRoutes() ([]models.AIRoute, error) {
	var routes []models.AIRoute
	if err := s.db.Preload("RouteProviders", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("priority ASC").Preload("Provider")
	}).Order("capability ASC, name ASC").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

func (s *Store) LoadRouteWithProviders(capability Capability) (*models.AIRoute, []models.AIProvider, error) {
	var route models.AIRoute
	err := s.db.Where("capability = ? AND enabled = ?", string(capability), true).
		Order("CASE WHEN name = 'default' THEN 0 ELSE 1 END").
		Order("id ASC").
		Preload("RouteProviders", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("enabled = ?", true).Order("priority ASC").Preload("Provider")
		}).
		First(&route).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrRouteNotFound
		}
		return nil, nil, err
	}

	providers := make([]models.AIProvider, 0, len(route.RouteProviders))
	for _, link := range route.RouteProviders {
		if !link.Enabled || !link.Provider.Enabled {
			continue
		}
		providers = append(providers, link.Provider)
	}
	if len(providers) == 0 {
		return &route, nil, ErrNoProviders
	}
	return &route, providers, nil
}

func (s *Store) UpsertProvider(provider *models.AIProvider) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}
	provider.Name = strings.TrimSpace(provider.Name)
	provider.ProviderType = strings.TrimSpace(provider.ProviderType)
	provider.BaseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	provider.Model = strings.TrimSpace(provider.Model)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	if provider.Name == "" || provider.BaseURL == "" || provider.Model == "" {
		return fmt.Errorf("provider fields are incomplete")
	}
	if provider.ProviderType == "" {
		provider.ProviderType = ProviderTypeOpenAICompatible
	}
	if provider.TimeoutSeconds <= 0 {
		provider.TimeoutSeconds = 120
	}
	// model_kind: empty normalizes to llm (the default); only llm/embedding are
	// accepted. Validate before any DB access so an invalid value fails fast.
	switch kind := strings.TrimSpace(provider.ModelKind); kind {
	case "":
		provider.ModelKind = "llm"
	case "llm", "embedding":
		provider.ModelKind = kind
	default:
		return fmt.Errorf("invalid model_kind: %q (want llm or embedding)", kind)
	}

	// Name is unique across providers. A provider carrying a non-zero ID is
	// an UPDATE of that row (possibly renaming it): the name is rejected only
	// when taken by a DIFFERENT row. A zero ID means CREATE: any existing name
	// is rejected. Without this split, renaming a provider fell through to
	// Create with an explicit primary key (duplicate pkey) and renaming onto
	// another provider's name silently overwrote that row.
	var conflicting models.AIProvider
	nameQuery := s.db.Where("name = ?", provider.Name)
	if provider.ID != 0 {
		nameQuery = nameQuery.Where("id <> ?", provider.ID)
	}
	err := nameQuery.First(&conflicting).Error
	if err == nil {
		return fmt.Errorf("provider name %q already exists", provider.Name)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if provider.ID != 0 {
		return s.db.Save(provider).Error
	}
	return s.db.Create(provider).Error
}

func (s *Store) UpsertRoute(route *models.AIRoute, providerIDs []uint) error {
	if route == nil {
		return fmt.Errorf("route is nil")
	}
	if strings.TrimSpace(route.Capability) == "" || strings.TrimSpace(route.Name) == "" {
		return fmt.Errorf("route fields are incomplete")
	}
	if route.Strategy == "" {
		route.Strategy = "ordered_failover"
	}

	// Route binding type check: an embedding capability route may only bind
	// embedding providers; every other (llm-class) capability route may only
	// bind llm providers. Done before the transaction so a rejection leaves the
	// route bindings untouched. Empty providerIDs clears bindings — no check.
	if len(providerIDs) > 0 {
		expectedKind := "llm"
		if route.Capability == string(CapabilityEmbedding) {
			expectedKind = "embedding"
		}
		for _, providerID := range providerIDs {
			var p models.AIProvider
			if err := s.db.Select("id, name, model_kind").First(&p, providerID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrProviderNotFound
				}
				return err
			}
			if p.ModelKind != expectedKind {
				if expectedKind == "embedding" {
					return fmt.Errorf("embedding 路由不能挂 llm 模型「%s」", p.Name)
				}
				return fmt.Errorf("llm 路由不能挂 embedding 模型「%s」", p.Name)
			}
		}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "capability"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "strategy", "description", "updated_at"}),
		}).Create(route).Error; err != nil {
			return err
		}

		if err := tx.Where("capability = ? AND name = ?", route.Capability, route.Name).First(route).Error; err != nil {
			return err
		}

		if err := tx.Where("route_id = ?", route.ID).Delete(&models.AIRouteProvider{}).Error; err != nil {
			return err
		}

		for idx, providerID := range providerIDs {
			link := models.AIRouteProvider{RouteID: route.ID, ProviderID: providerID, Priority: idx + 1, Enabled: true}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ResolvePrimaryProvider(capability Capability) (*models.AIProvider, *models.AIRoute, error) {
	route, providers, err := s.LoadRouteWithProviders(capability)
	if err != nil {
		return nil, nil, err
	}
	provider := providers[0]
	return &provider, route, nil
}

func (s *Store) LogCall(ctx context.Context, logEntry *models.AICallLog) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "Store.LogCall")
	defer span.End()
	if logEntry == nil || s.db == nil {
		return
	}
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.HasTraceID() {
		logEntry.TraceID = spanCtx.TraceID().String()
	}
	// token_usage 列是 JSONB，空串不是合法 JSON；空值时省略该列让 DB 置 NULL。
	if logEntry.TokenUsage == "" {
		_ = s.db.Omit("token_usage").Create(logEntry).Error
		return
	}
	_ = s.db.Create(logEntry).Error
}

// SaveEmbeddingCache persists one embedding result with ON CONFLICT DO NOTHING
// semantics: a concurrent writer that raced us to the same cache_key wins and
// our row is silently dropped. Cache writes are best-effort by design.
func (s *Store) SaveEmbeddingCache(_ context.Context, rec *models.AIEmbeddingCache) error {
	if s.db == nil || rec == nil {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(rec).Error
}

// LookupEmbeddingCache fetches a cached embedding by cache_key. Returns
// (nil, nil) on miss or when no DB is configured.
func (s *Store) LookupEmbeddingCache(_ context.Context, key string) (*models.AIEmbeddingCache, error) {
	if s.db == nil {
		return nil, nil
	}
	var rec models.AIEmbeddingCache
	err := s.db.Where("cache_key = ?", key).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func encodeMeta(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
