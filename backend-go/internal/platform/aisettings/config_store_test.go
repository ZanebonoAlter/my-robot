package aisettings

import (
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/platform/testutil"
)

func TestParseValidHHMM(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"00:00", true},
		{"09:30", true},
		{"12:00", true},
		{"21:00", true},
		{"23:59", true},
		{"24:00", false},
		{"25:99", false},
		{"abc", false},
		{"", false},
		{"1:30", false},   // must be zero-padded
		{"09:60", false},  // minutes > 59
		{"09:5", false},   // must be zero-padded
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := hhmmPattern.MatchString(tt.input)
			if result != tt.valid {
				t.Errorf("hhmmPattern.MatchString(%q) = %v, want %v", tt.input, result, tt.valid)
			}
		})
	}
}

func TestDefaultDailyReportTime(t *testing.T) {
	if defaultDailyReportTime != "21:00" {
		t.Errorf("defaultDailyReportTime = %q, want %q", defaultDailyReportTime, "21:00")
	}
}

func TestProxyConfigKey(t *testing.T) {
	if proxyConfigKey != "http_proxy_config" {
		t.Errorf("proxyConfigKey = %q, want %q", proxyConfigKey, "http_proxy_config")
	}
}

func TestDefaultRSSHubDocBase(t *testing.T) {
	if defaultRSSHubDocBase != "https://docs.rsshub.app" {
		t.Errorf("defaultRSSHubDocBase = %q, want %q", defaultRSSHubDocBase, "https://docs.rsshub.app")
	}
	if rsshubDocBaseKey != "rsshub_doc_base" {
		t.Errorf("rsshubDocBaseKey = %q, want %q", rsshubDocBaseKey, "rsshub_doc_base")
	}
}

// TestLoadAnalysisPausedConfig_DefaultFalse covers the fail-open default: when
// the analysis_paused key is absent from ai_settings, Load must return
// (false, nil, nil) — analysis is NOT paused.
func TestLoadAnalysisPausedConfig_DefaultFalse(t *testing.T) {
	testutil.SetupTestDB(t)

	paused, pausedAt, err := LoadAnalysisPausedConfig()
	require.NoError(t, err)
	require.False(t, paused)
	require.Nil(t, pausedAt)
}

// TestSaveAndLoadAnalysisPausedConfig covers the store round trip: engaging
// (true) stamps paused_at, releasing (false) clears it again.
func TestSaveAndLoadAnalysisPausedConfig(t *testing.T) {
	testutil.SetupTestDB(t)

	// Engage: paused=true, paused_at stamped to now (UTC, RFC3339).
	require.NoError(t, SaveAnalysisPausedConfig(true))
	paused, pausedAt, err := LoadAnalysisPausedConfig()
	require.NoError(t, err)
	require.True(t, paused)
	require.NotNil(t, pausedAt)

	// Release: paused=false, paused_at cleared.
	require.NoError(t, SaveAnalysisPausedConfig(false))
	paused, pausedAt, err = LoadAnalysisPausedConfig()
	require.NoError(t, err)
	require.False(t, paused)
	require.Nil(t, pausedAt)
}

func TestIsValidHTTPURL(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"https://docs.rsshub.app", true},
		{"http://example.com", true},
		{"https://docs.rsshub.app/routes", true},
		{"ftp://example.com", false},
		{"example.com", false},
		{"", false},
		{"not a url", false},
		{"javascript:alert(1)", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isValidHTTPURL(tt.input); got != tt.valid {
				t.Errorf("isValidHTTPURL(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}
