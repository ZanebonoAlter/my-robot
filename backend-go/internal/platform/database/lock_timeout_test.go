package database_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"
)

// These integration tests guard the db-migration-execution capability's
// "lock-timeout guard" helper (D-High-4, migration layer): withLockTimeout must
// (a) really set the lock_timeout GUC for the duration of fn, and (b) reset it
// to DEFAULT after fn returns (so a subsequent statement in the same
// transaction is no longer constrained — defensive against GUC leakage when
// the helper is ever reused on a pooled/bare connection).
//
// SET LOCAL only takes effect inside a transaction block, so the tests wrap
// each probe in db.Transaction — mirroring how the in-tx migration path runs.
//
// They run against a throwaway testcontainer (via testutil.OpenTestDB), so
// Docker must be available. Skipped under -short.

// showLockTimeout returns the current session lock_timeout GUC value within tx.
func showLockTimeout(tx *gorm.DB) string {
	var lt string
	if err := tx.Raw("SHOW lock_timeout").Row().Scan(&lt); err != nil {
		return ""
	}
	return lt
}

// TestWithLockTimeout_SetsGUC verifies that inside fn the lock_timeout GUC is
// set to the requested value. The guard is only meaningful if ALTER/ADD
// CONSTRAINT statements actually run under the reduced timeout.
func TestWithLockTimeout_SetsGUC(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)

	var observed string
	err := db.Transaction(func(tx *gorm.DB) error {
		return database.WithLockTimeout(tx, "3s", func(inner *gorm.DB) error {
			observed = showLockTimeout(inner)
			return nil
		})
	})
	require.NoError(t, err)
	require.Equal(t, "3s", observed,
		"withLockTimeout must set lock_timeout GUC to the requested value for the duration of fn")
}

// TestWithLockTimeout_ResetAfterCall verifies that after withLockTimeout
// returns, the same transaction's lock_timeout is back to its DEFAULT (not the
// reduced value). This defends against GUC leakage: even if the helper is later
// reused on a bare/pooled connection (no surrounding transaction to auto-reset
// SET LOCAL), the explicit reset prevents the reduced timeout from leaking
// into subsequent statements.
func TestWithLockTimeout_ResetAfterCall(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)

	// Capture the session's baseline (default) lock_timeout before the guard.
	var baseline string
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		baseline = showLockTimeout(tx)
		return nil
	}))
	require.NotEmpty(t, baseline, "sanity: must read a baseline lock_timeout")

	// Run the guard inside a transaction, then read lock_timeout again in the
	// SAME transaction after the helper returns. It must match the baseline,
	// not the "2s" we just set.
	var afterReset string
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := database.WithLockTimeout(tx, "2s", func(inner *gorm.DB) error {
			// Confirm the guard really applied during fn (otherwise the reset
			// assertion is vacuous).
			require.Equal(t, "2s", showLockTimeout(inner),
				"guard must apply during fn for the reset assertion to be meaningful")
			return nil
		}); err != nil {
			return err
		}
		afterReset = showLockTimeout(tx)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, baseline, afterReset,
		"withLockTimeout must reset lock_timeout to DEFAULT after fn returns (got %q, want baseline %q); "+
			"the reduced timeout must not leak into subsequent statements", afterReset, baseline)
}
