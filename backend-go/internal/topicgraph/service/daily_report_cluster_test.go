package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/topicgraph/repository"
)

func TestClusterTags_Empty(t *testing.T) {
	groups, err := ClusterTags(nil, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, groups)
}

func TestClusterTags_SingleTag(t *testing.T) {
	tags := []repository.TagInput{{ID: 1, Label: "Test Event", ArticleCount: 3}}
	groups, err := ClusterTags(nil, tags, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "Test Event", groups[0].GroupName)
	assert.Equal(t, []uint{1}, groups[0].TagIDs)
}

func TestClusterTags_TwoTags(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A", ArticleCount: 2},
		{ID: 2, Label: "Event B", ArticleCount: 3},
	}
	groups, err := ClusterTags(nil, tags, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestParseClusterResponse_Valid(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "G7 Summit"},
		{ID: 2, Label: "G7 Statement"},
		{ID: 3, Label: "Fed Rate Hike"},
	}
	content := `{"groups":[{"group_name":"G7峰会","tag_ids":[1,2]},{"group_name":"美联储加息","tag_ids":[3]}]}`
	groups, err := parseClusterResponse(content, tags, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "G7峰会", groups[0].GroupName)
	assert.Equal(t, []uint{1, 2}, groups[0].TagIDs)
}

func TestParseClusterResponse_UnassignedTags(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A"},
		{ID: 2, Label: "Event B"},
		{ID: 3, Label: "Event C"},
	}
	// Only ID 1 assigned in response; 2 and 3 should get their own groups.
	content := `{"groups":[{"group_name":"Group A","tag_ids":[1]}]}`
	groups, err := parseClusterResponse(content, tags, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 3)
}

func TestParseClusterResponse_UnknownIDs(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A"},
	}
	content := `{"groups":[{"group_name":"Group","tag_ids":[1, 999]}]}`
	groups, err := parseClusterResponse(content, tags, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, []uint{1}, groups[0].TagIDs)
}

func TestParseClusterResponse_EmptyGroupName(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A"},
	}
	content := `{"groups":[{"group_name":"","tag_ids":[1]}]}`
	groups, err := parseClusterResponse(content, tags, nil)
	assert.NoError(t, err)
	// Empty group name is skipped, so the tag gets its own fallback group.
	assert.Len(t, groups, 1)
	assert.Equal(t, "Event A", groups[0].GroupName)
}

func TestParseClusterResponse_DuplicateTagIDs(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A"},
		{ID: 2, Label: "Event B"},
	}
	content := `{"groups":[{"group_name":"G1","tag_ids":[1,2]},{"group_name":"G2","tag_ids":[1]}]}`
	groups, err := parseClusterResponse(content, tags, nil)
	assert.NoError(t, err)
	// Tag 1 appears in G1 first, so G2 should be empty and skipped.
	// Tag 2 is only in G1.
	assert.Len(t, groups, 1)
	assert.Equal(t, "G1", groups[0].GroupName)
}

func TestParseClusterResponse_InvalidJSON(t *testing.T) {
	tags := []repository.TagInput{{ID: 1, Label: "A"}}
	content := `not json`
	_, err := parseClusterResponse(content, tags, nil)
	assert.Error(t, err)
}

func TestBuildClusterPrompt(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Test", ArticleCount: 5, Description: "A test event"},
	}
	prompt := buildClusterPrompt(tags)
	assert.Contains(t, prompt, "[ID:1]")
	assert.Contains(t, prompt, "Test")
	assert.Contains(t, prompt, "A test event")
}

func TestClusterTags_ManyTagsSkipLLM(t *testing.T) {
	// With >2 tags, the real LLM would be called. This test just verifies
	// the logic path — we can't easily test the LLM path without a mock.
	// Instead we test parseClusterResponse directly.
	input := `{"groups":[{"group_name":"Tech","tag_ids":[1,2,3]},{"group_name":"Politics","tag_ids":[4,5]}]}`
	tags := []repository.TagInput{
		{ID: 1, Label: "Apple"},
		{ID: 2, Label: "Google"},
		{ID: 3, Label: "Microsoft"},
		{ID: 4, Label: "Election"},
		{ID: 5, Label: "Congress"},
	}
	groups, err := parseClusterResponse(input, tags, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 2)

	// Verify the JSON round-trip works
	data, err := json.Marshal(groups)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "Tech")
}

// TestParseClusterResponse_MatchedTopicID_Preserved confirms a legal
// matched_topic_id from the LLM is carried onto the cluster group.
func TestParseClusterResponse_MatchedTopicID_Preserved(t *testing.T) {
	tags := []repository.TagInput{{ID: 1, Label: "Apple"}, {ID: 2, Label: "Google"}}
	topics := []repository.BoardPersistentTopic{{ID: 7, Label: "Tech", Status: repository.TopicStatusActive}}
	content := `{"groups":[{"group_name":"Tech","tag_ids":[1,2],"matched_topic_id":7}]}`
	groups, err := parseClusterResponse(content, tags, topics)
	assert.NoError(t, err)
	require.Len(t, groups, 1)
	require.NotNil(t, groups[0].MatchedTopicID)
	assert.Equal(t, uint(7), *groups[0].MatchedTopicID)
}

// TestParseClusterResponse_HallucinatedTopicID_Degrades confirms an
// matched_topic_id NOT in the supplied topic set is dropped (degraded to nil)
// rather than corrupting the dual-confirmation step.
func TestParseClusterResponse_HallucinatedTopicID_Degrades(t *testing.T) {
	tags := []repository.TagInput{{ID: 1, Label: "Apple"}}
	topics := []repository.BoardPersistentTopic{{ID: 7, Label: "Tech", Status: repository.TopicStatusActive}}
	// LLM claims topic 999 which was never in the injected frame list.
	content := `{"groups":[{"group_name":"Tech","tag_ids":[1],"matched_topic_id":999}]}`
	groups, err := parseClusterResponse(content, tags, topics)
	assert.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Nil(t, groups[0].MatchedTopicID, "hallucinated topic id must degrade to nil")
}

// TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades confirms that
// when two groups both claim the same existing topic id, only the FIRST group
// keeps it; the second group's matched_topic_id degrades to nil (becomes a new
// topic). Without this guard, one durable frame would silently absorb two
// disjoint clusters, re-creating the over-broad-topic problem.
func TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A"},
		{ID: 2, Label: "Event B"},
		{ID: 3, Label: "Event C"},
	}
	topics := []repository.BoardPersistentTopic{{ID: 7, Label: "Tech", Status: repository.TopicStatusActive}}
	// Both groups claim topic 7.
	content := `{"groups":[
		{"group_name":"Tech A","tag_ids":[1,2],"matched_topic_id":7},
		{"group_name":"Tech B","tag_ids":[3],"matched_topic_id":7}
	]}`
	groups, err := parseClusterResponse(content, tags, topics)
	assert.NoError(t, err)
	require.Len(t, groups, 2)
	require.NotNil(t, groups[0].MatchedTopicID, "first claim keeps the topic")
	assert.Equal(t, uint(7), *groups[0].MatchedTopicID)
	assert.Nil(t, groups[1].MatchedTopicID, "second claim on the same topic must degrade to nil")
}

// TestBuildClusterSystemPrompt_InjectsExistingTopics confirms the durable
// narrative frames are present in the system prompt (root cause A fix).
func TestBuildClusterSystemPrompt_InjectsExistingTopics(t *testing.T) {
	topics := []repository.BoardPersistentTopic{
		{ID: 7, Label: "AI 编程工具平台化竞争", Status: repository.TopicStatusActive,
			LastSeenDate: time.Now(), HitCount: 5},
		{ID: 8, Label: "量子计算商用突破", Status: repository.TopicStatusCandidate,
			LastSeenDate: time.Now(), HitCount: 2},
	}
	prompt := buildClusterSystemPrompt(10, topics)
	assert.Contains(t, prompt, "已有的叙事框架")
	assert.Contains(t, prompt, "AI 编程工具平台化竞争")
	assert.Contains(t, prompt, "量子计算商用突破")
	assert.Contains(t, prompt, "matched_topic_id")
}

// TestBuildClusterSystemPrompt_NoTopicsOmitsSection confirms that without
// existing topics the prompt stays close to the original (no empty section).
func TestBuildClusterSystemPrompt_NoTopicsOmitsSection(t *testing.T) {
	prompt := buildClusterSystemPrompt(10, nil)
	assert.NotContains(t, prompt, "已有的叙事框架")
}
