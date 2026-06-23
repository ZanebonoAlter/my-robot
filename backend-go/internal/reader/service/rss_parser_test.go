package service

import (
	"testing"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

// TestExtractItemImage_Priorities locks in the resolution order used by
// extractItemImage: item.Image wins, then media:thumbnail, then enclosure.
// Regression guard for the detective-wall image_url coverage.
func TestExtractItemImage_Priorities(t *testing.T) {
	tests := []struct {
		name string
		item *gofeed.Item
		want string
	}{
		{
			name: "item.Image takes priority",
			item: &gofeed.Item{
				Image:      &gofeed.Image{URL: "https://img.example/cover.png"},
				Extensions: ext.Extensions{"media": {"thumbnail": {{Name: "thumbnail", Attrs: map[string]string{"url": "https://img.example/thumb.png"}}}}},
				Enclosures: []*gofeed.Enclosure{{URL: "https://img.example/enc.png", Type: "image/png"}},
			},
			want: "https://img.example/cover.png",
		},
		{
			name: "media:thumbnail fallback when item.Image absent",
			item: &gofeed.Item{
				Extensions: ext.Extensions{"media": {"thumbnail": {
					{Name: "thumbnail", Attrs: map[string]string{"url": "https://img.example/thumb.png"}},
				}}},
			},
			want: "https://img.example/thumb.png",
		},
		{
			name: "enclosure fallback when neither item.Image nor media:thumbnail",
			item: &gofeed.Item{
				Enclosures: []*gofeed.Enclosure{
					{URL: "https://img.example/doc.pdf", Type: "application/pdf"},
					{URL: "https://img.example/enc.png", Type: "image/png"},
				},
			},
			want: "https://img.example/enc.png",
		},
		{
			name: "empty when no image source available",
			item: &gofeed.Item{},
			want: "",
		},
		{
			name: "media:thumbnail with empty url skipped, next source used",
			item: &gofeed.Item{
				Extensions: ext.Extensions{"media": {"thumbnail": {
					{Name: "thumbnail", Attrs: map[string]string{"url": ""}},
				}}},
				Enclosures: []*gofeed.Enclosure{{URL: "https://img.example/enc.png", Type: "image/png"}},
			},
			want: "https://img.example/enc.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractItemImage(tt.item); got != tt.want {
				t.Errorf("extractItemImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExtractItemImage_ParsesMediaThumbnailFromRealFeed feeds a real Media RSS
// snippet through the gofeed parser to confirm extractItemImage picks up
// <media:thumbnail>, which was the primary gap before the enhancement.
func TestExtractItemImage_ParsesMediaThumbnailFromRealFeed(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Has media thumbnail</title>
      <link>https://example.com/a</link>
      <media:thumbnail url="https://cdn.example.com/media-thumb.jpg" width="320" height="240"/>
    </item>
  </channel>
</rss>`

	fp := gofeed.NewParser()
	feed, err := fp.ParseString(rss)
	if err != nil {
		t.Fatalf("gofeed parse failed: %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	// item.Image should be nil here because gofeed does not map media:thumbnail
	// onto item.Image — this is exactly the gap extractItemImage now covers.
	if got := extractItemImage(feed.Items[0]); got != "https://cdn.example.com/media-thumb.jpg" {
		t.Errorf("extractItemImage() = %q, want media-thumb.jpg", got)
	}
}

// TestFetchFaviconURL locks in the site-URL-based favicon derivation.
// Regression guard: the old implementation parsed the RSS feed URL (yielding
// aggregator favicons) and relied on Google's s2 service (blocked in CN).
func TestFetchFaviconURL(t *testing.T) {
	p := NewRSSParser()

	tests := []struct {
		name    string
		siteURL string
		want    string
	}{
		{
			name:    "derives favicon from site homepage",
			siteURL: "https://example.com/articles/latest",
			want:    "https://example.com/favicon.ico",
		},
		{
			name:    "preserves scheme and host only",
			siteURL: "http://news.example.org/section/sub",
			want:    "http://news.example.org/favicon.ico",
		},
		{
			name:    "empty URL returns empty (keep fallback state)",
			siteURL: "",
			want:    "",
		},
		{
			name:    "unparseable URL returns empty",
			siteURL: "://not-a-url",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.FetchFaviconURL(tt.siteURL); got != tt.want {
				t.Errorf("FetchFaviconURL(%q) = %q, want %q", tt.siteURL, got, tt.want)
			}
		})
	}
}
