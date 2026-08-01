package textutil

import "testing"

func TestNormalizeLabelKey(t *testing.T) {
	cases := map[string]string{
		"SK海力士":        "sk海力士",
		"SK 海力士":       "sk海力士",
		"  SK   海力士  ": "sk海力士",
		"SK\t海力士":      "sk海力士",
		"DeepSeek":     "deepseek",
		"  ":           "",
		"":             "",
	}
	for input, want := range cases {
		if got := NormalizeLabelKey(input); got != want {
			t.Errorf("NormalizeLabelKey(%q) = %q, want %q", input, got, want)
		}
	}
}
