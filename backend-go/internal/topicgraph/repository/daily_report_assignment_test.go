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

func TestSelectAnchorableTopics_PreservesActiveAndFiltersStaleCandidate(t *testing.T) {
	reportDate := time.Date(2026, 6, 26, 15, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	cfg := PersistentTopicConfig{CandidateDecayWindow: 7, CandidatePromptLimit: 20}
	topics := []BoardPersistentTopic{
		{ID: 1, Status: TopicStatusActive, LastSeenDate: reportDate.AddDate(0, 0, -90)},
		{ID: 2, Status: TopicStatusCandidate, LastSeenDate: reportDate.AddDate(0, 0, -7)},
		{ID: 3, Status: TopicStatusCandidate, LastSeenDate: reportDate.AddDate(0, 0, -8)},
	}

	selected, stats := selectAnchorableTopics(topics, reportDate, cfg)

	require.Equal(t, []uint{1, 2}, []uint{selected[0].ID, selected[1].ID})
	assert.Equal(t, 1, stats.ActiveCount)
	assert.Equal(t, 2, stats.CandidateCount)
	assert.Equal(t, 1, stats.FilteredByWindow)
}

func TestSelectAnchorableTopics_OrdersAndLimitsCandidates(t *testing.T) {
	reportDate := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	cfg := PersistentTopicConfig{CandidateDecayWindow: 7, CandidatePromptLimit: 3}
	topics := []BoardPersistentTopic{
		{ID: 9, Status: TopicStatusCandidate, LastSeenDate: reportDate.AddDate(0, 0, -2), HitCount: 10},
		{ID: 4, Status: TopicStatusCandidate, LastSeenDate: reportDate.AddDate(0, 0, -1), HitCount: 2},
		{ID: 2, Status: TopicStatusCandidate, LastSeenDate: reportDate.AddDate(0, 0, -1), HitCount: 5},
		{ID: 1, Status: TopicStatusCandidate, LastSeenDate: reportDate.AddDate(0, 0, -1), HitCount: 5},
	}

	selected, stats := selectAnchorableTopics(topics, reportDate, cfg)

	require.Len(t, selected, 3)
	assert.Equal(t, []uint{1, 2, 4}, []uint{selected[0].ID, selected[1].ID, selected[2].ID})
	assert.Equal(t, 1, stats.TruncatedByLimit)
}

// TestSelectAnchorableTopics_AgreementWithAssignment verifies the collection
// contract: only topics that survive selectAnchorableTopics can be accepted by
// planTopicAssignments (when MatchedTopicID points to them and embedding is
// within threshold). A candidate filtered by window or truncated by limit is
// absent from the anchorable set, so planTopicAssignments cannot accept it —
// even if its MatchedTopicID points to it. This guarantees ClusterTags
// injection and dual-confirmation assignment share the same topic set.
func TestSelectAnchorableTopics_AgreementWithAssignment(t *testing.T) {
	reportDate := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, CandidateDecayWindow: 7, CandidatePromptLimit: 2}

	// active (id=1), in-window candidate (id=2, gap=5), in-window candidate
	// (id=3, gap=1), stale candidate (id=4, gap=9 > window).
	// With limit=2, only candidates id=3 (newer) and id=2 stay; id=4 is filtered
	// by window.
	topics := []BoardPersistentTopic{
		{ID: 1, Status: TopicStatusActive, Embedding: vecStr(1, 0, 0), LastSeenDate: reportDate.AddDate(0, 0, -90)},
		{ID: 2, Status: TopicStatusCandidate, Embedding: vecStr(0.9, 0.43, 0), LastSeenDate: reportDate.AddDate(0, 0, -5)},
		{ID: 3, Status: TopicStatusCandidate, Embedding: vecStr(0.8, 0.5, 0), LastSeenDate: reportDate.AddDate(0, 0, -1)},
	}

	selected, stats := selectAnchorableTopics(topics, reportDate, cfg)
	require.Equal(t, 3, len(selected)) // active + 2 in-window candidates
	require.Equal(t, 0, stats.FilteredByWindow)
	require.Equal(t, 0, stats.TruncatedByLimit)

	// Build the anchorable ID set for fast lookup.
	anchorableIDs := make(map[uint]bool, len(selected))
	for _, t := range selected {
		anchorableIDs[t.ID] = true
	}

	// Create sections whose MatchedTopicID points at each candidate.
	// Assignments should only succeed for topics in the anchorable set.
	id2 := uint(2)
	id3 := uint(3)
	sections := []DailyReportSection{
		{ClusterLabel: "hits in-window-2", Embedding: vecStr(0.89, 0.42, 0), MatchedTopicID: &id2},
		{ClusterLabel: "hits in-window-3", Embedding: vecStr(0.79, 0.49, 0), MatchedTopicID: &id3},
	}

	decisions := planTopicAssignments(sections, selected, cfg, reportDate)
	require.Len(t, decisions, 2)

	for _, d := range decisions {
		assert.True(t, anchorableIDs[d.topicID], "topic %d not in anchorable set", d.topicID)
		assert.Equal(t, TopicConfAnchorHit, d.confidence, "section should anchor to anchorable topic")
		assert.NotNil(t, d.topicStatusAtReport)
	}

	// Now verify the negative: a section whose MatchedTopicID points at a
	// filtered candidate (id=4, gap > window) cannot anchor because id=4 is
	// not in the anchorable set. planTopicAssignments receives the restricted
	// set, so it opens a new candidate instead.
	staleTopics := []BoardPersistentTopic{
		{ID: 1, Status: TopicStatusActive, Embedding: vecStr(1, 0, 0), LastSeenDate: reportDate.AddDate(0, 0, -5)},
	}
	filteredSelected, filteredStats := selectAnchorableTopics(append(staleTopics, BoardPersistentTopic{
		ID: 4, Status: TopicStatusCandidate, Embedding: vecStr(0.9, 0.4, 0), LastSeenDate: reportDate.AddDate(0, 0, -9),
	}), reportDate, cfg)
	assert.Equal(t, 1, filteredStats.FilteredByWindow, "candidate with gap=9 should be filtered")
	// The filtered set contains only the active topic.
	require.Len(t, filteredSelected, 1)

	id4 := uint(4)
	sectionForStale := []DailyReportSection{{
		ClusterLabel: "misses stale", Embedding: vecStr(0.89, 0.39, 0), MatchedTopicID: &id4,
	}}
	decisionsForStale := planTopicAssignments(sectionForStale, filteredSelected, cfg, reportDate)
	require.Len(t, decisionsForStale, 1)
	assert.Equal(t, TopicConfAutoNew, decisionsForStale[0].confidence,
		"pointing at a filtered candidate must trigger auto_new, not anchor hit")
}

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
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3}
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
	require.NotNil(t, dec[0].topicStatusAtReport)
	assert.Equal(t, TopicStatusActive, *dec[0].topicStatusAtReport)
}

// TestPlanTopicAssignments_AnchorHit_MatchedWithinThresholdNotNearest is the
// regression guard for the 06-25 "all emerging" incident. The section is
// nearest to topic 12, but LLM clustering drift made the LLM pick topic 13
// (2nd-nearest, still within the 0.30 embedding threshold). Pre-fix the dual
// confirmation demanded matched_id == nearest → it failed (13 != 12) and
// opened a new candidate, severing the topic lineage. Post-fix the LLM gate
// accepts any topic within the embedding threshold, so it anchors to 13 at
// its actual distance. Both gates still apply (embedding AND LLM pointing at
// the same topic) — this is NOT pure-embedding matching.
func TestPlanTopicAssignments_AnchorHit_MatchedWithinThresholdNotNearest(t *testing.T) {
	mit := uint(13)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3}
	topics := []BoardPersistentTopic{
		{ID: 12, Embedding: vecStr(1, 0, 0), Status: TopicStatusActive},      // nearest (~0.0001)
		{ID: 13, Embedding: vecStr(0.9, 0.43, 0), Status: TopicStatusActive}, // 2nd-nearest (~0.093, within threshold)
	}
	sections := []DailyReportSection{{
		ClusterLabel:   "AI 编程竞争",
		Embedding:      vecStr(1, 0.01, 0),
		MatchedTopicID: &mit, // LLM drifted to 2nd-nearest
	}}
	dec := planTopicAssignments(sections, topics, cfg, time.Now())
	require.Len(t, dec, 1)
	assert.Equal(t, TopicConfAnchorHit, dec[0].confidence, "must anchor, not open new candidate")
	assert.Equal(t, uint(13), dec[0].topicID, "anchor to the LLM-chosen topic, not the nearest")
	assert.InDelta(t, 0.093, dec[0].distance, 0.01, "distance is the anchored topic's actual distance")
}

// TestPlanTopicAssignments_AutoNew_MatchedBeyondThreshold verifies the
// embedding gate still rejects a topic the LLM named when that topic is far
// (> threshold). The relaxation only accepts topics within the threshold; it
// is NOT pure-LLM matching.
func TestPlanTopicAssignments_AutoNew_MatchedBeyondThreshold(t *testing.T) {
	mit := uint(99)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3}
	topics := []BoardPersistentTopic{
		{ID: 12, Embedding: vecStr(1, 0, 0), Status: TopicStatusActive},      // near but LLM didn't pick it
		{ID: 99, Embedding: vecStr(0.2, 0.98, 0), Status: TopicStatusActive}, // LLM picked, but far (~0.79)
	}
	sections := []DailyReportSection{{
		ClusterLabel:   "量子计算商用",
		Embedding:      vecStr(1, 0.01, 0),
		MatchedTopicID: &mit,
	}}
	dec := planTopicAssignments(sections, topics, cfg, time.Now())
	require.Len(t, dec, 1)
	assert.Equal(t, TopicConfAutoNew, dec[0].confidence)
	assert.NotNil(t, dec[0].newCandidate)
	require.NotNil(t, dec[0].topicStatusAtReport)
	assert.Equal(t, TopicStatusCandidate, *dec[0].topicStatusAtReport)
}

func TestPlanTopicAssignments_AutoNew_DualConfirmationFail(t *testing.T) {
	// Section is embedding-close to topic 12 (dist < threshold), but the LLM
	// did NOT mark it (MatchedTopicID points elsewhere). Dual confirmation fails
	// → a new candidate must be opened, NOT an anchor hit.
	other := uint(99)
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3}
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
	cfg := PersistentTopicConfig{MatchThreshold: 0.30, UpgradeThreshold: 3}
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
	assert.Nil(t, dec[0].topicStatusAtReport)
}

func TestPlanLifecycle_EligibleCandidateStillRequiresManualConfirmation(t *testing.T) {
	// Reaching the occurrence threshold makes a candidate eligible for manual
	// confirmation, but the daily pipeline must not publish it automatically.
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 2, HitCount: 2,
		LastSeenDate: time.Now().AddDate(0, 0, -1),
	}}
	changes := planLifecycle(topics, time.Now(), map[uint]bool{1: true})
	require.Len(t, changes, 1)
	assert.Equal(t, TopicStatusCandidate, changes[0].status)
	assert.Equal(t, 3, changes[0].consecutiveHits)
	assert.Equal(t, 3, changes[0].hitCount)
}

func TestPlanLifecycle_HitKeepsCandidateBelowThreshold(t *testing.T) {
	// candidate at consecutive=1, hit today → stays candidate (2 < 3).
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 1, HitCount: 1,
		LastSeenDate: time.Now().AddDate(0, 0, -1),
	}}
	changes := planLifecycle(topics, time.Now(), map[uint]bool{1: true})
	require.Len(t, changes, 1)
	assert.Equal(t, TopicStatusCandidate, changes[0].status)
	assert.Equal(t, 2, changes[0].consecutiveHits)
}

func TestPlanLifecycle_MissResetsConsecutive(t *testing.T) {
	// candidate not hit today → consecutive resets to 0; stays candidate.
	topics := []BoardPersistentTopic{{
		ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 2, HitCount: 2,
		LastSeenDate: time.Now().AddDate(0, 0, -1),
	}}
	changes := planLifecycle(topics, time.Now(), map[uint]bool{})
	require.Len(t, changes, 1)
	assert.Equal(t, TopicStatusCandidate, changes[0].status)
	assert.Equal(t, 0, changes[0].consecutiveHits)
	assert.Equal(t, 2, changes[0].hitCount)
}

func TestPlanLifecycle_LongGapNeverArchives(t *testing.T) {
	// candidate 60 days behind last_seen, active 35 days behind last_seen —
	// neither must be archived because archiving is manual-only. Both miss
	// today, so both reset consecutive_hits to 0.
	today := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	topics := []BoardPersistentTopic{
		{ID: 1, Status: TopicStatusCandidate, ConsecutiveHits: 2, HitCount: 4, LastSeenDate: today.AddDate(0, 0, -60)},
		{ID: 2, Status: TopicStatusActive, ConsecutiveHits: 5, HitCount: 10, LastSeenDate: today.AddDate(0, 0, -35)},
	}
	changes := planLifecycle(topics, today, map[uint]bool{})

	require.Len(t, changes, 2)
	assert.Equal(t, uint(1), changes[0].topicID)
	assert.Equal(t, TopicStatusCandidate, changes[0].status)
	assert.Equal(t, 0, changes[0].consecutiveHits)
	assert.Equal(t, uint(2), changes[1].topicID)
	assert.Equal(t, TopicStatusActive, changes[1].status)
	assert.Equal(t, 0, changes[1].consecutiveHits)
}
