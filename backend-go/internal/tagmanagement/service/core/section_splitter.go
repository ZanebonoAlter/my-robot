package core

import "strings"

// Section is one column-level slice of an aggregate article's markdown summary.
type Section struct {
	Title   string // 栏目标题（h2 文本，细分片时保持父 h2 标题）
	Content string // 栏目正文（不含标题行本身；细分片的 Content 含 ### 子标题行）
}

const (
	// minSectionRunes: 短于此长度的栏目正文并入相邻片
	minSectionRunes = 300
	// maxSectionRunes: 超过此长度的栏目在有 ### 子标题时细分
	maxSectionRunes = 3000
	// maxSections: 切片数上限，超出时从尾部合并
	maxSections = 8
)

// splitSections splits an aggregate article's markdown summary into column-level
// sections. It is pure code and never issues LLM calls. Summaries without any
// `## ` heading yield nil so the caller can fall back to the mono path.
func splitSections(markdown string) []Section {
	sections := collectH2Sections(markdown)
	sections = dropIntroSections(sections)
	if len(sections) == 0 {
		return nil
	}
	sections = mergeShortSections(sections)
	sections = splitLongSections(sections)
	sections = capSectionCount(sections)
	if len(sections) == 0 {
		return nil
	}
	return sections
}

func collectH2Sections(markdown string) []Section {
	lines := strings.Split(markdown, "\n")
	var sections []Section
	var current *Section
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.Content = strings.TrimSpace(current.Content)
				sections = append(sections, *current)
			}
			current = &Section{Title: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
			continue
		}
		if current != nil {
			current.Content += line + "\n"
		}
	}
	if current != nil {
		current.Content = strings.TrimSpace(current.Content)
		sections = append(sections, *current)
	}
	return sections
}

func dropIntroSections(sections []Section) []Section {
	result := make([]Section, 0, len(sections))
	for _, s := range sections {
		if strings.Contains(s.Title, "导读") {
			continue
		}
		result = append(result, s)
	}
	return result
}

// mergeShortSections merges sections whose content is shorter than minSectionRunes
// into adjacent sections. Consecutive short sections accumulate in a carry; when
// the carry plus the current piece reaches minSectionRunes they merge into one
// section under the carry's first title. A long section absorbs any pending carry
// (short pieces merge forward, the target keeps its own title). Leftover carry at
// the end merges into the previous section, or stands alone if none exists.
func mergeShortSections(sections []Section) []Section {
	var result []Section
	var carry []Section
	carryRunes := 0

	emitCarryStandalone := func() {
		head := carry[0]
		var b strings.Builder
		b.WriteString(head.Content)
		for _, s := range carry[1:] {
			writeMergedSection(&b, s)
		}
		result = append(result, Section{Title: head.Title, Content: strings.TrimSpace(b.String())})
		carry = nil
		carryRunes = 0
	}
	prependCarryTo := func(target *Section) {
		var b strings.Builder
		for _, s := range carry {
			writeMergedSection(&b, s)
		}
		b.WriteString(target.Content)
		target.Content = strings.TrimSpace(b.String())
		carry = nil
		carryRunes = 0
	}

	for _, s := range sections {
		runes := len([]rune(s.Content))
		if runes >= minSectionRunes {
			if len(carry) > 0 {
				prependCarryTo(&s)
			}
			result = append(result, s)
			continue
		}
		carry = append(carry, s)
		carryRunes += runes
		if carryRunes >= minSectionRunes {
			emitCarryStandalone()
		}
	}

	if len(carry) > 0 {
		if len(result) > 0 {
			prependCarryTo(&result[len(result)-1])
		} else {
			emitCarryStandalone()
		}
	}
	return result
}

// writeMergedSection appends a merged-away section's title and content into the
// target content so the information stays visible to the extractor.
func writeMergedSection(b *strings.Builder, s Section) {
	b.WriteString("## " + s.Title + "\n")
	b.WriteString(s.Content)
	b.WriteString("\n\n")
}

// splitLongSections subdivides sections longer than maxSectionRunes that contain
// `### ` sub-headings into sub-sections (keeping the parent h2 title). Sub-pieces
// shorter than minSectionRunes merge with adjacent sub-pieces (forward first).
func splitLongSections(sections []Section) []Section {
	result := make([]Section, 0, len(sections))
	for _, s := range sections {
		if len([]rune(s.Content)) <= maxSectionRunes || (!strings.Contains(s.Content, "\n### ") && !strings.HasPrefix(s.Content, "### ")) {
			result = append(result, s)
			continue
		}
		sub := splitByH3(s.Content)
		sub = mergeShortSubSections(sub)
		for _, piece := range sub {
			result = append(result, Section{Title: s.Title, Content: piece})
		}
	}
	return result
}

func splitByH3(content string) []string {
	lines := strings.Split(content, "\n")
	var pieces []string
	var current strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			if current.Len() > 0 {
				pieces = append(pieces, strings.TrimSpace(current.String()))
			}
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		pieces = append(pieces, strings.TrimSpace(current.String()))
	}
	return pieces
}

// mergeShortSubSections merges sub-pieces shorter than minSectionRunes with
// adjacent sub-pieces, preferring forward merges; a trailing short piece merges
// backward into the previous one.
func mergeShortSubSections(pieces []string) []string {
	result := make([]string, 0, len(pieces))
	for i := 0; i < len(pieces); i++ {
		piece := pieces[i]
		if len([]rune(piece)) >= minSectionRunes {
			result = append(result, piece)
			continue
		}
		if i+1 < len(pieces) {
			pieces[i+1] = piece + "\n\n" + pieces[i+1]
			continue
		}
		if len(result) > 0 {
			result[len(result)-1] += "\n\n" + piece
			continue
		}
		result = append(result, piece)
	}
	return result
}

// capSectionCount merges sections from the tail (appending the last section's
// title+content to the second-to-last) until at most maxSections remain.
func capSectionCount(sections []Section) []Section {
	for len(sections) > maxSections {
		last := sections[len(sections)-1]
		prev := &sections[len(sections)-2]
		var b strings.Builder
		b.WriteString(prev.Content)
		b.WriteString("\n\n")
		writeMergedSection(&b, last)
		prev.Content = strings.TrimSpace(b.String())
		sections = sections[:len(sections)-1]
	}
	return sections
}
