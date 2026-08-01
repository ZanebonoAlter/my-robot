package service

import (
	"net/http"
	"net/http/httptest"
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

// TestProbeFaviconCandidates covers the two-tier favicon probe: homepage HTML
// <link rel="icon"> parsing (relative hrefs resolved to absolute) with the
// {scheme}://{host}/favicon.ico guess as the always-present final candidate.
func TestProbeFaviconCandidates(t *testing.T) {
	p := NewRSSParser()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/relative-link":
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/static/icon.png"></head></html>`))
		case "/shortcut-link":
			_, _ = w.Write([]byte(`<html><head><link rel="shortcut icon" href="favicon.ico"></head></html>`))
		case "/apple-link":
			_, _ = w.Write([]byte(`<html><head><link rel="apple-touch-icon" href="/apple.png"></head></html>`))
		case "/absolute-link":
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="https://cdn.example.com/icon.png"></head></html>`))
		case "/no-link":
			_, _ = w.Write([]byte(`<html><head><title>plain</title></head></html>`))
		case "/rss-xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<rss><channel><link>https://example.com</link></channel></rss>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tests := []struct {
		name    string
		siteURL string
		want    []string
	}{
		{
			name:    "relative link href resolved to absolute, guess last",
			siteURL: srv.URL + "/relative-link",
			want:    []string{srv.URL + "/static/icon.png", srv.URL + "/favicon.ico"},
		},
		{
			name:    "shortcut icon resolving to the guess is deduped",
			siteURL: srv.URL + "/shortcut-link",
			want:    []string{srv.URL + "/favicon.ico"},
		},
		{
			name:    "apple-touch-icon accepted",
			siteURL: srv.URL + "/apple-link",
			want:    []string{srv.URL + "/apple.png", srv.URL + "/favicon.ico"},
		},
		{
			name:    "absolute link href kept as-is",
			siteURL: srv.URL + "/absolute-link",
			want:    []string{"https://cdn.example.com/icon.png", srv.URL + "/favicon.ico"},
		},
		{
			name:    "no icon link -> guess only",
			siteURL: srv.URL + "/no-link",
			want:    []string{srv.URL + "/favicon.ico"},
		},
		{
			name:    "homepage fetch fails -> guess only",
			siteURL: srv.URL + "/missing-page",
			want:    []string{srv.URL + "/favicon.ico"},
		},
		{
			name:    "non-HTML homepage (e.g. RSS at root) -> guess only",
			siteURL: srv.URL + "/rss-xml",
			want:    []string{srv.URL + "/favicon.ico"},
		},
		{
			name:    "empty site URL -> nil (keep fallback state)",
			siteURL: "",
			want:    nil,
		},
		{
			name:    "unparseable site URL -> nil",
			siteURL: "://not-a-url",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.ProbeFaviconCandidates(tt.siteURL)
			if len(got) != len(tt.want) {
				t.Fatalf("ProbeFaviconCandidates(%q) = %v, want %v", tt.siteURL, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("candidate[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
