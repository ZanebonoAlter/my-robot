package repository

import (
	"encoding/json"
	"errors"
	"time"

	"syntopica-backend/internal/platform/database"
)

// SourceType enumerates valid data source types.
type SourceType string

const (
	SourceTypeETFQuote     SourceType = "etf_quote"
	SourceTypeExchangeRate SourceType = "exchange_rate"
	SourceTypeGDELTEvent   SourceType = "gdelt_event"
)

var validSourceTypes = map[SourceType]bool{
	SourceTypeETFQuote:     true,
	SourceTypeExchangeRate: true,
	SourceTypeGDELTEvent:   true,
}

// ValidateSourceType returns an error if sourceType is not a known value.
func ValidateSourceType(sourceType string) error {
	if validSourceTypes[SourceType(sourceType)] {
		return nil
	}
	return errors.New("unknown source_type: " + sourceType)
}

// ── Models ───────────────────────────────────────────────────────────────────

// BoardDataSource binds a data source to a semantic board.
// Table: board_data_sources  UNIQUE(semantic_board_id, source_type)
type BoardDataSource struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	SemanticBoardID uint           `gorm:"uniqueIndex:idx_board_src;not null" json:"semantic_board_id"`
	SourceType      string         `gorm:"size:40;uniqueIndex:idx_board_src;not null" json:"source_type"`
	Config          map[string]any `gorm:"type:jsonb;serializer:json;default:'{}'" json:"config"`
	Enabled         bool           `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (BoardDataSource) TableName() string { return "board_data_sources" }

// LifelineGranularity enumerates context granularity levels.
type LifelineGranularity string

const (
	GranularityWeek  LifelineGranularity = "week"
	GranularityMonth LifelineGranularity = "month"
	GranularityYear  LifelineGranularity = "year"
	GranularityAll   LifelineGranularity = "all"
)

// TopicLifelineContext stores the news narrative summary at a given granularity.
// Table: topic_lifeline_context  UNIQUE(persistent_topic_id, granularity)
type TopicLifelineContext struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	PersistentTopicID uint      `gorm:"uniqueIndex:idx_topic_gran;not null" json:"persistent_topic_id"`
	Granularity       string    `gorm:"size:10;uniqueIndex:idx_topic_gran;not null" json:"granularity"`
	Content           string    `gorm:"type:text;not null" json:"content"`
	AsOfDate          time.Time `gorm:"type:date;not null" json:"as_of_date"`
	Source            string    `gorm:"size:12;not null;default:manual" json:"source"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (TopicLifelineContext) TableName() string { return "topic_lifeline_context" }

// TopicEnrichmentResult is an immutable snapshot of one enhancement run.
// Table: topic_enrichment_result
type TopicEnrichmentResult struct {
	ID                  uint            `gorm:"primarykey" json:"id"`
	PersistentTopicID   uint            `gorm:"not null;index" json:"persistent_topic_id"`
	EvolutionAssessment string          `gorm:"type:text" json:"evolution_assessment"`
	Sectors             json.RawMessage `gorm:"type:jsonb" json:"sectors"`
	CausalChain         string          `gorm:"type:text" json:"causal_chain"`
	ToolCalls           json.RawMessage `gorm:"type:jsonb" json:"tool_calls"`
	InputSnapshot       json.RawMessage `gorm:"type:jsonb" json:"input_snapshot"`
	SessionID           string          `gorm:"size:120" json:"session_id"`
	CreatedAt           time.Time       `json:"created_at"`
}

func (TopicEnrichmentResult) TableName() string { return "topic_enrichment_result" }

// TopicEnrichmentReview records a comparison between two result snapshots.
// Table: topic_enrichment_review
type TopicEnrichmentReview struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	PersistentTopicID uint      `gorm:"not null;index" json:"persistent_topic_id"`
	PrevResultID      *uint     `gorm:"index" json:"prev_result_id"`
	CurrResultID      uint      `gorm:"not null;index" json:"curr_result_id"`
	DeviationSummary  string    `gorm:"type:text;not null" json:"deviation_summary"`
	AffectedContext   string    `gorm:"size:10" json:"affected_context"`
	Confidence        *float64  `json:"confidence"`
	Applied           bool      `gorm:"not null;default:false" json:"applied"`
	Source            string    `gorm:"size:12;not null;default:llm_assisted" json:"source"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (TopicEnrichmentReview) TableName() string { return "topic_enrichment_review" }

func init() {
	database.RegisterModels(
		&BoardDataSource{},
		&TopicLifelineContext{},
		&TopicEnrichmentResult{},
		&TopicEnrichmentReview{},
	)
}
