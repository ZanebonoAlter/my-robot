package service

import (
	"strings"
	"testing"

	"syntopica-backend/internal/topicgraph/repository"
)

// TestBuildHighlightsPrompt_InjectsArticleContext verifies the highlights prompt carries
// representative article titles+summaries when ArticleContext is set, and omits the line when empty.
func TestBuildHighlightsPrompt_InjectsArticleContext(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "降准", ArticleCount: 5, ArticleContext: "《央行降准》下调0.5个百分点"},
		{ID: 2, Label: "无上下文标签", ArticleCount: 2}, // ArticleContext empty
	}
	got := buildHighlightsPrompt(tags, nil)
	if !strings.Contains(got, "《央行降准》下调0.5个百分点") {
		t.Errorf("highlights prompt should include article context, got:\n%s", got)
	}
	// empty ArticleContext tag must still list the tag but emit no context line
	if !strings.Contains(got, "无上下文标签") {
		t.Errorf("highlights prompt should still list the no-context tag, got:\n%s", got)
	}
}

// TestBuildThreadsPrompt_InjectsArticleContext verifies threads prompt carries article context.
func TestBuildThreadsPrompt_InjectsArticleContext(t *testing.T) {
	cluster := repository.ClusterGroup{GroupName: "货币政策", TagIDs: []uint{1}}
	tags := []repository.TagInput{
		{ID: 1, Label: "降准", ArticleCount: 5, ArticleContext: "《央行降准》释放长期资金"},
	}
	got := buildThreadsPrompt(cluster, tags)
	if !strings.Contains(got, "《央行降准》释放长期资金") {
		t.Errorf("threads prompt should include article context, got:\n%s", got)
	}
}

// TestBuildClusterPrompt_InjectsArticleContext verifies cluster prompt carries article context.
func TestBuildClusterPrompt_InjectsArticleContext(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "降准", ArticleCount: 5, ArticleContext: "《央行降准》释放约1万亿流动性"},
	}
	got := buildClusterPrompt(tags)
	if !strings.Contains(got, "《央行降准》释放约1万亿流动性") {
		t.Errorf("cluster prompt should include article context, got:\n%s", got)
	}
}
