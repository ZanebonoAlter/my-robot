package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"syntopica-backend/internal/topicgraph/repository"
)

// ── parseKeywordExpr: the V1-V8 variant matrix + whitebox branch table B1-B7 ──

func TestParseKeywordExpr_Empty(t *testing.T) {
	assert.Nil(t, parseKeywordExpr(""))
}

func TestParseKeywordExpr_WhitespaceOnly(t *testing.T) {
	assert.Nil(t, parseKeywordExpr("　 \t")) // full-width + tab + half-width
	assert.Nil(t, parseKeywordExpr("   "))  // half-width only
	assert.Nil(t, parseKeywordExpr("\t\t")) // tabs only
	assert.Nil(t, parseKeywordExpr("　"))    // full-width only (U+3000)
}

func TestParseKeywordExpr_Delimiters(t *testing.T) {
	// Single bare separator.
	assert.Nil(t, parseKeywordExpr("|"))
	// Consecutive separators.
	assert.Nil(t, parseKeywordExpr("||"))
	// Trailing '|' = unfinished expression → whole input invalid.
	assert.Nil(t, parseKeywordExpr("ASML|"))
	// Leading '|' = empty first group silently dropped.
	assert.Equal(t, [][]string{{"出口"}}, parseKeywordExpr("| 出口"))
	// Empty group in the middle is dropped too — factors cross-multiply.
	assert.Equal(t, [][]string{{"出口", "ASML"}, {"出口", "镓锗"}}, parseKeywordExpr("出口 || ASML|镓锗"))
}

func TestParseKeywordExpr_Single(t *testing.T) {
	assert.Equal(t, [][]string{{"ASML"}}, parseKeywordExpr("ASML"))
}

func TestParseKeywordExpr_And(t *testing.T) {
	assert.Equal(t, [][]string{{"出口", "限制"}}, parseKeywordExpr("出口 限制"))
	// Whitespace variants inside AND terms (half/full-width, tab).
	assert.Equal(t, [][]string{{"出口", "限制", "新规"}}, parseKeywordExpr("出口　限制\t新规"))
}

func TestParseKeywordExpr_Or(t *testing.T) {
	assert.Equal(t, [][]string{{"ASML"}, {"镓锗"}}, parseKeywordExpr("ASML|镓锗"))
}

func TestParseKeywordExpr_Mixed(t *testing.T) {
	// "ASML|镓锗 出口" → (ASML OR 镓锗) AND 出口. Splits '|' first, then
	// whitespace: the AND term lands in BOTH groups.
	assert.Equal(t, [][]string{{"ASML", "出口"}, {"镓锗", "出口"}}, parseKeywordExpr("ASML|镓锗 出口"))
}

func TestParseKeywordExpr_LiteralMetacharacters(t *testing.T) {
	// Regex metacharacters and emoji are literal text, never interpreted (V6/V8).
	assert.Equal(t, [][]string{{"C++"}}, parseKeywordExpr("C++"))
	assert.Equal(t, [][]string{{".*"}}, parseKeywordExpr(".*"))
	assert.Equal(t, [][]string{{"（）"}}, parseKeywordExpr("（）"))
	assert.Equal(t, [][]string{{"🇺🇸", "制裁"}}, parseKeywordExpr("🇺🇸 制裁"))
	// Case is preserved by the parser (matching lowercases both sides).
	assert.Equal(t, [][]string{{"ASML"}}, parseKeywordExpr("ASML"))
}

func TestValidateKeywordExpr(t *testing.T) {
	assert.True(t, ValidateKeywordExpr("ASML"))
	assert.True(t, ValidateKeywordExpr("ASML|镓锗 出口"))
	assert.True(t, ValidateKeywordExpr(" 出口 ")) // surrounding whitespace trimmed by term split
	assert.False(t, ValidateKeywordExpr(""))
	assert.False(t, ValidateKeywordExpr("　\t "))
	assert.False(t, ValidateKeywordExpr("|"))
	assert.False(t, ValidateKeywordExpr("ASML|"))
	assert.False(t, ValidateKeywordExpr("||"))
}

// ── matchKeywordSections ──

func sectionText(id uint, threads ...repository.ThreadText) repository.SectionText {
	return repository.SectionText{
		SectionID: id,
		ReportID:  id * 100,
		Threads:   threads,
	}
}

func TestMatchKeywordSections_AndAllPresent(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "出口管制升级", Summary: "镓锗出口受限"}),
		sectionText(2, repository.ThreadText{Title: "无关内容", Summary: "出口增长强劲"}),
	}
	hits := matchKeywordSections("出口 限制", sections)
	assert.Empty(t, hits, "section2 has 出口 but not 限制 → AND missing → no hit")

	sections3 := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "出口新规", Summary: "限制措施出台"}),
	}
	hits = matchKeywordSections("出口 限制", sections3)
	assert.Len(t, hits, 1, "both AND terms present (title+summary combined) → hit")
	assert.Equal(t, uint(1), hits[0].SectionID)
	assert.ElementsMatch(t, []string{"出口", "限制"}, hits[0].MatchedWords)
}

func TestMatchKeywordSections_AndMissing(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "出口增长", Summary: "出口数据创新高"}),
	}
	assert.Empty(t, matchKeywordSections("出口 限制", sections))
}

func TestMatchKeywordSections_OrAny(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "镓锗收储", Summary: "稀有金属管理"}),
	}
	hits := matchKeywordSections("ASML|镓锗", sections)
	assert.Len(t, hits, 1)
	assert.Equal(t, []string{"镓锗"}, hits[0].MatchedWords)
}

func TestMatchKeywordSections_MixedOrAnd(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "ASML财报", Summary: "对华出口政策"}),
		sectionText(2, repository.ThreadText{Title: "镓锗管制", Summary: "出口收紧"}),
		sectionText(3, repository.ThreadText{Title: "ASML财报", Summary: "营收超预期"}),
	}
	hits := matchKeywordSections("ASML|镓锗 出口", sections)
	assert.Len(t, hits, 2, "sections 1+2 hit (OR term + AND term 出口); 3 misses the AND term")
	assert.ElementsMatch(t, []uint{1, 2}, []uint{hits[0].SectionID, hits[1].SectionID})
}

func TestMatchKeywordSections_CaseInsensitiveBothDirections(t *testing.T) {
	lowerExpr := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "ASML 光刻机", Summary: ""}),
	}
	hits := matchKeywordSections("asml", lowerExpr)
	assert.Len(t, hits, 1, "expr asml matches text ASML")

	upperExpr := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "asml 光刻机", Summary: ""}),
	}
	hits = matchKeywordSections("ASML", upperExpr)
	assert.Len(t, hits, 1, "expr ASML matches text asml")
}

func TestMatchKeywordSections_NoThreads(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1), // no threads → never hits
		sectionText(2, repository.ThreadText{Title: "ASML", Summary: ""}),
	}
	hits := matchKeywordSections("ASML", sections)
	assert.Len(t, hits, 1)
	assert.Equal(t, uint(2), hits[0].SectionID)
}

func TestMatchKeywordSections_TitleOnlyThread(t *testing.T) {
	// Thread missing its summary degrades to title-only text (P2).
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "霍尔木兹海峡局势", Summary: ""}),
	}
	hits := matchKeywordSections("霍尔木兹", sections)
	assert.Len(t, hits, 1)
}

func TestMatchKeywordSections_LiteralNotRegex(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "C++ 标准更新", Summary: ""}),
		sectionText(2, repository.ThreadText{Title: "CAA 标准更新", Summary: ""}),
	}
	hits := matchKeywordSections("C++", sections)
	assert.Len(t, hits, 1, "'C++' is literal: section1 hits, section2 (CAA) does not")
	assert.Equal(t, uint(1), hits[0].SectionID)
}

func TestMatchKeywordSections_InvalidExpr(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1, repository.ThreadText{Title: "ASML", Summary: ""}),
	}
	assert.Empty(t, matchKeywordSections("", sections))
	assert.Empty(t, matchKeywordSections("||", sections))
}

func TestMatchKeywordSections_MultipleThreadsConcatenated(t *testing.T) {
	sections := []repository.SectionText{
		sectionText(1,
			repository.ThreadText{Title: "半导体巨头财报", Summary: ""},
			repository.ThreadText{Title: "无关话题", Summary: "ASML 提及光刻机"},
		),
	}
	hits := matchKeywordSections("ASML", sections)
	assert.Len(t, hits, 1, "term in the second thread's summary still hits (all threads concatenated)")
}

// ── reason text ──

func TestBuildKeywordHitReason(t *testing.T) {
	assert.Equal(t, "含关键字『ASML』", buildKeywordHitReason([]string{"ASML"}))
	assert.Equal(t, "含关键字『ASML、出口』", buildKeywordHitReason([]string{"ASML", "出口"}))
}

// ── instant-match window boundary ──

func TestWatchWindowStart(t *testing.T) {
	now := mustDate("2026-08-24")
	norm := func(s string) time.Time { return repository.NormalizeReportDate(mustDate(s)) }
	// 14-day window ends today (inclusive): [08-10 .. 08-24] — the 14th day
	// back (08-10) is inside, the 15th (08-09) is not. NormalizeReportDate
	// pins dates to 12:00 UTC (the repo's timezone-collapse convention).
	assert.Equal(t, norm("2026-08-10"), watchWindowStart(now, 14))
	// 1-day window = [yesterday, today].
	assert.Equal(t, norm("2026-08-23"), watchWindowStart(now, 1))
	// Degenerate values clamp to today.
	assert.Equal(t, norm("2026-08-24"), watchWindowStart(now, 0))
	assert.Equal(t, norm("2026-08-24"), watchWindowStart(now, -3))
}

func mustDate(s string) (t time.Time) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
