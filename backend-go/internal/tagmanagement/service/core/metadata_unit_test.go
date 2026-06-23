package core

import (
	"fmt"
	"testing"
)

func TestLimitArticleTagsKeepsTopFiveInOrder(t *testing.T) {
	tags := make([]TopicTag, 0, 10)
	for i := 0; i < 10; i++ {
		tags = append(tags, TopicTag{
			Label:    fmt.Sprintf("Tag %d", i),
			Slug:     fmt.Sprintf("tag-%d", i),
			Category: "keyword",
			Score:    float64(10 - i),
		})
	}

	limited := limitArticleTags(tags)

	if len(limited) != 5 {
		t.Fatalf("limited tag count = %d, want 5", len(limited))
	}
	for i, tag := range limited {
		want := fmt.Sprintf("Tag %d", i)
		if tag.Label != want {
			t.Fatalf("tag at index %d = %q, want %q", i, tag.Label, want)
		}
	}
}
