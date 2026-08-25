package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"gorm.io/gorm/clause"

	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/tracing"
	"syntopica-backend/internal/topicgraph/repository"
)

// Keyword watch expressions (watch-keyword-and-quickadd design §4.2):
//
//	'｜'-separated OR groups, whitespace-separated AND terms within a group.
//	"ASML|镓锗 出口" ⇒ (ASML OR 镓锗) AND 出口. Whitespace = half-width
//	space, full-width space (U+3000), tab. Matching is case-insensitive and
//	literal (regex metacharacters / emoji never interpreted).
//
// The keyword track is deterministic: zero AI calls, the hit reason is the
// mechanical text 含关键字『XX』.

// KeywordInstantWindowDays is the default instant-match lookback window in
// days (today inclusive) applied when a keyword watch is created.
const KeywordInstantWindowDays = 14

// ValidateKeywordExpr reports whether a keyword watch expression parses to at
// least one valid OR group. Exported for handler-side creation validation.
func ValidateKeywordExpr(expr string) bool {
	return len(parseKeywordExpr(expr)) > 0
}

// parseKeywordExpr parses a keyword expression into its disjunctive-normal-
// form groups: a section hits when ANY returned group has ALL its terms
// present. Mechanically: split top-level whitespace AND factors, split each
// factor on '|' into OR alternatives, then cross-multiply alternatives into
// conjunction groups. e.g. "ASML|镓锗 出口" → [[ASML,出口],[镓锗,出口]] —
// net semantics (ASML OR 镓锗) AND 出口 (design §4.2).
//
// A trailing '|' (after right-trimming whitespace — "ASML|", "|", "||") reads
// as an unfinished expression and invalidates the whole input (nil return);
// empty alternatives elsewhere are silently dropped ("| 出口" → [[出口]],
// "出口 || ASML" → [[出口,ASML]]). Case is preserved here — matching
// lowercases both sides.
func parseKeywordExpr(expr string) [][]string {
	if strings.HasSuffix(strings.TrimRight(expr, whitespaceCutset), "|") {
		return nil // unfinished expression (trailing '|')
	}
	factors := strings.FieldsFunc(expr, isKeywordWhitespace)
	if len(factors) == 0 {
		return nil
	}
	groups := [][]string{nil}
	for _, factor := range factors {
		var alts []string
		for _, alt := range strings.Split(factor, "|") {
			if alt != "" {
				alts = append(alts, alt)
			}
		}
		if len(alts) == 0 {
			continue // bare separator factor (e.g. "|") contributes nothing
		}
		var next [][]string
		for _, g := range groups {
			for _, a := range alts {
				g2 := make([]string, 0, len(g)+1)
				g2 = append(g2, g...)
				g2 = append(g2, a)
				next = append(next, g2)
			}
		}
		groups = next
	}
	if len(groups) == 1 && len(groups[0]) == 0 {
		return nil // no factor contributed any term
	}
	return groups
}

// whitespaceCutset / isKeywordWhitespace define the AND-term separator set:
// half-width space, full-width space (U+3000), tab.
const whitespaceCutset = " \u3000\t"

func isKeywordWhitespace(r rune) bool {
	return r == ' ' || r == '\u3000' || r == '\t'
}

// KeywordHit is one matched section for a keyword watch: the section identity
// plus the expression terms literally found in its threads text.
type KeywordHit struct {
	SectionID    uint
	ReportID     uint
	PeriodDate   time.Time
	MatchedWords []string
}

// matchKeywordSections matches a keyword expression against each section's
// threads text (every thread's title+summary, concatenated, lowercased). A
// section hits when ANY conjunction group of the expression has ALL its
// terms present (case-insensitive literal Contains) — net semantics:
// whitespace = AND, '|' = OR (design §4.2). Sections without threads never
// hit. An invalid expression matches nothing.
func matchKeywordSections(expr string, sections []repository.SectionText) []KeywordHit {
	groups := parseKeywordExpr(expr)
	if len(groups) == 0 {
		return nil
	}
	var hits []KeywordHit
	for _, s := range sections {
		if len(s.Threads) == 0 {
			continue
		}
		text := strings.ToLower(sectionThreadsText(s.Threads))
		matched, ok := matchKeywordGroups(groups, text)
		if !ok {
			continue
		}
		hits = append(hits, KeywordHit{
			SectionID:    s.SectionID,
			ReportID:     s.ReportID,
			PeriodDate:   s.PeriodDate,
			MatchedWords: matched,
		})
	}
	return hits
}

// sectionThreadsText concatenates a section's threads text: title + summary
// per thread. A thread missing its summary degrades to title-only (legal).
func sectionThreadsText(threads []repository.ThreadText) string {
	var sb strings.Builder
	for _, th := range threads {
		sb.WriteString(th.Title)
		sb.WriteString("\n")
		sb.WriteString(th.Summary)
		sb.WriteString("\n")
	}
	return sb.String()
}

// matchKeywordGroups evaluates the DNF conjunction groups against lowercased
// text: hit when ANY group has ALL its terms present. Returns the present
// terms (union over matching groups, expression order, deduplicated) and
// whether any group matched.
func matchKeywordGroups(groups [][]string, lowerText string) ([]string, bool) {
	var matched []string
	seen := make(map[string]bool)
	anyHit := false
	for _, group := range groups {
		groupHit := true
		for _, term := range group {
			if !strings.Contains(lowerText, strings.ToLower(term)) {
				groupHit = false
				break
			}
			if !seen[term] {
				seen[term] = true
				matched = append(matched, term)
			}
		}
		if groupHit {
			anyHit = true
		}
	}
	if !anyHit {
		return nil, false
	}
	return matched, true
}

// buildKeywordHitReason renders the mechanical keyword-hit reason, e.g.
// 含关键字『ASML、出口』 (multiple hit words joined by 、). Deterministic,
// zero AI involved (design §4.3).
func buildKeywordHitReason(words []string) string {
	return "含关键字『" + strings.Join(words, "、") + "』"
}

// MatchKeywordInstant scans the board's sections within the last sinceDays
// (today inclusive) for the given keyword watch and upserts the matched hits
// with OnConflict DoNothing — idempotent with daily-report-time matching on
// the (watch_id, section_id, report_id) unique index. Returns the number of
// matched sections (for the creation-response feedback banner).
// Label watches never instant-match (returns 0, nil — defensive guard).
func MatchKeywordInstant(ctx context.Context, boardID, watchID uint, sinceDays int) (int, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "service.MatchKeywordInstant")
	defer span.End()
	return matchKeywordInstantAt(ctx, boardID, watchID, sinceDays, time.Now())
}

// watchWindowStart returns the inclusive date-granular lower bound of a
// sinceDays lookback ending today: 14 days ⇒ [today-14, today] — the 14th
// day back is inside the window, the 15th is not.
func watchWindowStart(now time.Time, sinceDays int) time.Time {
	if sinceDays < 0 {
		sinceDays = 0
	}
	return repository.NormalizeReportDate(now).AddDate(0, 0, -sinceDays)
}

// matchKeywordInstantAt is the testable core of MatchKeywordInstant with an
// injectable clock for window-boundary tests.
func matchKeywordInstantAt(ctx context.Context, boardID, watchID uint, sinceDays int, now time.Time) (int, error) {
	watch, err := repository.Repo.GetWatchByID(watchID)
	if err != nil {
		return 0, fmt.Errorf("load watch: %w", err)
	}
	if watch.Type != repository.WatchTypeKeyword {
		return 0, nil // label watches never instant-match
	}
	texts, err := repository.Repo.ListWatchSectionTextsSince(boardID, watchWindowStart(now, sinceDays))
	if err != nil {
		return 0, fmt.Errorf("load section texts: %w", err)
	}
	hits := matchKeywordSections(watch.Label, texts)
	if len(hits) == 0 {
		return 0, nil
	}
	rows := make([]repository.TopicWatchHit, 0, len(hits))
	for _, h := range hits {
		rows = append(rows, repository.TopicWatchHit{
			WatchID:   watchID,
			SectionID: h.SectionID,
			// Design §4.4: report_id = the section's owning report,
			// period_date = the section's date (so the hit surfaces on the
			// right historical report's watch bar).
			ReportID:   h.ReportID,
			PeriodDate: h.PeriodDate,
			Reason:     buildKeywordHitReason(h.MatchedWords),
		})
	}
	if err := repository.Repo.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "watch_id"}, {Name: "section_id"}, {Name: "report_id"}},
		DoNothing: true,
	}).Create(&rows).Error; err != nil {
		return 0, fmt.Errorf("write instant keyword hits: %w", err)
	}
	logging.Infof("topic-watch: instant keyword match wrote %d hits for watch %d board %d",
		len(rows), watchID, boardID)
	return len(rows), nil
}
