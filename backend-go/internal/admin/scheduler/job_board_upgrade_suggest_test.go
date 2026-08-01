package scheduler

import "testing"

// TestParseBoardUpgradeHHMM covers the ai_settings time-config parser used by
// NextBoardUpgradeSuggestTime (§5.4 job wiring). Valid HH:MM within 00:00–23:59
// parses; out-of-range and malformed values error so the job falls back to the
// 06:30 default instead of panicking on a bad config.
func TestParseBoardUpgradeHHMM(t *testing.T) {
	cases := []struct {
		in      string
		h, m    int
		wantErr bool
	}{
		{"06:30", 6, 30, false},
		{"00:00", 0, 0, false},
		{"23:59", 23, 59, false},
		{"6:30", 6, 30, false}, // Sscanf %d tolerates no leading zero
		{"24:00", 0, 0, true},  // hour out of range
		{"06:60", 0, 0, true},  // minute out of range
		{"-1:30", 0, 0, true},  // negative hour
		{"abc", 0, 0, true},
		{"", 0, 0, true},
	}
	for _, c := range cases {
		h, m, err := parseBoardUpgradeHHMM(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseBoardUpgradeHHMM(%q) want error, got (%d,%d,nil)", c.in, h, m)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseBoardUpgradeHHMM(%q) unexpected error: %v", c.in, err)
			continue
		}
		if h != c.h || m != c.m {
			t.Errorf("parseBoardUpgradeHHMM(%q) = (%d,%d), want (%d,%d)", c.in, h, m, c.h, c.m)
		}
	}
}
