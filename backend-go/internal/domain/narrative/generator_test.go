package narrative

import (
	"strings"
	"testing"

	"syntopica-backend/internal/platform/jsonutil"
)

func TestParseNarrativeResponse_ValidWrappedJSON(t *testing.T) {
	input := `{"narratives": [` +
		`{"title": "AI崛起", "summary": "人工智能快速发展", "status": "emerging", "related_tag_ids": [1, 2], "parent_ids": [], "confidence_score": 0.9},` +
		`{"title": "气候行动", "summary": "全球气候谈判取得进展", "status": "continuing", "related_tag_ids": [3], "parent_ids": [10], "confidence_score": 0.7}` +
		`]}`

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 narratives, got %d", len(result))
	}
	if result[0].Title != "AI崛起" {
		t.Errorf("first title: got %q", result[0].Title)
	}
	if result[0].Status != "emerging" {
		t.Errorf("first status: got %q", result[0].Status)
	}
	if result[0].ConfidenceScore != 0.9 {
		t.Errorf("first confidence: got %f", result[0].ConfidenceScore)
	}
	if result[1].Title != "气候行动" {
		t.Errorf("second title: got %q", result[1].Title)
	}
	if len(result[1].ParentIDs) != 1 || result[1].ParentIDs[0] != 10 {
		t.Errorf("second parent_ids: got %v", result[1].ParentIDs)
	}
}

func TestParseNarrativeResponse_ValidDirectArray(t *testing.T) {
	input := `[{"title": "测试叙事", "summary": "一段摘要", "status": "ending", "related_tag_ids": [5], "parent_ids": [1, 2], "confidence_score": 0.5}]`

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 narrative, got %d", len(result))
	}
	if result[0].Title != "测试叙事" {
		t.Errorf("title: got %q", result[0].Title)
	}
	if result[0].Status != "ending" {
		t.Errorf("status: got %q", result[0].Status)
	}
}

func TestParseNarrativeResponse_EmptyTitleIncluded(t *testing.T) {
	input := `{"narratives": [{"title": "", "summary": "有摘要但没标题", "status": "emerging", "related_tag_ids": [], "parent_ids": [], "confidence_score": 0.3}]}`

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("parseNarrativeResponse does not filter; expected 1, got %d", len(result))
	}
	if result[0].Title != "" {
		t.Errorf("expected empty title, got %q", result[0].Title)
	}
}

func TestParseNarrativeResponse_EmptySummaryIncluded(t *testing.T) {
	input := `{"narratives": [{"title": "有标题无摘要", "summary": "", "status": "emerging", "related_tag_ids": [], "parent_ids": [], "confidence_score": 0.4}]}`

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("parseNarrativeResponse does not filter; expected 1, got %d", len(result))
	}
}

func TestParseNarrativeResponse_EmptyWrappedArray(t *testing.T) {
	input := `{"narratives": []}`

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error for empty wrapped array: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 narratives, got %d", len(result))
	}
}

func TestParseNarrativeResponse_InvalidJSON(t *testing.T) {
	_, err := parseNarrativeResponse("this is not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse narrative JSON") {
		t.Errorf("error message: got %q", err.Error())
	}
}

func TestParseNarrativeResponse_MarkdownFenced(t *testing.T) {
	inner := `[{"title": "联邦叙事", "summary": "测试摘要", "status": "merging", "related_tag_ids": [7], "parent_ids": [], "confidence_score": 0.6}]`
	input := "```json\n" + inner + "\n```"

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 narrative, got %d", len(result))
	}
	if result[0].Title != "联邦叙事" {
		t.Errorf("title: got %q", result[0].Title)
	}
}

func TestParseNarrativeResponse_MissingStatus(t *testing.T) {
	input := `{"narratives": [{"title": "无状态", "summary": "没有status字段", "related_tag_ids": [], "parent_ids": [], "confidence_score": 0.5}]}`

	result, err := parseNarrativeResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 narrative, got %d", len(result))
	}
	if result[0].Status != "" {
		t.Errorf("missing status should produce empty string, got %q", result[0].Status)
	}
}

func TestStripMarkdownFence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no fence", `{"a":1}`, `{"a":1}`},
		{"json fence", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"bare fence", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"trailing whitespace", "```json\n{\"a\":1}\n```  \n", `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonutil.SanitizeLLMJSON(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeLLMJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateNarrativeOutputs_FiltersEmptyTitle(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "", Summary: "有摘要", Status: "emerging", RelatedTagIDs: []uint{1}},
		{Title: "有效标题", Summary: "有效摘要", Status: "emerging", RelatedTagIDs: []uint{1}},
	}
	tags := []TagInput{{ID: 1, Label: "test"}}
	result := validateNarrativeOutputs(outputs, tags, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 after filtering empty title, got %d", len(result))
	}
	if result[0].Title != "有效标题" {
		t.Errorf("expected '有效标题', got %q", result[0].Title)
	}
}

func TestValidateNarrativeOutputs_FiltersEmptySummary(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "有标题", Summary: "", Status: "emerging", RelatedTagIDs: []uint{1}},
	}
	tags := []TagInput{{ID: 1, Label: "test"}}
	result := validateNarrativeOutputs(outputs, tags, nil)
	if len(result) != 0 {
		t.Fatalf("expected 0 after filtering empty summary, got %d", len(result))
	}
}

func TestValidateNarrativeOutputs_FixesInvalidStatus(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "测试", Summary: "摘要", Status: "unknown_status", RelatedTagIDs: []uint{1}},
	}
	tags := []TagInput{{ID: 1, Label: "test"}}
	result := validateNarrativeOutputs(outputs, tags, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Status != "emerging" {
		t.Errorf("invalid status should be fixed to 'emerging', got %q", result[0].Status)
	}
}

func TestValidateNarrativeOutputs_FiltersHallucinatedTagIDs(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "叙事", Summary: "摘要", Status: "emerging", RelatedTagIDs: []uint{1, 999, 2}},
	}
	tags := []TagInput{{ID: 1, Label: "tag1"}, {ID: 2, Label: "tag2"}}
	result := validateNarrativeOutputs(outputs, tags, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if len(result[0].RelatedTagIDs) != 2 || result[0].RelatedTagIDs[0] != 1 || result[0].RelatedTagIDs[1] != 2 {
		t.Errorf("hallucinated tag 999 should be filtered, got %v", result[0].RelatedTagIDs)
	}
}

func TestValidateNarrativeOutputs_DropsOutputWithNoValidTags(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "全幻觉", Summary: "全伪造ID", Status: "emerging", RelatedTagIDs: []uint{888, 999}},
		{Title: "有效", Summary: "有效摘要", Status: "emerging", RelatedTagIDs: []uint{1}},
	}
	tags := []TagInput{{ID: 1, Label: "test"}}
	result := validateNarrativeOutputs(outputs, tags, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 after dropping all-hallucinated, got %d", len(result))
	}
	if result[0].Title != "有效" {
		t.Errorf("expected '有效', got %q", result[0].Title)
	}
}

func TestValidateNarrativeOutputs_FiltersHallucinatedParentIDs(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "叙事", Summary: "摘要", Status: "continuing", RelatedTagIDs: []uint{1}, ParentIDs: []uint{10, 999}},
	}
	tags := []TagInput{{ID: 1, Label: "test"}}
	prev := []PreviousNarrative{{ID: 10, Title: "昨日叙事"}}
	result := validateNarrativeOutputs(outputs, tags, prev)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if len(result[0].ParentIDs) != 1 || result[0].ParentIDs[0] != 10 {
		t.Errorf("hallucinated parent 999 should be filtered, got %v", result[0].ParentIDs)
	}
}

func TestValidateNarrativeOutputs_ClearsParentIDsWhenNoPrev(t *testing.T) {
	outputs := []NarrativeOutput{
		{Title: "叙事", Summary: "摘要", Status: "continuing", RelatedTagIDs: []uint{1}, ParentIDs: []uint{10}},
	}
	tags := []TagInput{{ID: 1, Label: "test"}}
	result := validateNarrativeOutputs(outputs, tags, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if len(result[0].ParentIDs) != 0 {
		t.Errorf("parent_ids should be cleared when no prev narratives, got %v", result[0].ParentIDs)
	}
}
