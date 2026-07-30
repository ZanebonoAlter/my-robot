package aisettings

import (
	"testing"
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
