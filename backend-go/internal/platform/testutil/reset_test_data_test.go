package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
)

// ResetTestData must: (a) truncate all business tables, (b) restore migration
// seed rows from the golden-schema snapshot, (c) clear custom ai_settings keys
// that tests inserted, (d) preserve schema_migrations.
func TestResetTestData_ClearsBusinessTables(t *testing.T) {
	db := SetupTestDB(t)

	require.NoError(t, db.Create(&models.TopicTag{Slug: "tt-1"}).Error)
	var n int64
	db.Model(&models.TopicTag{}).Count(&n)
	require.EqualValues(t, 1, n)

	db = ResetTestData(t, db)

	db.Model(&models.TopicTag{}).Count(&n)
	require.EqualValues(t, 0, n, "ResetTestData must clear business tables")
}

func TestResetTestData_RestoresSeedAndClearsCustomKeys(t *testing.T) {
	db := SetupTestDB(t)

	// Simulate a test inserting a non-seed custom ai_settings key.
	// The probe key is deliberately NOT one of the migration seed keys
	// (seeded keys are semantic_board_{match,upgrade}_*), so the INSERT
	// below does not collide with uni_ai_settings_key. This mirrors the
	// *intent* of tagmanagement/service/board tests, which also insert
	// custom ai_settings keys (those run after a truncate, so they reuse
	// seed-key names; here we insert against a freshly-seeded table, so we
	// pick a guaranteed-non-seed key).
	const nonSeedKey = "test_nonseed_reset_probe"
	require.NoError(t, db.Create(&models.AISettings{
		Key: nonSeedKey, Value: "0.6",
	}).Error)

	db = ResetTestData(t, db)

	var custom models.AISettings
	err := db.Where("key = ?", nonSeedKey).First(&custom).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound,
		"custom (non-seed) ai_settings key must be cleared by ResetTestData")

	// Seed rows must still be present (restored from snapshot).
	var seedCount int64
	db.Model(&models.AISettings{}).Count(&seedCount)
	require.Greater(t, seedCount, int64(0), "migration seed ai_settings rows must be restored")
}

func TestResetTestData_PreservesSchemaMigrations(t *testing.T) {
	db := SetupTestDB(t)

	db = ResetTestData(t, db)

	var n int64
	db.Raw("SELECT count(*) FROM schema_migrations").Scan(&n)
	require.Greater(t, n, int64(0), "schema_migrations must survive ResetTestData")
}

func TestResetTestData_IdempotentAcrossCalls(t *testing.T) {
	db := SetupTestDB(t)

	db = ResetTestData(t, db)
	var firstCount int64
	db.Model(&models.AISettings{}).Count(&firstCount)

	db = ResetTestData(t, db)
	db = ResetTestData(t, db)
	var finalCount int64
	db.Model(&models.AISettings{}).Count(&finalCount)

	require.Equal(t, firstCount, finalCount,
		"ResetTestData must be idempotent: seed count must not drift across calls")
}
