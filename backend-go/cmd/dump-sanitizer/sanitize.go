package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
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
	// ConflictClause is appended before the INSERT terminator for tables whose
	// sanitized values may collapse unique keys, e.g. stripped feed URLs.
	ConflictClause string
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
// browse the product. Slicing is rune-based so multi-byte UTF-8 (e.g. Chinese)
// is never split mid-character, which would corrupt the output.
func truncateContent(maxLen int) func(string) string {
	return func(s string) string {
		if len([]rune(s)) <= maxLen {
			return s
		}
		return string([]rune(s)[:maxLen]) + "…[truncated for demo]"
	}
}

func composeSanitizers(fns ...func(string) string) func(string) string {
	return func(s string) string {
		for _, fn := range fns {
			s = fn(s)
		}
		return s
	}
}

func redactSensitiveTokens(s string) string {
	replacer := strings.NewReplacer(
		"api_key", "[redacted-token]",
		"API_KEY", "[redacted-token]",
		"api-key", "[redacted-token]",
		"API-Key", "[redacted-token]",
	)
	return replacer.Replace(s)
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

// defaultRSSHubRewrite rewrites this demo's self-hosted RSSHub to the public
// instance. Operators with a different source host override via RSSHUB_REWRITE.
const defaultRSSHubRewrite = "rsshub.app=rsshub.app"

// rewriteRSSHubHost rewrites a self-hosted RSSHub host to the public instance
// so the demo never leaks private infrastructure (e.g. an operator's public IP).
// Configured via the RSSHUB_REWRITE env var as "sourceHost=targetHost"; falls
// back to defaultRSSHubRewrite when unset. The target is always served over
// https. Non-URL or non-matching values pass through untouched.
func rewriteRSSHubHost(s string) string {
	rule := strings.TrimSpace(os.Getenv("RSSHUB_REWRITE"))
	if rule == "" {
		rule = defaultRSSHubRewrite
	}
	if s == "" {
		return s
	}
	parts := strings.SplitN(rule, "=", 2)
	if len(parts) != 2 {
		return s
	}
	srcHost, dstHost := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if srcHost == "" || dstHost == "" {
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Host != srcHost {
		return s
	}
	u.Scheme = "https"
	u.Host = dstHost
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
