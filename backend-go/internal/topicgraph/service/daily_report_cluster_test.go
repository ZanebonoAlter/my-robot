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
	groups, err := ClusterTags(nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, groups)
}

func TestClusterTags_SingleTag(t *testing.T) {
	tags := []repository.TagInput{{ID: 1, Label: "Test Event", ArticleCount: 3}}
	groups, err := ClusterTags(nil, tags, nil, nil)
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
	groups, err := ClusterTags(nil, tags, nil, nil)
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

// ---- Slice D tests ----

// TestClusterTags_WithBriefsShortCircuit verifies the short-circuit path
// (≤2 tags) still works when briefs are non-nil — the function SHALL NOT
// panic or behave differently when briefs are present.
func TestClusterTags_WithBriefsShortCircuit(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Event A", ArticleCount: 2},
		{ID: 2, Label: "Event B", ArticleCount: 3},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		7: {{TopicID: 7, SectionID: 101, SectionLabel: "Test", ThreadTitles: []string{"thread"}}},
	}
	// Short-circuit path: ≤2 tags skips LLM, briefs are irrelevant but must not break.
	groups, err := ClusterTags(nil, tags, nil, briefs)
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
}

// TestClusterTags_NilBriefsDegradation verifies that nil briefs does not
// break ClusterTags — the function SHALL fall back to label-only injection.
func TestClusterTags_NilBriefsDegradation(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "Test", ArticleCount: 1},
	}
	// nil briefs = degradation path
	groups, err := ClusterTags(nil, tags, nil, nil)
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
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
	prompt := buildClusterSystemPrompt(10, topics, nil)
	assert.Contains(t, prompt, "已有的叙事框架")
	assert.Contains(t, prompt, "AI 编程工具平台化竞争")
	assert.Contains(t, prompt, "量子计算商用突破")
	assert.Contains(t, prompt, "matched_topic_id")
}

// TestBuildClusterSystemPrompt_NoTopicsOmitsSection confirms that without
// existing topics the prompt stays close to the original (no empty section).
func TestBuildClusterSystemPrompt_NoTopicsOmitsSection(t *testing.T) {
	prompt := buildClusterSystemPrompt(10, nil, nil)
	assert.NotContains(t, prompt, "已有的叙事框架")
}

// ---- Slice D: lane context injection tests ----

func TestBuildClusterSystemPrompt_ActiveTopicWithRecentBriefs(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	topics := []repository.BoardPersistentTopic{
		{ID: 7, Label: "以黎冲突升级", Status: repository.TopicStatusActive,
			LastSeenDate: now, HitCount: 5},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		7: {
			{
				TopicID: 7, SectionID: 101,
				SectionLabel: "真主党越境打击",
				PeriodDate:   now.AddDate(0, 0, -1),
				ThreadTitles: []string{"真主党向以色列北部发射火箭", "以军拦截黎方无人机"},
			},
			{
				TopicID: 7, SectionID: 102,
				SectionLabel: "以军空袭黎南部",
				PeriodDate:   now.AddDate(0, 0, -2),
				ThreadTitles: []string{"以色列空袭黎巴嫩南部目标"},
			},
		},
	}
	prompt := buildClusterSystemPrompt(10, topics, briefs)
	// Active topic with briefs SHALL include recent content
	assert.Contains(t, prompt, "以黎冲突升级")
	assert.Contains(t, prompt, "近期实际内容")
	assert.Contains(t, prompt, "真主党越境打击")
	assert.Contains(t, prompt, "真主党向以色列北部发射火箭")
	assert.Contains(t, prompt, "以军空袭黎南部")
	assert.Contains(t, prompt, "以色列空袭黎巴嫩南部目标")
}

func TestBuildClusterSystemPrompt_ActiveTopicEmptyBriefs(t *testing.T) {
	now := time.Now()
	topics := []repository.BoardPersistentTopic{
		{ID: 7, Label: "以黎冲突升级", Status: repository.TopicStatusActive,
			LastSeenDate: now, HitCount: 5},
	}
	// Empty briefs map → degradation: label-only for active topic
	prompt := buildClusterSystemPrompt(10, topics, map[uint][]repository.TopicRecentBrief{})
	assert.Contains(t, prompt, "以黎冲突升级")
	// Degradation: briefs empty means no "近期实际内容" guidance
	assert.NotContains(t, prompt, "近期实际内容")
}

func TestBuildClusterSystemPrompt_CandidateTopicNoBriefs(t *testing.T) {
	now := time.Now()
	topics := []repository.BoardPersistentTopic{
		{ID: 15, Label: "候选叙事", Status: repository.TopicStatusCandidate,
			LastSeenDate: now, HitCount: 2},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		15: {
			{TopicID: 15, SectionID: 201, SectionLabel: "some-section",
				PeriodDate: now, ThreadTitles: []string{"some-thread"}},
		},
	}
	prompt := buildClusterSystemPrompt(10, topics, briefs)
	// Candidate topic SHALL NOT show recent content even if briefs exist
	assert.Contains(t, prompt, "候选叙事")
	assert.NotContains(t, prompt, "some-section")
	assert.NotContains(t, prompt, "some-thread")
	assert.NotContains(t, prompt, "近期实际内容")
}

func TestBuildClusterSystemPrompt_ActiveAndCandidateMixed(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	topics := []repository.BoardPersistentTopic{
		{ID: 7, Label: "以黎冲突升级", Status: repository.TopicStatusActive,
			LastSeenDate: now, HitCount: 5},
		{ID: 15, Label: "候选叙事", Status: repository.TopicStatusCandidate,
			LastSeenDate: now, HitCount: 2},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		7: {
			{TopicID: 7, SectionID: 101, SectionLabel: "真主党越境打击",
				PeriodDate: now, ThreadTitles: []string{"真主党发射火箭"}},
		},
		15: {
			{TopicID: 15, SectionID: 201, SectionLabel: "should-not-appear",
				PeriodDate: now, ThreadTitles: []string{"hidden-thread"}},
		},
	}
	prompt := buildClusterSystemPrompt(10, topics, briefs)
	// Active has content
	assert.Contains(t, prompt, "以黎冲突升级")
	assert.Contains(t, prompt, "真主党越境打击")
	assert.Contains(t, prompt, "近期实际内容")
	// Candidate is label-only
	assert.Contains(t, prompt, "候选叙事")
	assert.NotContains(t, prompt, "should-not-appear")
	assert.NotContains(t, prompt, "hidden-thread")
}

func TestBuildClusterSystemPrompt_GuidanceTextPresent(t *testing.T) {
	now := time.Now()
	topics := []repository.BoardPersistentTopic{
		{ID: 7, Label: "AI 编程工具", Status: repository.TopicStatusActive,
			LastSeenDate: now, HitCount: 3},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		7: {
			{TopicID: 7, SectionID: 101, SectionLabel: "Codex 更新",
				PeriodDate: now, ThreadTitles: []string{"Codex 推出插件系统"}},
		},
	}
	prompt := buildClusterSystemPrompt(10, topics, briefs)
	// The guidance text must instruct the LLM to base decisions on actual content
	assert.Contains(t, prompt, "依据框架近期实际内容判断归属")
	assert.Contains(t, prompt, "而非仅凭标题字面沾边")
}
