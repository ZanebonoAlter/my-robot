package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"syntopica-backend/internal/topicgraph/repository"
)

func tagInput(id uint, label, desc, ctx string) repository.TagInput {
	return repository.TagInput{ID: id, Label: label, Description: desc, ArticleContext: ctx}
}

func TestBuildSectionEmbedText_MultiTagAssembly(t *testing.T) {
	tags := []repository.TagInput{
		tagInput(1, "美伊谈判", "美国与伊朗就核协议谈判", "《谈判》双方代表团抵达；"),
		tagInput(2, "油价波动", "", ""),
	}
	got := buildSectionEmbedText(tags, nil, "")

	lines := strings.Split(got, "\n")
	assert.Len(t, lines, 2)
	assert.Equal(t, "美伊谈判：美国与伊朗就核协议谈判；代表文章：《谈判》双方代表团抵达；", lines[0])
	// Empty description / ArticleContext are omitted entirely.
	assert.Equal(t, "油价波动", lines[1])
}

func TestBuildSectionEmbedText_ArticleContextTruncatedPerTag(t *testing.T) {
	long := strings.Repeat("卷", 500)
	got := buildSectionEmbedText([]repository.TagInput{tagInput(1, "标签", "", long)}, nil, "")
	// label(2) + "；代表文章："(6 runes) → total = 2+6+100 = 108
	assert.Equal(t, 108, utf8.RuneCountInString(got))
	assert.True(t, strings.HasSuffix(got, strings.Repeat("卷", 100)))
}

func TestBuildSectionEmbedText_TotalTruncated(t *testing.T) {
	// 10 tags × 300 runes each ≈ 3000 runes → capped at maxSectionEmbedRunes
	// (480, sized for the embedding gateway's 512-token input limit).
	var tags []repository.TagInput
	for i := 0; i < 10; i++ {
		tags = append(tags, tagInput(uint(i+1), strings.Repeat("甲", 300), "", ""))
	}
	got := buildSectionEmbedText(tags, nil, "")
	assert.Equal(t, maxSectionEmbedRunes, utf8.RuneCountInString(got))
}

func TestBuildSectionEmbedText_FallbackToThreadTitles(t *testing.T) {
	threads := []repository.DailyReportThread{
		{Title: "线索一"},
		{Title: "  "}, // blank titles skipped
		{Title: "线索二"},
	}
	got := buildSectionEmbedText(nil, threads, "兜底标题")
	assert.Equal(t, "线索一\n线索二", got)
}

func TestBuildSectionEmbedText_FallbackToClusterLabel(t *testing.T) {
	got := buildSectionEmbedText(nil, nil, " 兜底标题 ")
	assert.Equal(t, "兜底标题", got)
}

func TestBuildSectionEmbedText_BlankTagsFallThrough(t *testing.T) {
	// Whitespace-only tag fields produce an empty main text → thread fallback.
	tags := []repository.TagInput{tagInput(1, "  ", " ", "")}
	got := buildSectionEmbedText(tags, []repository.DailyReportThread{{Title: "线索"}}, "标题")
	assert.Equal(t, "线索", got)
}

func TestTruncateRunesMax(t *testing.T) {
	assert.Equal(t, "abc", truncateRunesMax("abc", 5))
	assert.Equal(t, "ab", truncateRunesMax("abc", 2))
	assert.Equal(t, "中文", truncateRunesMax("中文测试", 2))
}
