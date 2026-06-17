package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// ExportSpec describes how a single table is exported and sanitized.
type ExportSpec struct {
	Table   string   // physical table name
	Columns []string // explicit column list for SELECT + INSERT (excludes non-DB fields like articles.category_id)
	// Where filters rows. Use a printf-style placeholder ":days" replaced with
	// the configured export window at runtime. Empty means no filter.
	Where string
	// VectorColumns are columns whose SELECT projection is replaced with
	// NULL::vector, so the embedding never leaves the source DB.
	VectorColumns map[string]bool
	// Sanitizers map column name → transformation applied to the raw string
	// value before it is emitted as an INSERT literal.
	Sanitizers map[string]func(string) string
	// NoSequence is true for composite-key tables that have no `id` sequence
	// (e.g. board_composition). Skips the trailing setval() statement.
	NoSequence bool
}

// --- Sanitizer implementations ------------------------------------------------

// clearAll returns the empty string for any input.
func clearAll(string) string { return "" }

// emptyJSON returns a JSON object literal for jsonb columns that may carry creds.
func emptyJSON(string) string { return "{}" }

// truncateContent caps a long text field (e.g. article body) at maxLen
// characters, appending an ellipsis marker when truncated. This keeps the demo
// readable while shrinking the seed file, since the full text is not needed to
// browse the product.
func truncateContent(maxLen int) func(string) string {
	return func(s string) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen] + "…[truncated for demo]"
	}
}

// stripQuery removes the query string from a URL to drop tracking tokens.
// If the value is not a valid URL, it is returned unchanged.
func stripQuery(s string) string {
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return s
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// sha256Hash deterministically hashes a value so joins on session_id still work
// without leaking the original identifier. Implemented in main.go (uses sha256).
func sha256Hash(s string) string { return hashHex(s) }

// hashHex returns the lowercase hex SHA-256 of s.
func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// --- SQL literal formatting ---------------------------------------------------

// quoteString single-quotes a string for PostgreSQL, doubling embedded single
// quotes. Backslashes are not special under standard_conforming_strings = on
// (the PostgreSQL default since 9.1), so they are left untouched.
//
// All exported values are emitted as quoted literals: PostgreSQL implicitly
// coerces an unknown-type quoted literal into the target column type (integer,
// numeric, boolean, timestamp, jsonb, ...), so a single quoting path covers
// every column. NULLs are emitted separately (as the unquoted keyword) by the
// exporter when a scanned value is not valid.
func quoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
