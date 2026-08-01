package service

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/topicgraph/repository"
)

// TestClusterTagsLane_FakeLLM exercises the full lane pipeline with a fake LLM,
// covering the spec scenarios the unit tests decompose:
//   - L1 strong match → direct group, NO LLM call
//   - vacuum topic → strong-match tag downgraded to L2 (LLM adjudicates)
//   - L2 keep → merges into the target topic's group
//   - L2 new → routed into the L3 pool
//   - L3 (+L2-new ≤2) → each its own group, no LLM
func TestClusterTagsLane_FakeLLM(t *testing.T) {
	// Swap the LLM hook for a fake and restore it after the test.
	var llmCalls int32
	origChat := clusterChatFn
	t.Cleanup(func() { clusterChatFn = origChat })
	clusterChatFn = func(ctx context.Context, system, user string, schema *airouter.JSONSchema, operation string) (string, error) {
		atomic.AddInt32(&llmCalls, 1)
		if operation == "daily_report.decide_l2_tags" {
			// tag2 → keep (target 8, its nearest candidate); tag3 → new.
			return `{"decisions":[{"tag_id":2,"decision":"keep"},{"tag_id":3,"decision":"new"}]}`, nil
		}
		// L3 new-narrative path is ≤2 here so should not be reached; return a
		// neutral single-group response as a safety net.
		return `{"groups":[{"group_name":"g","tag_ids":[]}]}`, nil
	}

	normalTopic := repository.BoardPersistentTopic{ID: 7, Label: "T7", Centroid: repository.FloatsToPgVector(unit(1, 0)), Status: repository.TopicStatusActive}
	vacuumTopic := repository.BoardPersistentTopic{ID: 8, Label: "T8-vac", Centroid: repository.FloatsToPgVector(unit(0, 1)), IsVacuum: true, Status: repository.TopicStatusActive}
	topics := []repository.BoardPersistentTopic{normalTopic, vacuumTopic}

	tags := []repository.TagInput{
		{ID: 1, Label: "l1-strong"},
		{ID: 2, Label: "vacuum-downgraded"},
		{ID: 3, Label: "l2-new"},
		{ID: 4, Label: "l3-far"},
	}
	emb := map[uint][]float64{
		1: unit(1, 0),  // dist 0.0 to T7 → L1
		2: unit(0, 1),  // dist 0.0 to T8 (vacuum) → L2 downgrade
		3: unit(0.8, 0.6), // dist 0.2 to T7 → L2 (candidate T7)
		4: unit(-1, 0), // dist 2.0 to T7, 1.0 to T8 → L3
	}

	clusters, err := ClusterTagsLane(context.Background(), tags, topics, emb, nil, laneCfg())
	require.NoError(t, err)

	// Exactly one LLM call (L2裁决); L1 skipped the LLM, L3 was ≤2.
	assert.Equal(t, int32(1), atomic.LoadInt32(&llmCalls), "only the L2 LLM call should fire")

	// Bucket into lanes for assertion.
	byLane := map[string][]repository.ClusterGroup{}
	for _, c := range clusters {
		byLane[c.Lane] = append(byLane[c.Lane], c)
	}
	require.Len(t, byLane["l1"], 1, "one L1 group (tag1 → T7)")
	require.NotNil(t, byLane["l1"][0].MatchedTopicID)
	assert.Equal(t, uint(7), *byLane["l1"][0].MatchedTopicID)
	assert.Equal(t, []uint{1}, byLane["l1"][0].TagIDs)

	require.Len(t, byLane["l2"], 1, "one L2 group (tag2 keep → T8)")
	require.NotNil(t, byLane["l2"][0].MatchedTopicID)
	assert.Equal(t, uint(8), *byLane["l2"][0].MatchedTopicID, "vacuum-downgraded tag anchors to T8 via keep")
	assert.Equal(t, []uint{2}, byLane["l2"][0].TagIDs)

	// L3: tag3 (L2-new) + tag4 → each its own group (≤2 fallback).
	require.Len(t, byLane["l3"], 2, "tag3 (L2-new) + tag4 each form an L3 group")
	l3TagIDs := map[uint]bool{}
	for _, g := range byLane["l3"] {
		assert.Nil(t, g.MatchedTopicID, "L3 groups have no owning topic")
		for _, id := range g.TagIDs {
			l3TagIDs[id] = true
		}
	}
	assert.True(t, l3TagIDs[3] && l3TagIDs[4], "both the L2-new tag and the L3 tag landed in L3 groups")
}

// TestClusterTagsLane_L2OnlySwitchOffShortlist verifies the off-shortlist
// downgrade end-to-end: an LLM "switch" to a topic NOT in the candidate set
// degrades to new and the tag lands in an L3 group.
func TestClusterTagsLane_L2OnlySwitchOffShortlist(t *testing.T) {
	origChat := clusterChatFn
	t.Cleanup(func() { clusterChatFn = origChat })
	clusterChatFn = func(ctx context.Context, system, user string, schema *airouter.JSONSchema, operation string) (string, error) {
		// switch to topic 999 (not in any candidate set) → off-shortlist → new.
		return `{"decisions":[{"tag_id":1,"decision":"switch","target_topic_id":999}]}`, nil
	}

	topics := []repository.BoardPersistentTopic{{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0)), Status: repository.TopicStatusActive}}
	tags := []repository.TagInput{{ID: 1, Label: "weak"}, {ID: 2, Label: "weak2"}, {ID: 3, Label: "weak3"}}
	emb := map[uint][]float64{
		1: unit(0.8, 0.6), // L2
		2: unit(0.8, 0.6), // L2
		3: unit(0.8, 0.6), // L2 — 3 L2 tags > 2 ⇒ LLM fires
	}
	clusters, err := ClusterTagsLane(context.Background(), tags, topics, emb, nil, laneCfg())
	require.NoError(t, err)

	// tag1's off-shortlist switch → new → routed to L3. All three tags end up
	// in L3 (the other two default-keep, but they keep to topic 7 → L2 groups).
	l2Owner := map[uint]int{}
	l3Tags := map[uint]bool{}
	for _, c := range clusters {
		if c.Lane == "l2" && c.MatchedTopicID != nil {
			l2Owner[*c.MatchedTopicID] += len(c.TagIDs)
		}
		if c.Lane == "l3" {
			for _, id := range c.TagIDs {
				l3Tags[id] = true
			}
		}
	}
	assert.True(t, l3Tags[1], "off-shortlist switch tag routed to L3")
	assert.GreaterOrEqual(t, l2Owner[7], 2, "the other two L2 tags default-keep to topic 7")
}
