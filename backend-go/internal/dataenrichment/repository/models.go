package repository

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"syntopica-backend/internal/platform/database"
)

// SourceType enumerates valid data source types. The type + CHECK mechanism
// is retained as an extension point for future structured external sources,
// but after the financial-direction removal there are NO built-in source types
// (etf_quote/exchange_rate/gdelt_event have been removed). ValidateSourceType
// therefore rejects every value until a new source type is registered here.
type SourceType string

var (
	sourceTypeMu     sync.RWMutex
	validSourceTypes = map[SourceType]bool{}
)

// RegisterSourceType adds st to the set of valid source types. The source_type
// enum is intentionally extensible (spec "板块数据源绑定"): future structured
// external sources register their type at wiring time rather than hard-coding a
// constant here. The removed built-in financial types (etf_quote /
// exchange_rate / gdelt_event) are NOT registered and stay rejected.
func RegisterSourceType(st SourceType) {
	sourceTypeMu.Lock()
	defer sourceTypeMu.Unlock()
	validSourceTypes[st] = true
}

// ValidateSourceType returns an error if sourceType is not a known value.
func ValidateSourceType(sourceType string) error {
	sourceTypeMu.RLock()
	defer sourceTypeMu.RUnlock()
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

// TopicLifelineContext stores the news narrative summary at a given granularity + period.
// Table: topic_lifeline_context  UNIQUE(persistent_topic_id, granularity, period)
// See design.md §2.1: period-archival storage (one row per period, not rolling overwrite).
type TopicLifelineContext struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	PersistentTopicID uint      `gorm:"uniqueIndex:idx_topic_gran_period;not null" json:"persistent_topic_id"`
	Granularity       string    `gorm:"size:10;uniqueIndex:idx_topic_gran_period;not null" json:"granularity"`
	Period            string    `gorm:"size:12;uniqueIndex:idx_topic_gran_period;not null" json:"period"`
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
	ID                uint            `gorm:"primarykey" json:"id"`
	PersistentTopicID uint            `gorm:"not null;index" json:"persistent_topic_id"`
	PrevResultID      *uint           `gorm:"index" json:"prev_result_id"`
	CurrResultID      uint            `gorm:"not null;index" json:"curr_result_id"`
	Verdict           json.RawMessage `gorm:"type:jsonb" json:"verdict"`
	DeviationSummary  string          `gorm:"type:text;not null" json:"deviation_summary"`
	AffectedContext   string          `gorm:"size:10" json:"affected_context"`
	Confidence        *float64        `json:"confidence"`
	Applied           bool            `gorm:"not null;default:false" json:"applied"`
	Source            string          `gorm:"size:12;not null;default:llm_assisted" json:"source"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

func (TopicEnrichmentReview) TableName() string { return "topic_enrichment_review" }

// StockDebateResult stores FinGenius multi-role debate output per stock.
// See design.md §4.2b: append-only by (result_id, sector, code).
// Table: stock_debate_result
type StockDebateResult struct {
	ID                      uint            `gorm:"primarykey" json:"id"`
	TopicEnrichmentResultID uint            `gorm:"not null;index" json:"topic_enrichment_result_id"`
	PersistentTopicID       uint            `gorm:"not null;index" json:"persistent_topic_id"`
	Sector                  string          `gorm:"size:80;not null" json:"sector"`
	Code                    string          `gorm:"size:20;not null" json:"code"`
	Name                    string          `gorm:"size:60" json:"name"`
	Verdict                 string          `gorm:"size:8;not null" json:"verdict"`
	Consensus               string          `gorm:"size:12" json:"consensus"`
	Agents                  json.RawMessage `gorm:"type:jsonb" json:"agents"`
	Votes                   json.RawMessage `gorm:"type:jsonb" json:"votes"`
	FingeniusResearch       json.RawMessage `gorm:"type:jsonb" json:"fingenius_research"`
	FingeniusBattle         json.RawMessage `gorm:"type:jsonb" json:"fingenius_battle"`
	FingeniusTaskID         string          `gorm:"size:120" json:"fingenius_task_id"`
	DistillStatus           string          `gorm:"size:12;not null;default:done" json:"distill_status"`
	HTMLContent             string          `gorm:"type:text" json:"html_content"`
	CreatedAt               time.Time       `json:"created_at"`
}

func (StockDebateResult) TableName() string { return "stock_debate_result" }

// TopicEnrichmentQA stores an append-only report follow-up Q&A round tied to
// one immutable TopicEnrichmentResult snapshot. Multiple rounds may share a
// result_id; the report itself is never rewritten (报告不可变).
// Table: topic_enrichment_qa
//
// Added by causal-analysis-agent: 分析目标从「演进定位」改为「探索判断 agent」，
// 报告追问交互层需要独立的新表，旧演进定位数据(result/review)由迁移清空。
type TopicEnrichmentQA struct {
	ID                      uint            `gorm:"primarykey" json:"id"`
	TopicEnrichmentResultID uint            `gorm:"not null;index" json:"topic_enrichment_result_id"`
	Question                string          `gorm:"type:text;not null" json:"question"`
	Answer                  string          `gorm:"type:text" json:"answer"`
	ToolCalls               json.RawMessage `gorm:"type:jsonb" json:"tool_calls"`
	Source                  string          `gorm:"size:12;not null;default:qa" json:"source"`
	// Sedimented marks a Q&A round the user manually pinned as a durable note.
	// Sediment only flips this flag on the qa row; the report itself (result) is
	// never rewritten (业务约束#2: result 不可变). Default false.
	Sedimented bool      `gorm:"not null;default:false" json:"sedimented"`
	CreatedAt  time.Time `json:"created_at"`
}

func (TopicEnrichmentQA) TableName() string { return "topic_enrichment_qa" }

func init() {
	database.RegisterModels(
		&BoardDataSource{},
		&TopicLifelineContext{},
		&TopicEnrichmentResult{},
		&TopicEnrichmentReview{},
		&StockDebateResult{},
		&TopicEnrichmentQA{},
	)
}
