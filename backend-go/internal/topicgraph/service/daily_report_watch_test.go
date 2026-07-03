package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/topicgraph/repository"
)

func TestBuildWatchHitPromptContainsWatchesAndSections(t *testing.T) {
	watches := []repository.BoardTopicWatch{
		{ID: 1, Label: "美伊会不会真打起来"},
		{ID: 2, Label: "中美关税走向"},
	}
	sections := []repository.DailyReportSection{
		{ID: 101, ClusterLabel: "G7峰会各方表态"},
		{ID: 102, ClusterLabel: "美伊恢复核谈判"},
	}

	prompt := buildWatchHitPrompt(watches, sections)

	// Must contain watch IDs and labels
	assert.Contains(t, prompt, "美伊会不会真打起来")
	assert.Contains(t, prompt, "中美关税走向")
	assert.Contains(t, prompt, "[id:1]")
	assert.Contains(t, prompt, "[id:2]")

	// Must contain section IDs and labels
	assert.Contains(t, prompt, "G7峰会各方表态")
	assert.Contains(t, prompt, "美伊恢复核谈判")
	assert.Contains(t, prompt, "[section_id:101]")
	assert.Contains(t, prompt, "[section_id:102]")

	// Must contain the expected output format hint
	assert.Contains(t, prompt, `"hits"`)
	assert.Contains(t, prompt, `"watch_id"`)
}

func TestBuildWatchHitPrompt_Empty(t *testing.T) {
	prompt := buildWatchHitPrompt(nil, nil)
	assert.Contains(t, prompt, "关注标记")
	assert.Contains(t, prompt, "日报节列表")
}

func TestParseWatchHitResponse_Valid(t *testing.T) {
	content := `{"hits":[
		{"watch_id":1,"section_id":101,"reason":"该节讨论了美伊问题"},
		{"watch_id":2,"section_id":102,"reason":"涉及关税讨论"}
	]}`

	validWatchIDs := map[uint]bool{1: true, 2: true}
	validSectionIDs := map[uint]bool{101: true, 102: true}
	report := &repository.BoardDailyReport{
		ID:         200,
		PeriodDate: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	}

	hits, err := parseWatchHitResponse(content, validWatchIDs, validSectionIDs, report)
	require.NoError(t, err)
	require.Len(t, hits, 2)

	assert.Equal(t, uint(1), hits[0].WatchID)
	assert.Equal(t, uint(101), hits[0].SectionID)
	assert.Equal(t, uint(200), hits[0].ReportID)
	assert.Equal(t, "该节讨论了美伊问题", hits[0].Reason)

	assert.Equal(t, uint(2), hits[1].WatchID)
	assert.Equal(t, uint(102), hits[1].SectionID)
}

func TestParseWatchHitResponse_FiltersHallucinatedIDs(t *testing.T) {
	content := `{"hits":[
		{"watch_id":1,"section_id":101,"reason":"valid"},
		{"watch_id":999,"section_id":101,"reason":"hallucinated watch_id"},
		{"watch_id":1,"section_id":999,"reason":"hallucinated section_id"}
	]}`

	validWatchIDs := map[uint]bool{1: true}
	validSectionIDs := map[uint]bool{101: true}
	report := &repository.BoardDailyReport{
		ID:         200,
		PeriodDate: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	}

	hits, err := parseWatchHitResponse(content, validWatchIDs, validSectionIDs, report)
	require.NoError(t, err)
	require.Len(t, hits, 1, "only the valid entry should survive")
	assert.Equal(t, uint(1), hits[0].WatchID)
	assert.Equal(t, uint(101), hits[0].SectionID)
}

func TestParseWatchHitResponse_InvalidJSON(t *testing.T) {
	report := &repository.BoardDailyReport{ID: 200, PeriodDate: time.Now()}
	_, err := parseWatchHitResponse("not-json", map[uint]bool{}, map[uint]bool{}, report)
	assert.Error(t, err)
}

func TestParseWatchHitResponse_EmptyHits(t *testing.T) {
	content := `{"hits":[]}`
	report := &repository.BoardDailyReport{ID: 200, PeriodDate: time.Now()}
	hits, err := parseWatchHitResponse(content, map[uint]bool{1: true}, map[uint]bool{101: true}, report)
	require.NoError(t, err)
	assert.Empty(t, hits)
}

// Verify the JSON schema structure matches what the AI is asked to produce.
func TestWatchHitJSONSchemaRoundtrip(t *testing.T) {
	// Simulate what the AI should return
	expected := rawWatchHitResponse{
		Hits: []rawWatchHit{
			{WatchID: 1, SectionID: 100, Reason: "test reason"},
		},
	}
	bytes, err := json.Marshal(expected)
	require.NoError(t, err)

	var parsed rawWatchHitResponse
	require.NoError(t, json.Unmarshal(bytes, &parsed))
	require.Len(t, parsed.Hits, 1)
	assert.Equal(t, uint(1), parsed.Hits[0].WatchID)
	assert.Equal(t, "test reason", parsed.Hits[0].Reason)
}
