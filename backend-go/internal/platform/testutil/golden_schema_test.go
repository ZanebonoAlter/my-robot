package testutil

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/models"
)

// SetupTestDB must build the golden schema (runTestMigrations) only ONCE per
// process; subsequent calls reset via ResetTestData without rebuilding.
func TestSetupTestDB_MigratesOncePerProcess(t *testing.T) {
	// Force migrateOnce to fire (if it hasn't already in this process), then
	// measure: 3 more SetupTestDB calls must not re-run migrations.
	SetupTestDB(t)
	before := atomic.LoadInt64(&migrationsRunCount)

	SetupTestDB(t)
	SetupTestDB(t)
	SetupTestDB(t)

	after := atomic.LoadInt64(&migrationsRunCount)
	require.Equal(t, before, after,
		"runTestMigrations must not re-run after the golden schema is built; delta=%d", after-before)
}

// After the golden schema is built, repeated ResetTestData cycles must NOT
// trigger the pgvector OID-drift bug (no DROP SCHEMA => stable OID).
func TestGoldenSchema_OIDStableAcrossResets(t *testing.T) {
	db := SetupTestDB(t)

	for cycle := 1; cycle <= 5; cycle++ {
		require.NoError(t, db.Create(&models.SemanticLabel{
			Label:     "oid-stability-probe",
			Slug:      "oid-stability-probe",
			LabelType: "board",
			Status:    "active",
		}).Error, "cycle %d: vector-bearing insert must succeed", cycle)

		// Vector cast must resolve (OID not broken).
		var v string
		require.NoError(t, db.Raw("SELECT '[0]'::vector::text").Row().Scan(&v),
			"cycle %d: vector cast failed => OID drift", cycle)

		db = ResetTestData(t, db)
	}
}
