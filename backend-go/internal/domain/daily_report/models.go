package daily_report

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
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
type SectionRelation struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	FromSectionID uint      `gorm:"not null;index:idx_section_relations_from" json:"from_section_id"`
	ToSectionID   uint      `gorm:"not null;index:idx_section_relations_to" json:"to_section_id"`
	Distance      float64   `gorm:"not null" json:"distance"`
	CreatedAt     time.Time `json:"created_at"`
}

func (SectionRelation) TableName() string {
	return "daily_report_section_relations"
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
	AvgScore      float64             `gorm:"default:0" json:"avg_score"`
	Embedding     string              `gorm:"type:vector" json:"-"`
	CreatedAt     time.Time           `json:"created_at"`
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
func floatsToPgVector(v []float64) string {
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
}

// ClusterGroup represents a group of tags clustered by the LLM.
type ClusterGroup struct {
	GroupName string `json:"group_name"`
	TagIDs    []uint `json:"tag_ids"`
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

// ensureSectionEmbeddingDimension sets the daily_report_sections.embedding column to
// the correct vector dimension and creates the HNSW index if dimension ≤ 2000.
// Called once at startup via tagging.RegisterVectorDimEnsurer.
func ensureSectionEmbeddingDimension(dim int) {
	db := database.DB
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
