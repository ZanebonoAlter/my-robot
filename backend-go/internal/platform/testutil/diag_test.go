package testutil

import (
	"testing"

	"gorm.io/gorm"
	"syntopica-backend/internal/models"
)

// Regression guard: inserting into a vector-bearing table must succeed across
// multiple ReimportTestDB cycles.
//
// Background: ReimportTestDB does DROP SCHEMA public CASCADE, which recreates
// the pgvector `vector` type with a new oid each cycle. If the connection pool
// is reused, its cached server-side prepared statements reference the old oid
// and vector-column inserts fail with `cache lookup failed for type <old-oid>`
// or `cached plan must not change result type`. ReimportTestDB now reopens the
// pool after each rebuild; this test pins that behavior.
//
// Run: go test ./internal/platform/testutil/ -run TestReimportPreservesVectorInserts -v -count=1
func TestReimportPreservesVectorInserts(t *testing.T) {
	db := SetupTestDB(t)

	for cycle := 1; cycle <= 4; cycle++ {
		if cycle > 1 {
			// Use the returned fresh connection (ReimportTestDB closes the old pool).
			db = ReimportTestDB(t, db)
		}
		if err := insertBoard(t, db, cycle); err != nil {
			t.Fatalf("cycle %d: vector-bearing insert failed after reimport: %v", cycle, err)
		}
		// Sanity: the vector type still resolves a cast on the fresh connection.
		var v string
		if err := db.Raw("SELECT '[0]'::vector::text").Row().Scan(&v); err != nil {
			t.Fatalf("cycle %d: '[0]'::vector cast failed: %v", cycle, err)
		}
	}
}

func insertBoard(t *testing.T, db *gorm.DB, cycle int) error {
	t.Helper()
	return db.Create(&models.SemanticLabel{
		Label:     "regression-board",
		Slug:      "regression-board",
		LabelType: "board",
		Status:    "active",
	}).Error
}
