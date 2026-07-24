package models

import "time"

type SemanticLabel struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Label string `gorm:"size:160" json:"label"`
	Slug  string `gorm:"size:160;uniqueIndex:idx_semantic_labels_slug" json:"slug"`
	// Vector columns are declared without a fixed dimension. The actual dimension is
	// determined at runtime by the configured embedder (see auxlabel.EnsureVectorDimensionOnce)
	// and may differ across deployments, so hardcoding it here would race AutoMigrate.
	Embedding      *string  `gorm:"type:vector;column:embedding" json:"-"`
	MergeEmbedding *string  `gorm:"type:vector;column:merge_embedding" json:"-"`
	LabelType      string   `gorm:"size:20;index:idx_semantic_labels_label_type" json:"label_type"`
	Aliases        []string `gorm:"type:jsonb;serializer:json;default:'[]'" json:"aliases"`
	RefCount       int      `json:"ref_count"`
	Description    string   `gorm:"type:text" json:"description"`
	DisplayOrder   int      `json:"display_order"`
	Source         string   `gorm:"size:50" json:"source"`
	Status         string   `gorm:"size:20;index:idx_semantic_labels_status" json:"status"`
	Protected      bool     `json:"protected"`
	// EnrichmentEnabled — whether cycle-B enrichment is enabled for this board (default false).
	EnrichmentEnabled bool `json:"enrichment_enabled"`
	// WindowDays — real-time detail window for cycle-B (default 14).
	WindowDays int `json:"window_days"`
	// ContextLayers — which granularity layers the interpreter reads (default ["week","month","year","all"]).
	ContextLayers []string  `gorm:"type:jsonb;serializer:json;default:'[\"week\",\"month\",\"year\",\"all\"]'" json:"context_layers"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SemanticLabel) TableName() string {
	return "semantic_labels"
}

type TopicTagSemanticLabel struct {
	TopicTagID      uint `gorm:"primaryKey" json:"topic_tag_id"`
	SemanticLabelID uint `gorm:"primaryKey" json:"semantic_label_id"`

	TopicTag      *TopicTag      `gorm:"foreignKey:TopicTagID;constraint:OnDelete:CASCADE" json:"topic_tag,omitempty"`
	SemanticLabel *SemanticLabel `gorm:"foreignKey:SemanticLabelID;constraint:OnDelete:CASCADE" json:"semantic_label,omitempty"`
}

func (TopicTagSemanticLabel) TableName() string {
	return "topic_tag_semantic_labels"
}

type TopicTagBoardLabel struct {
	TopicTagID        uint      `gorm:"primaryKey" json:"topic_tag_id"`
	SemanticBoardID   uint      `gorm:"primaryKey" json:"semantic_board_id"`
	Score             float64   `json:"score"`
	MatchReason       string    `gorm:"type:text" json:"match_reason"`
	Downgraded        bool      `json:"downgraded"`
	DirectionMismatch bool      `json:"direction_mismatch"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	TopicTag      *TopicTag      `gorm:"foreignKey:TopicTagID;constraint:OnDelete:CASCADE" json:"topic_tag,omitempty"`
	SemanticBoard *SemanticLabel `gorm:"foreignKey:SemanticBoardID;constraint:OnDelete:CASCADE" json:"semantic_board,omitempty"`
}

func (TopicTagBoardLabel) TableName() string {
	return "topic_tag_board_labels"
}

type BoardComposition struct {
	BoardID          uint `gorm:"primaryKey" json:"board_id"`
	AuxiliaryLabelID uint `gorm:"primaryKey" json:"auxiliary_label_id"`

	Board          *SemanticLabel `gorm:"foreignKey:BoardID;constraint:OnDelete:CASCADE" json:"board,omitempty"`
	AuxiliaryLabel *SemanticLabel `gorm:"foreignKey:AuxiliaryLabelID;constraint:OnDelete:CASCADE" json:"auxiliary_label,omitempty"`
}

func (BoardComposition) TableName() string {
	return "board_composition"
}
