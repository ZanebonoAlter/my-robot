package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
)

func setupTaggerEmbeddingTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	InitRepository(db)
	t.Cleanup(func() {

	})

	if err := Repo.DB().AutoMigrate(
		&models.TopicTag{},
		&models.TopicTagRelation{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
}

func TestShouldDeleteAbstractChildEmbeddingPreservesNormalChildWithAbstractSibling(t *testing.T) {
	t.Skip("shouldDeleteAbstractChildEmbedding removed — abstract child creation path removed")
}

func TestShouldDeleteAbstractChildEmbeddingDeletesNormalChildWithoutAbstractSibling(t *testing.T) {
	t.Skip("shouldDeleteAbstractChildEmbedding removed — abstract child creation path removed")
}
