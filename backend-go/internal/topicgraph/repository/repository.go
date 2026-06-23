package repository

import (
	"gorm.io/gorm"
)

// Repo is the package-level repository singleton.
var Repo *TopicGraphRepository

// InitRepository creates and stores the global repository using the given DB connection.
// Must be called once at startup before any handlers use Repo.
func InitRepository(db *gorm.DB) {
	Repo = NewTopicGraphRepository(db)
}

// TopicGraphRepository provides data access methods for topic graph, daily reports,
// and related entities. It replaces direct database.DB access.
type TopicGraphRepository struct {
	db *gorm.DB
}

// NewTopicGraphRepository creates a new repository with the given DB connection.
func NewTopicGraphRepository(db *gorm.DB) *TopicGraphRepository {
	return &TopicGraphRepository{db: db}
}

// DB returns the underlying *gorm.DB for ad-hoc queries.
func (r *TopicGraphRepository) DB() *gorm.DB {
	return r.db
}
