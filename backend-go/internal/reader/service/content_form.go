package service

import (
	"regexp"
	"strings"
)

// validContentForms lists the only accepted content form mark values.
var validContentForms = map[string]bool{
	"mono":      true,
	"aggregate": true,
}

// contentFormMarkPattern matches an HTML comment like `<!-- form: mono -->`
// that appears on the first line of the summary (leading/trailing whitespace
// around the line is tolerated).
var contentFormMarkPattern = regexp.MustCompile(`(?m)^\s*<!--\s*form:\s*(\S+)\s*-->\s*$`)

// parseContentFormMark extracts the content form mark from the first line of
// an AI-generated summary. On success it returns the form value ("mono" or
// "aggregate") and the summary with the mark line stripped. If the first line
// carries no valid mark, it returns ("", summary) unchanged.
func parseContentFormMark(summary string) (form string, cleaned string) {
	if summary == "" {
		return "", summary
	}

	firstLineEnd := strings.IndexByte(summary, '\n')
	if firstLineEnd == -1 {
		firstLineEnd = len(summary)
	}
	firstLine := summary[:firstLineEnd]

	match := contentFormMarkPattern.FindStringSubmatch(firstLine)
	if match == nil {
		return "", summary
	}

	form = match[1]
	if !validContentForms[form] {
		return "", summary
	}

	rest := strings.TrimSpace(summary[firstLineEnd:])
	return form, rest
}
