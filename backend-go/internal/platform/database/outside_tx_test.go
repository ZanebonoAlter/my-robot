package database_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"
)

// These integration tests guard the db-migration-execution capability's
// "outside-tx execution" path (D-High-3): a migration declaring RunOutsideTx=true
// runs without an outer transaction (so CREATE INDEX CONCURRENTLY and other
// transaction-incompatible DDL can succeed). The framework must:
//   - really execute Up outside a transaction (pg_is_in_transaction() == false),
//   - NOT record the version when Up fails (so the next startup retries), and
//   - record the version when Up succeeds.
//
// They run against a throwaway testcontainer (via testutil.OpenTestDB), so Docker
// must be available. Skipped under -short.
//
// The tests inject probe migrations by calling the exported runMigrationsList
// (database.RunMigrationsList) directly with a hand-built []Migration, rather
// than mutating the production postgresMigrations() slice.

// TestRunOutsideTx_NotInTransaction verifies that a migration with
// RunOutsideTx=true actually runs outside any outer transaction block. We prove
// this with the canonical PostgreSQL test: CREATE INDEX CONCURRENTLY, which
// errors with SQLSTATE 25001 ("cannot run inside a transaction block") when a
// surrounding transaction exists. So if the probe Up closure can run
// CONCURRENTLY without error, the migration is genuinely outside a transaction.
// (PostgreSQL has no `pg_is_in_transaction()` function — even in PG18 the
// reliably distinguishable behavior is the CONCURRENTLY-eligibility check, and
// that is precisely the capability D-High-3 needs to unlock.)
func TestRunOutsideTx_NotInTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)

	// Create a throwaway table the probe can index. Done outside the migration
	// so the probe is a pure "am I in a tx?" check.
	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS outside_tx_probe (id int)").Error)
	defer func() {
		_ = db.Exec("DROP TABLE IF EXISTS outside_tx_probe").Error
	}()

	probe := database.Migration{
		Version:      "99990101_0001",
		Description:  "test: assert migration runs outside a transaction (CONCURRENTLY must succeed)",
		RunOutsideTx: true,
		Up: func(tx *gorm.DB) error {
			// CREATE INDEX CONCURRENTLY fails with SQLSTATE 25001 inside a tx
			// block; success here proves Up is running outside one.
			return tx.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS outside_tx_probe_idx ON outside_tx_probe(id)").Error
		},
	}

	require.NoError(t, database.RunMigrationsList(db, []database.Migration{probe}),
		"RunOutsideTx=true migration must run outside a transaction so CONCURRENTLY DDL succeeds; "+
			"an error here means Up was incorrectly wrapped in a transaction block")

	// Cleanup the recorded version so the probe doesn't linger for other tests
	// on this shared container.
	require.NoError(t, db.Exec("DROP INDEX IF EXISTS outside_tx_probe_idx").Error)
	require.NoError(t, db.Exec("DELETE FROM schema_migrations WHERE version = '99990101_0001'").Error)
}

// TestRunOutsideTx_FailureNotRecorded verifies the retry invariant: when a
// RunOutsideTx=true migration's Up returns an error, the version must NOT be
// recorded in schema_migrations. This mirrors the in-tx path (where the whole
// transaction — Up + INSERT — rolls back on Up failure). The next startup must
// retry the migration. This is essential for CREATE INDEX CONCURRENTLY, which
// can leave an INVALID index on failure and must be retried (after the
// migration closure cleans up the INVALID residue).
func TestRunOutsideTx_FailureNotRecorded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)

	probe := database.Migration{
		Version:      "99990101_0002",
		Description:  "test: outside-tx Up failure must not record version",
		RunOutsideTx: true,
		Up: func(tx *gorm.DB) error {
			return errors.New("simulated outside-tx failure")
		},
	}

	err := database.RunMigrationsList(db, []database.Migration{probe})
	require.Error(t, err, "outside-tx migration with failing Up must return an error")

	var count int
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = '99990101_0002'").Scan(&count).Error)
	require.Equal(t, 0, count,
		"failed outside-tx migration must not be recorded (must be retriable on next startup)")
}

// TestRunOutsideTx_SuccessRecorded verifies the happy path: when a
// RunOutsideTx=true migration's Up succeeds, the version IS recorded and the
// side effects of Up persist (no surrounding transaction to roll them back).
func TestRunOutsideTx_SuccessRecorded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)

	probe := database.Migration{
		Version:      "99990101_0003",
		Description:  "test: outside-tx Up success must record version + persist side effects",
		RunOutsideTx: true,
		Up: func(tx *gorm.DB) error {
			return tx.Exec("CREATE TABLE IF NOT EXISTS outside_tx_probe (id int)").Error
		},
	}

	require.NoError(t, database.RunMigrationsList(db, []database.Migration{probe}))

	// Version recorded.
	var count int
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = '99990101_0003'").Scan(&count).Error)
	require.Equal(t, 1, count, "successful outside-tx migration must be recorded")

	// Side effect persisted (table exists).
	var exists bool
	require.NoError(t, db.Raw("SELECT to_regclass('public.outside_tx_probe') IS NOT NULL").Row().Scan(&exists))
	require.True(t, exists, "outside-tx migration side effect (CREATE TABLE) must persist")

	// Cleanup.
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS outside_tx_probe").Error)
	require.NoError(t, db.Exec("DELETE FROM schema_migrations WHERE version = '99990101_0003'").Error)
}
