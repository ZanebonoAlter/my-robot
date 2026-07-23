package dataenrichment

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"syntopica-backend/internal/platform/database"
)

func setupDataEnrichmentTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.DB = db

	if err := database.RunAutoMigrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
}

func TestBoardDataSourceModelCanAutoMigrate(t *testing.T) {
	setupDataEnrichmentTestDB(t)

	var count int64
	if err := database.DB.Raw("SELECT COUNT(*) FROM board_data_sources").Scan(&count).Error; err != nil {
		t.Fatalf("board_data_sources table should exist: %v", err)
	}
}

func TestTopicLifelineContextModelCanAutoMigrate(t *testing.T) {
	setupDataEnrichmentTestDB(t)

	var count int64
	if err := database.DB.Raw("SELECT COUNT(*) FROM topic_lifeline_context").Scan(&count).Error; err != nil {
		t.Fatalf("topic_lifeline_context table should exist: %v", err)
	}
}

func TestTopicEnrichmentResultModelCanAutoMigrate(t *testing.T) {
	setupDataEnrichmentTestDB(t)

	var count int64
	if err := database.DB.Raw("SELECT COUNT(*) FROM topic_enrichment_result").Scan(&count).Error; err != nil {
		t.Fatalf("topic_enrichment_result table should exist: %v", err)
	}
}

func TestTopicEnrichmentReviewModelCanAutoMigrate(t *testing.T) {
	setupDataEnrichmentTestDB(t)

	var count int64
	if err := database.DB.Raw("SELECT COUNT(*) FROM topic_enrichment_review").Scan(&count).Error; err != nil {
		t.Fatalf("topic_enrichment_review table should exist: %v", err)
	}
}

func TestTopicEnrichmentQAModelCanAutoMigrate(t *testing.T) {
	setupDataEnrichmentTestDB(t)

	var count int64
	if err := database.DB.Raw("SELECT COUNT(*) FROM topic_enrichment_qa").Scan(&count).Error; err != nil {
		t.Fatalf("topic_enrichment_qa table should exist: %v", err)
	}
}

func TestSemanticLabelHasEnrichmentFields(t *testing.T) {
	setupDataEnrichmentTestDB(t)

	hasEnrichmentEnabled := false
	hasWindowDays := false

	rows, err := database.DB.Raw("PRAGMA table_info(semantic_labels)").Rows()
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "enrichment_enabled" {
			hasEnrichmentEnabled = true
		}
		if name == "window_days" {
			hasWindowDays = true
		}
	}
	if !hasEnrichmentEnabled {
		t.Fatal("semantic_labels table missing enrichment_enabled column")
	}
	if !hasWindowDays {
		t.Fatal("semantic_labels table missing window_days column")
	}
}
