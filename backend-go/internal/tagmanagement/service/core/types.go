package core


type ExtractionInput struct {
	Title        string
	Summary      string
	FeedName     string
	CategoryName string
	ArticleID    *uint
	SummaryID    *uint
	PubDate      string
}

// TopicTag represents a tag extracted from AI summaries
// Used for API responses and internal processing
type AuxiliaryLabel struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type TopicTag struct {
	ID              uint             `json:"id,omitempty"`
	Label           string           `json:"label"`
	Slug            string           `json:"slug"`
	Category        string           `json:"category"`              // event, person, keyword
	Icon            string           `json:"icon,omitempty"`        // Iconify icon id
	Aliases         []string         `json:"aliases,omitempty"`     // Alternative names
	Description     string           `json:"description,omitempty"` // LLM-generated tag description
	AuxiliaryLabels []AuxiliaryLabel `json:"auxiliary_labels,omitempty"`
	Score           float64          `json:"score"`
	IsNew           bool             `json:"is_new,omitempty"`      // True if newly created
	MatchedTo       uint             `json:"matched_to,omitempty"`  // ID of existing tag if matched
	Kind            string           `json:"kind,omitempty"`        // Legacy: maps to Category for backward compat
	FeedCount       int              `json:"feed_count,omitempty"`  // Distinct feed count referencing this tag
	IsAbstract      bool             `json:"is_abstract,omitempty"` // True if this is an abstract (parent) tag
	ChildSlugs      []string         `json:"child_slugs,omitempty"` // Slugs of child tags (only for abstract tags)
	QualityScore    float64          `json:"quality_score,omitempty"`
	IsLowQuality    bool             `json:"is_low_quality,omitempty"`
	IsWatched       bool             `json:"is_watched,omitempty"`
	ArticleCount    int              `json:"article_count,omitempty"`
}

type AggregatedTopicTag struct {
	Slug         string   `json:"slug"`
	Label        string   `json:"label"`
	Category     string   `json:"category"`
	Kind         string   `json:"kind,omitempty"`
	Icon         string   `json:"icon,omitempty"`
	Aliases      []string `json:"aliases,omitempty"`
	Score        float64  `json:"score"`
	ArticleCount int      `json:"article_count"`
	FeedCount    int      `json:"feed_count,omitempty"`
}

// ExtractedTag is the raw output from AI extraction
type ExtractedTag struct {
	Label           string           `json:"label"`
	Category        string           `json:"category"` // event, person, keyword
	Aliases         []string         `json:"aliases,omitempty"`
	Description     string           `json:"description,omitempty"`
	AuxiliaryLabels []AuxiliaryLabel `json:"auxiliary_labels,omitempty"`
}

// TagResolutionRequest is sent to AI for ambiguous tag matching
type TagResolutionRequest struct {
	CandidateTag   ExtractedTag     `json:"candidate_tag"`
	SimilarTags    []SimilarTagInfo `json:"similar_tags"`
	SummaryContext string           `json:"summary_context"`
}

// SimilarTagInfo provides context about similar existing tags
type SimilarTagInfo struct {
	ID         uint     `json:"id"`
	Label      string   `json:"label"`
	Category   string   `json:"category"`
	Aliases    []string `json:"aliases"`
	Similarity float64  `json:"similarity"`
	UsageCount int      `json:"usage_count,omitempty"`
	FeedCount  int      `json:"feed_count,omitempty"`
}

// TagResolutionResponse is AI's decision on tag matching
type TagResolutionResponse struct {
	Decision    string `json:"decision"` // "reuse" or "create_new"
	ReuseTagID  uint   `json:"reuse_tag_id,omitempty"`
	Reason      string `json:"reason"`
	NewLabel    string `json:"new_label,omitempty"` // Fine-tuned label if creating
	NewCategory string `json:"new_category,omitempty"`
}

// TopicTagSummary represents a brief tag reference on an article card
type TopicTagSummary struct {
	Slug     string `json:"slug"`
	Label    string `json:"label"`
	Category string `json:"category"`
}

// GetTopicArticlesParams holds query parameters for GetTopicArticles API
type GetTopicArticlesParams struct {
	Page       int    `form:"page" binding:"min=1"`
	PageSize   int    `form:"page_size" binding:"min=1,max=100"`
	WindowType string `form:"type" binding:"oneof=daily weekly"`
	AnchorDate string `form:"date"`
}

// PendingArticle represents an article that has a tag but is not yet in any digest
type PendingArticle struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	PubDate   string `json:"pub_date,omitempty"`
	FeedName  string `json:"feed_name"`
	FeedIcon  string `json:"feed_icon,omitempty"`
	FeedColor string `json:"feed_color,omitempty"`
}

type SimilarityEdge struct {
	TagAID     uint
	TagBID     uint
	Similarity float64
}
