package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/topicgraph/repository"
)

func TestFilterVisibleTopics_HidesObservingCandidates(t *testing.T) {
	topics := []repository.BoardPersistentTopic{
		{ID: 1, Status: repository.TopicStatusActive, ConsecutiveHits: 0, HitCount: 1},
		{ID: 2, Status: repository.TopicStatusArchived, ConsecutiveHits: 0, HitCount: 1},
		{ID: 3, Status: repository.TopicStatusCandidate, ConsecutiveHits: 0, HitCount: 2}, // below threshold 3 (by hit_count)
		{ID: 4, Status: repository.TopicStatusCandidate, ConsecutiveHits: 0, HitCount: 3}, // meets threshold (by hit_count)
		{ID: 5, Status: repository.TopicStatusCandidate, ConsecutiveHits: 1, HitCount: 5}, // above threshold; cons kept low to prove hit_count is the gate
	}
	result := repository.FilterVisibleTopics(topics, 3)
	require.Len(t, result, 4)
	ids := make([]uint, len(result))
	for i, topic := range result {
		ids[i] = topic.ID
	}
	require.ElementsMatch(t, []uint{1, 2, 4, 5}, ids)
	// Verify the observing candidate is excluded
	for _, topic := range result {
		assert.NotEqual(t, uint(3), topic.ID, "observing candidate id=3 must not be visible")
	}
}

// TestFilterVisibleTopics_UsesHitCountNotConsecutive confirms the visibility
// gate is cumulative hit_count, NOT consecutive_hits: a candidate with high
// consecutive_hits but low hit_count is hidden, and one with low consecutive
// but high hit_count is shown.
func TestFilterVisibleTopics_UsesHitCountNotConsecutive(t *testing.T) {
	topics := []repository.BoardPersistentTopic{
		// high consecutive (5) but low hit_count (1) → hidden (underqualified by cumulative)
		{ID: 1, Status: repository.TopicStatusCandidate, ConsecutiveHits: 5, HitCount: 1},
		// low consecutive (0) but high hit_count (3) → shown (qualified by cumulative)
		{ID: 2, Status: repository.TopicStatusCandidate, ConsecutiveHits: 0, HitCount: 3},
	}
	result := repository.FilterVisibleTopics(topics, 3)
	require.Len(t, result, 1)
	require.Equal(t, uint(2), result[0].ID, "only cumulative hit_count>=threshold qualifies, regardless of consecutive")
}

func TestBuildDailyReportProgressMessageMatchesFrontendContract(t *testing.T) {
	msg := buildProgressMessage("job-1", "generating", 2849, "刚果（金）局势", 0, "0/1")

	require.Equal(t, "daily_report_progress", msg["type"])
	require.Equal(t, "job-1", msg["job_id"])
	require.Equal(t, uint(2849), msg["board_id"])
	require.Equal(t, "刚果（金）局势", msg["board_name"])
	require.Equal(t, "generating", msg["status"])
	require.Equal(t, 0, msg["saved"])
	require.Equal(t, "0/1", msg["progress"])
	require.NotEmpty(t, msg["timestamp"])
}

func TestBuildDailyReportDoneMessageMatchesFrontendContract(t *testing.T) {
	msg := buildDoneMessage("job-1", 1, 1)

	require.Equal(t, "daily_report_done", msg["type"])
	require.Equal(t, "job-1", msg["job_id"])
	require.Equal(t, 1, msg["total_saved"])
	require.Equal(t, 1, msg["total_boards"])
	require.NotEmpty(t, msg["timestamp"])
}
