package handler

import (
	"testing"
)

func TestMatchTier(t *testing.T) {
	cases := []struct {
		reason     string
		downgraded bool
		want       int
	}{
		// direct_hit → tier 0 (best)
		{"direct_hit", false, 0},
		{"direct_hit", true, 0}, // downgraded has no effect on direct_hit

		// hit_rate → tier 1
		{"hit_rate", false, 1},
		{"hit_rate", true, 1},

		// max_sim non-downgraded → tier 2
		{"max_sim", false, 2},
		// max_sim downgraded → tier 3 (fall to weakest)
		{"max_sim", true, 3},

		// weighted → tier 3 (weakest)
		{"weighted", false, 3},
		{"weighted", true, 3},

		// unknown reason → tier 3 (safe fallback)
		{"unknown", false, 3},
		{"", false, 3},
	}
	for _, c := range cases {
		got := MatchTier(c.reason, c.downgraded)
		if got != c.want {
			t.Errorf("MatchTier(%q, %v) = %d, want %d", c.reason, c.downgraded, got, c.want)
		}
	}
}
