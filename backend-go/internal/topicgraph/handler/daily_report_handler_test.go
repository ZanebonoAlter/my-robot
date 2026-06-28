package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/topicgraph/repository"
)

func TestFilterVisibleTopics_HidesObservingCandidates(t *testing.T) {
	topics := []repository.BoardPersistentTopic{
		{ID: 1, Status: repository.TopicStatusActive, ConsecutiveHits: 0},
		{ID: 2, Status: repository.TopicStatusArchived, ConsecutiveHits: 0},
		{ID: 3, Status: repository.TopicStatusCandidate, ConsecutiveHits: 2}, // below threshold 3
		{ID: 4, Status: repository.TopicStatusCandidate, ConsecutiveHits: 3}, // meets threshold
		{ID: 5, Status: repository.TopicStatusCandidate, ConsecutiveHits: 5}, // above threshold
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
