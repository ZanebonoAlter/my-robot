package models

import "time"

// ── preference-vector-feed-discovery：偏好向量 / RSSHub 路由目录 / 订阅源推荐 ──
//
// pgvector 列写法沿用 topic_tag_embeddings（EmbeddingVec string + type:vector + column:embedding），
// Dimension/Model 入库以便重算/粗筛前校验同维同模型。jsonb 列用 MetadataMap（已实现 Value/Scan）。

// PreferenceVector 是按 SemanticBoard（board_id=NULL 表示全局桶）聚合的偏好向量。
// source=behavior 由 scheduler 全量重算（不覆盖 seed 行）；source=seed 由问答加权合并累积。
// UNIQUE(board_id, source)：board_id 非 NULL 组合由 GORM uniqueIndex 保证；
// 全局桶（board_id IS NULL）单行由 service 层 upsert 保证（PG 普通 unique 允许多 NULL）。
type PreferenceVector struct {
	ID             uint        `gorm:"primaryKey" json:"id"`
	BoardID        *uint       `gorm:"index;uniqueIndex:idx_preference_vectors_board_source" json:"board_id,omitempty"`
	Source         string      `gorm:"size:20;uniqueIndex:idx_preference_vectors_board_source" json:"source"` // behavior | seed
	EmbeddingVec   string      `gorm:"type:vector;column:embedding" json:"-"`
	Dimension      int         `json:"dimension"`
	Model          string      `gorm:"size:50" json:"model"`
	TagWeights     MetadataMap `gorm:"type:jsonb;serializer:json;default:'{}'" json:"tag_weights"` // {tag_label: weight} top 列表
	LastComputedAt time.Time   `json:"last_computed_at"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`

	Board *SemanticLabel `gorm:"foreignKey:BoardID" json:"board,omitempty"`
}

func (PreferenceVector) TableName() string { return "preference_vectors" }

// RSSHubRoute 是从自建 RSSHub 实例 /api/namespace 同步的路由元数据。
// requires_parameters/usable_directly 在入库时按 path 参数段解析（D3）。
// content_hash 用于增量 diff；status 标可用性校验结果。
type RSSHubRoute struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	Namespace          string     `gorm:"size:100;uniqueIndex:idx_rsshub_routes_ns_path" json:"namespace"`
	Path               string     `gorm:"size:255;uniqueIndex:idx_rsshub_routes_ns_path" json:"path"`
	Name               string     `gorm:"size:255" json:"name"`
	URL                string     `gorm:"type:text" json:"url"`
	Description        string     `gorm:"type:text" json:"description"`
	Parameters         string     `gorm:"type:jsonb;column:parameters" json:"parameters"` // 原始 JSON（数组/对象）
	Example            string     `gorm:"type:text" json:"example"`
	RequiresParameters bool       `json:"requires_parameters"`
	UsableDirectly     bool       `json:"usable_directly"`
	ContentHash        string     `gorm:"size:64;index" json:"content_hash"`             // D2 diff
	Status             string     `gorm:"size:20;index;default:'unknown'" json:"status"` // unknown | ok | broken | gone
	LastCheckedAt      *time.Time `json:"last_checked_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (RSSHubRoute) TableName() string { return "rsshub_routes" }

// RouteEmbedding 存路由的语义向量（文本取 namespace+name+description 摘要）。
// UNIQUE(route_id)：单路由单向量；text_hash 变更入队重算。
type RouteEmbedding struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RouteID      uint      `gorm:"uniqueIndex:idx_route_embeddings_route" json:"route_id"`
	EmbeddingVec string    `gorm:"type:vector;column:embedding" json:"-"`
	Dimension    int       `json:"dimension"`
	Model        string    `gorm:"size:50" json:"model"`
	TextHash     string    `gorm:"size:64;index" json:"text_hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Route *RSSHubRoute `gorm:"foreignKey:RouteID;constraint:OnDelete:CASCADE" json:"route,omitempty"`
}

func (RouteEmbedding) TableName() string { return "route_embeddings" }

// FeedRecommendation 是订阅源推荐卡片。
// recommendation_hash = route_id+board_id（不含 source），qa/manual_refresh 共享幂等池与 dismiss 冷却池（D5/D6）。
type FeedRecommendation struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	RouteID            uint       `gorm:"index;index:idx_feed_rec_status" json:"route_id"`
	BoardID            *uint      `gorm:"index" json:"board_id,omitempty"`
	Source             string     `gorm:"size:20" json:"source"` // manual_refresh | qa
	Score              float64    `gorm:"index:idx_feed_rec_status" json:"score"`
	LLMReason          string     `gorm:"type:text" json:"llm_reason"`
	Status             string     `gorm:"size:20;index:idx_feed_rec_status;default:'pending'" json:"status"` // pending | accepted | dismissed
	AcceptedFeedID     *uint      `gorm:"index" json:"accepted_feed_id,omitempty"`
	RecommendationHash string     `gorm:"size:64;uniqueIndex:idx_feed_recommendations_hash" json:"recommendation_hash"`
	DismissedAt        *time.Time `json:"dismissed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	Route        *RSSHubRoute   `gorm:"foreignKey:RouteID" json:"route,omitempty"`
	Board        *SemanticLabel `gorm:"foreignKey:BoardID" json:"board,omitempty"`
	AcceptedFeed *Feed          `gorm:"foreignKey:AcceptedFeedID" json:"accepted_feed,omitempty"`
}

func (FeedRecommendation) TableName() string { return "feed_recommendations" }
