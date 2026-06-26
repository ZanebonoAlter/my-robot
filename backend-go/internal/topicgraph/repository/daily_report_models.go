package repository

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	tagging "syntopica-backend/internal/tagmanagement"
)

// BoardDailyReport — one report per board per day
type BoardDailyReport struct {
	ID                      uint      `gorm:"primarykey" json:"id"`
	SemanticBoardID         uint      `gorm:"index;not null" json:"semantic_board_id"`
	PeriodDate              time.Time `gorm:"type:date;not null" json:"period_date"`
	Title                   string    `json:"title"`
	Summary                 string    `json:"summary"`
	Highlights              JSON      `gorm:"type:jsonb" json:"highlights"`
	Dynamics                string    `gorm:"type:text" json:"dynamics"`
	ArticleCount            int       `json:"article_count"`
	EventTagCount           int       `json:"event_tag_count"`
	ClusterCount            int       `json:"cluster_count"`
	Status                  string    `gorm:"size:20;default:generating" json:"status"`
	RawClusters             JSON      `gorm:"type:jsonb" json:"raw_clusters,omitempty"`
	PrevReportID            *uint     `json:"prev_report_id,omitempty"`
	GenerationPromptVersion string    `gorm:"size:20" json:"generation_prompt_version,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`

	Sections []DailyReportSection `gorm:"foreignKey:ReportID" json:"sections,omitempty"`
}

func (BoardDailyReport) TableName() string {
	return "board_daily_reports"
}

// SectionRelation represents a many-to-many relation between sections across days.
// RelationType distinguishes edges produced by the Hungarian bipartite matcher
// (similarity) from edges written because both endpoints share the same
// PersistentTopic (identity). Identity edges bypass the 0.28 match penalty so a
// narrative chain survives cluster-label drift across days.
type SectionRelation struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	FromSectionID uint      `gorm:"not null;index:idx_section_relations_from" json:"from_section_id"`
	ToSectionID   uint      `gorm:"not null;index:idx_section_relations_to" json:"to_section_id"`
	Distance      float64   `gorm:"not null" json:"distance"`
	RelationType  string    `gorm:"size:20;not null;default:similarity;index:idx_section_relations_type" json:"relation_type"`
	CreatedAt     time.Time `json:"created_at"`
}

func (SectionRelation) TableName() string {
	return "daily_report_section_relations"
}

// Persistent topic status values.
const (
	TopicStatusCandidate = "candidate" // freshly emerged narrative, under observation
	TopicStatusActive    = "active"    // promoted after consecutive hits
	TopicStatusArchived  = "archived"  // decayed past the decay window
)

// PersistentTopicConfidence values recorded on each DailyReportSection.
const (
	TopicConfAnchorHit = "anchor_hit" // matched an existing topic via dual confirmation
	TopicConfAutoNew   = "auto_new"   // dual confirmation failed → opened a new candidate
	TopicConfUnmatched = "unmatched"  // section has no embedding, cannot assign
)

// BoardPersistentTopic is a durable narrative frame within a SemanticBoard.
// One board has N topics; each DailyReportSection is assigned to exactly one
// topic (1:N). Topics auto-promote from candidate→active after consecutive hits
// and decay to archived when no section references them for decay_window days.
type BoardPersistentTopic struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	SemanticBoardID uint      `gorm:"not null;index:idx_persistent_topics_board_status,priority:1" json:"semantic_board_id"`
	Label           string    `gorm:"size:200;not null" json:"label"`
	Description     string    `gorm:"type:text" json:"description"`
	Embedding       string    `gorm:"type:vector" json:"-"`
	Status          string    `gorm:"size:20;not null;default:candidate;index:idx_persistent_topics_board_status,priority:2" json:"status"`
	FirstSeenDate   time.Time `gorm:"type:date;not null" json:"first_seen_date"`
	LastSeenDate    time.Time `gorm:"type:date;not null" json:"last_seen_date"`
	HitCount        int       `gorm:"not null;default:1" json:"hit_count"`
	ConsecutiveHits int       `gorm:"not null;default:0" json:"consecutive_hits"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (BoardPersistentTopic) TableName() string {
	return "board_persistent_topics"
}

// DailyReportSection — one section per cluster
type DailyReportSection struct {
	ID            uint                `gorm:"primarykey" json:"id"`
	ReportID      uint                `gorm:"index;not null" json:"report_id"`
	ClusterIndex  int                 `json:"cluster_index"`
	ClusterLabel  string              `gorm:"size:200" json:"cluster_label"`
	ClusterTagIDs JSON                `gorm:"type:jsonb" json:"cluster_tag_ids"`
	Threads       []DailyReportThread `gorm:"foreignKey:SectionID" json:"threads,omitempty"`
	ArticleCount  int                 `json:"article_count"`
	BestTier      int                 `gorm:"default:0" json:"best_tier"`
	AvgScore          float64             `gorm:"default:0" json:"avg_score"`
	QualityBreakdown  JSON                `gorm:"type:jsonb" json:"quality_breakdown"`
	Embedding         string              `gorm:"type:vector" json:"-"`
	// Persistent topic assignment. NOT NULL is intentionally omitted at the DB
	// layer to tolerate the backfill window and historical rows; the assignment
	// algorithm guarantees new sections are always assigned (except the
	// unmatched branch when Embedding is empty).
	PersistentTopicID    *uint   `gorm:"index" json:"persistent_topic_id,omitempty"`
	TopicMatchDistance   float64 `json:"topic_match_distance,omitempty"`
	TopicMatchConfidence string  `gorm:"size:20" json:"topic_match_confidence,omitempty"`
	// PersistentTopic carries the nested topic brief for the daily-report
	// detail API, so the UI can classify sections by topic status (active vs
	// candidate). Transient — loaded via AttachTopicBriefsToReport, never
	// persisted. Mirrors SectionTimelineNode.PersistentTopic.
	PersistentTopic *PersistentTopicBrief `gorm:"-" json:"persistent_topic,omitempty"`
	// MatchedTopicID is the topic the LLM picked during ClusterTags; carried
	// transiently (not persisted) for the dual-confirmation assignment step.
	MatchedTopicID *uint     `gorm:"-" json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

func (DailyReportSection) TableName() string {
	return "daily_report_sections"
}

// DailyReportThread — one narrative thread, stored independently
type DailyReportThread struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	ReportID          uint      `gorm:"index;not null" json:"report_id"`
	SectionID         uint      `gorm:"index;not null" json:"section_id"`
	Title             string    `json:"title"`
	Summary           string    `json:"summary"`
	TagIDs            JSON      `gorm:"type:jsonb" json:"tag_ids"`
	Confidence        float64   `gorm:"default:0" json:"confidence"`
	RelatedArticleIDs JSON      `gorm:"type:jsonb" json:"related_article_ids,omitempty"`
	Embedding         string    `gorm:"type:vector" json:"-"`
	// FitDistance is a pointer so that nil ("no signal" — embed failure or
	// owning section without an embedding) is distinguishable from a real 0.0
	// ("perfect fit" — thread title embedding identical to its section's).
	// Under float64+omitempty both serialized to an absent `fit_distance`
	// field, conflating the best possible fit with no signal. nil is omitted by
	// omitempty; a non-nil 0.0 is serialized as `"fit_distance":0`. No DB
	// default so historical rows stay NULL per spec.
	FitDistance       *float64  `json:"fit_distance,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

func (DailyReportThread) TableName() string {
	return "daily_report_threads"
}

// JSON is a custom type for GORM jsonb columns.
type JSON []byte

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSON value: %v", value)
	}
	*j = append((*j)[0:0], bytes...)
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = append((*j)[0:0], data...)
	return nil
}

// floatsToPgVector converts a float64 slice to pgvector string format: [0.1,0.2,0.3]
func FloatsToPgVector(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// TagInput mirrors narrative.TagInput for use in the daily report pipeline.
type TagInput struct {
	ID           uint    `json:"id"`
	Label        string  `json:"label"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	ArticleCount int     `json:"article_count"`
	Source       string  `json:"source"`
	MatchReason  string  `json:"match_reason"`
	Score        float64 `json:"score"`
	Downgraded   bool    `json:"downgraded"`
}

// ClusterGroup represents a group of tags clustered by the LLM.
// MatchedTopicID carries the LLM's pick of an existing PersistentTopic for
// this group (nil when the LLM judges it a new narrative). Validated by the
// assignment step against the supplied topic set to drop hallucinated IDs.
type ClusterGroup struct {
	GroupName      string `json:"group_name"`
	TagIDs         []uint `json:"tag_ids"`
	MatchedTopicID *uint  `json:"matched_topic_id,omitempty"`
}

// Highlight represents a key highlight in the daily report.
type Highlight struct {
	Title  string `json:"title"`
	Reason string `json:"reason"`
	TagIDs []uint `json:"tag_ids"`
}

// Thread represents a narrative thread within a cluster section.
type Thread struct {
	Title             string  `json:"title"`
	Summary           string  `json:"summary"`
	TagIDs            []uint  `json:"tag_ids"`
	Confidence        float64 `json:"confidence"`
	RelatedArticleIDs []uint  `json:"related_article_ids,omitempty"`
}

// HotspotDigestCard is a lightweight card used in digest listing by topic tag.
type HotspotDigestCard struct {
	ID          uint                         `json:"id"`
	Title       string                       `json:"title"`
	Link        string                       `json:"link"`
	FeedName    string                       `json:"feed_name"`
	FeedIcon    string                       `json:"feed_icon,omitempty"`
	FeedColor   string                       `json:"feed_color,omitempty"`
	PublishedAt string                       `json:"published_at,omitempty"`
	Tags        []tagging.AggregatedTopicTag `json:"tags,omitempty"`
}

// ArticleTagData represents aggregated data from article_topic_tags for graph building.
type ArticleTagData struct {
	ArticleID uint
	FeedID    uint
	FeedTitle string
	FeedColor string
	TopicTag  *models.TopicTag
	Score     float64
}

// ensureSectionEmbeddingDimension sets the daily_report_sections.embedding column to
// the correct vector dimension and creates the HNSW index if dimension ≤ 2000.
// Called once at startup via tagging.RegisterVectorDimEnsurer.
func ensureSectionEmbeddingDimension(dim int) {
	db := Repo.DB()
	if db == nil {
		return
	}

	if err := db.Exec("SET LOCAL lock_timeout = '5s'" /* #nosec G201 */).Error; err != nil {
		logging.Warnf("Failed to set lock_timeout: %v", err)
	}

	var typeStr string
	if err := db.Raw(`
		SELECT format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.relname = 'daily_report_sections' AND a.attname = 'embedding'
	`).Row().Scan(&typeStr); err != nil {
		return // column may not exist yet
	}

	expected := fmt.Sprintf("vector(%d)", dim)
	needAlter := typeStr != expected
	needIndex := true

	if needAlter {
		logging.Infof("Altering daily_report_sections.embedding from %s to %s", typeStr, expected)
		_ = db.Exec("DROP INDEX IF EXISTS idx_daily_report_sections_embedding").Error
		if err := db.Exec(fmt.Sprintf(
			"ALTER TABLE daily_report_sections ALTER COLUMN embedding TYPE %s", expected,
		)).Error; err != nil {
			logging.Warnf("Failed to alter daily_report_sections.embedding to %s: %v", expected, err)
			return
		}
	}

	// pgvector HNSW supports up to 2000 dimensions.
	if dim > 2000 {
		logging.Infof("Skipping HNSW index on daily_report_sections.embedding: dimension %d > 2000 limit", dim)
		needIndex = false
	}

	if needIndex {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_report_sections_embedding ON daily_report_sections USING hnsw (embedding vector_cosine_ops)`).Error; err != nil {
			logging.Warnf("Failed to create HNSW index on daily_report_sections.embedding: %v", err)
		}
	}
}

// ensurePersistentTopicEmbeddingDimension aligns board_persistent_topics.embedding
// to the active vector dimension and creates the HNSW index when dim ≤ 2000.
// Mirrors ensureSectionEmbeddingDimension; called at startup via
// tagging.RegisterVectorDimEnsurer.
func ensurePersistentTopicEmbeddingDimension(dim int) {
	db := Repo.DB()
	if db == nil {
		return
	}

	if err := db.Exec("SET LOCAL lock_timeout = '5s'" /* #nosec G201 */).Error; err != nil {
		logging.Warnf("Failed to set lock_timeout: %v", err)
	}

	var typeStr string
	if err := db.Raw(`
		SELECT format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.relname = 'board_persistent_topics' AND a.attname = 'embedding'
	`).Row().Scan(&typeStr); err != nil {
		return // column may not exist yet
	}

	expected := fmt.Sprintf("vector(%d)", dim)
	needAlter := typeStr != expected
	needIndex := true

	if needAlter {
		logging.Infof("Altering board_persistent_topics.embedding from %s to %s", typeStr, expected)
		_ = db.Exec("DROP INDEX IF EXISTS idx_board_persistent_topics_embedding").Error
		if err := db.Exec(fmt.Sprintf(
			"ALTER TABLE board_persistent_topics ALTER COLUMN embedding TYPE %s", expected,
		)).Error; err != nil {
			logging.Warnf("Failed to alter board_persistent_topics.embedding to %s: %v", expected, err)
			return
		}
	}

	// pgvector HNSW supports up to 2000 dimensions.
	if dim > 2000 {
		logging.Infof("Skipping HNSW index on board_persistent_topics.embedding: dimension %d > 2000 limit", dim)
		needIndex = false
	}

	if needIndex {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_board_persistent_topics_embedding ON board_persistent_topics USING hnsw (embedding vector_cosine_ops)`).Error; err != nil {
			logging.Warnf("Failed to create HNSW index on board_persistent_topics.embedding: %v", err)
		}
	}
}
