package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// TestLoadPersistentTopicConfig_SectionMergeEnabled covers the
// daily_report_section_merge_enabled kill switch (fix-section-merge-blackhole):
// absent row → default false; "true"/"1" → true; invalid → warn + false.
func TestLoadPersistentTopicConfig_SectionMergeEnabled(t *testing.T) {
	seed := func(t *testing.T, db *gorm.DB, value string) {
		t.Helper()
		require.NoError(t, db.Create(&models.AISettings{
			Key:   "daily_report_section_merge_enabled",
			Value: value,
		}).Error)
	}

	t.Run("no row defaults to false", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		cfg := LoadPersistentTopicConfig(db)
		assert.False(t, cfg.SectionMergeEnabled)
	})

	t.Run("true enables merge", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		seed(t, db, "true")
		cfg := LoadPersistentTopicConfig(db)
		assert.True(t, cfg.SectionMergeEnabled)
	})

	t.Run("invalid value warns and keeps default false", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		seed(t, db, "maybe")
		cfg := LoadPersistentTopicConfig(db)
		assert.False(t, cfg.SectionMergeEnabled)
	})
}

// TestLoadPersistentTopicConfig_CandidateGate covers the
// persistent_topic_candidate_l1_gate_enabled switch (candidate-topic-l2-gate):
// absent row → default true (gate on); explicit true/false parse; invalid →
// warn + keep default true.
func TestLoadPersistentTopicConfig_CandidateGate(t *testing.T) {
	seed := func(t *testing.T, db *gorm.DB, value string) {
		t.Helper()
		require.NoError(t, db.Create(&models.AISettings{
			Key:   "persistent_topic_candidate_l1_gate_enabled",
			Value: value,
		}).Error)
	}

	t.Run("no row defaults to true", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		cfg := LoadPersistentTopicConfig(db)
		assert.True(t, cfg.CandidateL1GateEnabled)
	})

	t.Run("explicit false disables gate", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		seed(t, db, "false")
		cfg := LoadPersistentTopicConfig(db)
		assert.False(t, cfg.CandidateL1GateEnabled)
	})

	t.Run("explicit true enables gate", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		seed(t, db, "true")
		cfg := LoadPersistentTopicConfig(db)
		assert.True(t, cfg.CandidateL1GateEnabled)
	})

	t.Run("invalid value warns and keeps default true", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		seed(t, db, "banana")
		cfg := LoadPersistentTopicConfig(db)
		assert.True(t, cfg.CandidateL1GateEnabled)
	})
}
