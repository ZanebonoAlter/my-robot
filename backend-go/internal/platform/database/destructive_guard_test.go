package database_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/config"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"

	// Side-effect imports: trigger init() that registers domain models via
	// database.RegisterModels, so RunAutoMigrate creates their tables
	// (topic_enrichment_result, topic_lifeline_context, etc.) — the targets of
	// the destructive migrations under test.
	_ "syntopica-backend/internal/dataenrichment/repository"
	_ "syntopica-backend/internal/topicgraph/repository"
)

// These integration tests guard the db-migration-safety capability: destructive
// migrations (TRUNCATE) must self-skip when MIGRATIONS_ALLOW_DESTRUCTIVE is unset
// (production default), and execute only when explicitly enabled.
//
// They run against a throwaway testcontainer (via testutil.OpenTestDB), so Docker
// must be available. Skipped under -short.

// runMigrationsWithGate runs the production migration path (AutoMigrate +
// RunMigrations) with the destructive-migration gate in a controlled state. It
// uses OpenTestDB + manual migration invocation (NOT testutil.runTestMigrations,
// which force-enables the gate), so each test controls the gate via config.
//
// seedRow inserts a probe row into topic_enrichment_result after AutoMigrate
// creates the table but before RunMigrations. Whether the row survives depends
// on whether the destructive TRUNCATE migrations (20260712/0718) run.
//
// Returns the *gorm.DB for follow-up assertions (does NOT set database.DB global).
func runMigrationsWithGate(t *testing.T, allowDestructive bool, seedRow bool) *gorm.DB {
	t.Helper()
	db := testutil.OpenTestDB(t)

	// Enable pgvector (mirrors first production migration) + AutoMigrate.
	require.NoError(t, db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error)
	require.NoError(t, database.RunAutoMigrate(db))

	if seedRow {
		// topic_enrichment_result is created by AutoMigrate. Insert a probe row;
		// its survival depends on the destructive TRUNCATE gate.
		require.NoError(t, db.Exec(`INSERT INTO topic_enrichment_result (persistent_topic_id, created_at) VALUES (999, NOW())`).Error,
			"seed probe row into topic_enrichment_result")
	}

	// Set the gate as RunMigrations reads it from config.AppConfig.
	config.AppConfig = &config.Config{Database: config.DatabaseConfig{AllowDestructiveMigrations: allowDestructive}}
	require.NoError(t, database.RunMigrations(db))
	return db
}

// TestDestructiveMigrationSkipsWhenGateClosed verifies the production-safe default:
// with the gate closed, the TRUNCATE migrations self-skip and seed data survives.
func TestDestructiveMigrationSkipsWhenGateClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := runMigrationsWithGate(t, false /* allowDestructive */, true /* seedRow */)

	// Gate closed → destructive migrations skipped → probe row survives.
	var count int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM topic_enrichment_result").Scan(&count).Error)
	require.Equal(t, 1, count, "destructive migration must skip when gate closed (MIGRATIONS_ALLOW_DESTRUCTIVE unset); probe row should survive")

	// Skipped migrations are still marked applied (avoid repeat warn on next startup).
	var versions []string
	require.NoError(t, db.Raw("SELECT version FROM schema_migrations WHERE version IN ('20260706_0001','20260712_0001','20260718_0001') ORDER BY version").Scan(&versions).Error)
	require.Equal(t, []string{"20260706_0001", "20260712_0001", "20260718_0001"}, versions,
		"destructive migrations must be marked applied even when skipped")
}

// TestDestructiveMigrationRunsWhenGateOpen verifies the dev/test path: with the
// gate open, TRUNCATE executes and seed data is cleared.
func TestDestructiveMigrationRunsWhenGateOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := runMigrationsWithGate(t, true /* allowDestructive */, true /* seedRow */)

	// The container is a process singleton shared with the gate-closed test, which
	// already marked the 3 destructive versions as applied. Re-run RunMigrations
	// after deleting those version records so the gate-open path actually executes
	// the TRUNCATE closures (RunMigrations skips already-applied versions).
	require.NoError(t, db.Exec("DELETE FROM schema_migrations WHERE version IN ('20260706_0001','20260712_0001','20260718_0001')").Error)
	config.AppConfig = &config.Config{Database: config.DatabaseConfig{AllowDestructiveMigrations: true}}
	require.NoError(t, database.RunMigrations(db))

	// Gate open → destructive migrations ran → probe row cleared by TRUNCATE.
	var count int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM topic_enrichment_result").Scan(&count).Error)
	require.Equal(t, 0, count, "destructive migration must run when gate open (MIGRATIONS_ALLOW_DESTRUCTIVE=1); probe row should be truncated")
}
