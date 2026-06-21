package repository

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: build a 3-dim unit-ish vector pgvector string.
func vecStr(v ...float64) string { return FloatsToPgVector(v) }

func TestCosineDistance_OrthogonalIsOne(t *testing.T) {
	// [1,0,0] vs [0,1,0] → similarity 0 → distance 1.
	d := cosineDistance([]float64{1, 0, 0}, []float64{0, 1, 0})
	assert.InDelta(t, 1.0, d, 1e-9)
}

func TestCosineDistance_IdenticalIsZero(t *testing.T) {
	d := cosineDistance([]float64{0.3, 0.7, -0.2}, []float64{0.3, 0.7, -0.2})
	assert.InDelta(t, 0.0, d, 1e-9)
}

func TestCosineDistance_MismatchedLengthIsInf(t *testing.T) {
	assert.True(t, math.IsInf(cosineDistance([]float64{1, 0}, []float64{1, 0, 0}), 1) || math.IsInf(cosineDistance([]float64{1, 0}, []float64{1, 0, 0}), 0) || cosineDistance([]float64{1, 0}, []float64{1, 0, 0}) == math.MaxFloat64)
}

func TestPlanTopicAssignments_AnchorHit(t *testing.T) {
	// Existing topic at [1,0,0]; section at [0.99,0.01,0] is near (dist ~0.0),
	// and the LLM agrees by setting MatchedTopicID to that topic.
	mit := uint(12)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	topics := []BoardPersistentTopic{{ID: 12, Embedding: vecStr(1, 0, 0), Status: TopicStatusActive}}
	sections := []DailyReportSection{{
		ClusterLabel: "AI 编程竞争", Embedding: vecStr(0.99, 0.01, 0),
		MatchedTopicID: &mit,
	}}
	dec := planTopicAssignments(sections, topics, cfg, time.Now())
	require.Len(t, dec, 1)
	assert.Equal(t, TopicConfAnchorHit, dec[0].confidence)
	assert.Equal(t, uint(12), dec[0].topicID)
	assert.Nil(t, dec[0].newCandidate)
}

func TestPlanTopicAssignments_AutoNew_DualConfirmationFail(t *testing.T) {
	// Section is embedding-close to topic 12 (dist < threshold), but the LLM
	// did NOT mark it (MatchedTopicID points elsewhere). Dual confirmation fails
	// → a new candidate must be opened, NOT an anchor hit.
	other := uint(99)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	topics := []BoardPersistentTopic{{ID: 12, Embedding: vecStr(1, 0, 0), Status: TopicStatusActive}}
	sections := []DailyReportSection{{
		ClusterLabel: "开发者生态重构", Embedding: vecStr(0.99, 0.01, 0),
		MatchedTopicID: &other, // LLM disagrees
	}}
	dec := planTopicAssignments(sections, topics, cfg, time.Now())
	require.Len(t, dec, 1)
	assert.Equal(t, TopicConfAutoNew, dec[0].confidence)
	assert.NotNil(t, dec[0].newCandidate)
	assert.Equal(t, "开发者生态重构", dec[0].newCandidate.label)
}

func TestPlanTopicAssignments_AutoNew_DistanceExceedsThreshold(t *testing.T) {
	// Section is far from every topic (orthogonal, dist=1 > 0.30) → auto_new
	// even though there is no MatchedTopicID to disagree with.
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	topics := []BoardPersistentTopic{{ID: 12, Embedding: vecStr(1, 0, 0), Status: TopicStatusActive}}
	sections := []DailyReportSection{{
		ClusterLabel: "量子计算商用", Embedding: vecStr(0, 1, 0),
	}}
	dec := planTopicAssignments(sections, topics, cfg, time.Now())
	require.Len(t, dec, 1)
	assert.Equal(t, TopicConfAutoNew, dec[0].confidence)
	assert.NotNil(t, dec[0].newCandidate)
}

func TestPlanTopicAssignments_Unmatched_EmptyEmbedding(t *testing.T) {
	cfg := DefaultPersistentTopicConfig()
	sections := []DailyReportSection{{ClusterLabel: "no embedding", Embedding: ""}}
	dec := planTopicAssignments(sections, nil, cfg, time.Now())
	require.Len(t, dec, 1)
	assert.Equal(t, TopicConfUnmatched, dec[0].confidence)
	assert.Nil(t, dec[0].newCandidate)
}

func TestPlanLifecycle_UpgradeOnConsecutiveHits(t *testing.T) {
	// candidate at consecutive=2, hit today → promoted to active (3 ≥ 3).
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 2, HitCount: 2,
		LastSeenDate: time.Now().AddDate(0, 0, -1),
	}}
	changes := planLifecycle(topics, time.Now(), map[uint]bool{1: true}, cfg)
	require.Len(t, changes, 1)
	assert.Equal(t, TopicStatusActive, changes[0].status)
	assert.Equal(t, 3, changes[0].consecutiveHits)
	assert.Equal(t, 3, changes[0].hitCount)
}

func TestPlanLifecycle_HitKeepsCandidateBelowThreshold(t *testing.T) {
	// candidate at consecutive=1, hit today → stays candidate (2 < 3).
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 1, HitCount: 1,
		LastSeenDate: time.Now().AddDate(0, 0, -1),
	}}
	changes := planLifecycle(topics, time.Now(), map[uint]bool{1: true}, cfg)
	require.Len(t, changes, 1)
	assert.Equal(t, TopicStatusCandidate, changes[0].status)
	assert.Equal(t, 2, changes[0].consecutiveHits)
}

func TestPlanLifecycle_MissResetsConsecutive(t *testing.T) {
	// candidate not hit today → consecutive resets to 0; stays candidate.
	// (No change row produced because only hits and active-decay emit rows.)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 2, HitCount: 2,
		LastSeenDate: time.Now().AddDate(0, 0, -1),
	}}
	changes := planLifecycle(topics, time.Now(), map[uint]bool{}, cfg)
	// candidate miss emits no row (reset is implicit; no status transition).
	assert.Len(t, changes, 0)
}

func TestPlanLifecycle_ArchiveOnDecay(t *testing.T) {
	// active not hit for 35 days (> decay 30) → archived.
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	today := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusActive, ConsecutiveHits: 5, HitCount: 10,
		LastSeenDate: today.AddDate(0, 0, -35),
	}}
	changes := planLifecycle(topics, today, map[uint]bool{}, cfg)
	require.Len(t, changes, 1)
	assert.Equal(t, TopicStatusArchived, changes[0].status)
}

func TestPlanLifecycle_KeepWithinDecayWindow(t *testing.T) {
	// active not hit for 14 days (< decay 30) → stays active, no change row.
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3, DecayWindow: 30}
	today := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusActive, ConsecutiveHits: 5, HitCount: 10,
		LastSeenDate: today.AddDate(0, 0, -14),
	}}
	changes := planLifecycle(topics, today, map[uint]bool{}, cfg)
	assert.Len(t, changes, 0)
}
