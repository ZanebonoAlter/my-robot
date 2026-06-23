package service

import (
	"testing"

	"syntopica-backend/internal/models"
)

func TestBuildArticleFromEntryTracksOnlyRunnableStates(t *testing.T) {
	service := NewFeedService()
	entry := ParsedEntry{
		Title:       "Fresh News",
		Description: "desc",
		Content:     "content",
		Link:        "https://example.com/article",
		Author:      "bot",
	}

	tests := []struct {
		name                  string
		firecrawlEnabled      bool
		articleSummaryEnabled bool
		wantFirecrawlStatus   string
		wantSummaryStatus     string
	}{
		{
			name:                  "both enabled: summary incomplete, firecrawl pending",
			firecrawlEnabled:      true,
			articleSummaryEnabled: true,
			wantFirecrawlStatus:   "pending",
			wantSummaryStatus:     "incomplete",
		},
		{
			name:                  "summary only: summary pending, no firecrawl",
			firecrawlEnabled:      false,
			articleSummaryEnabled: true,
			wantFirecrawlStatus:   "completed",
			wantSummaryStatus:     "pending",
		},
		{
			name:                  "neither enabled: both default",
			firecrawlEnabled:      false,
			articleSummaryEnabled: false,
			wantFirecrawlStatus:   "completed",
			wantSummaryStatus:     "complete",
		},
		{
			name:                  "firecrawl only: summary complete, firecrawl pending",
			firecrawlEnabled:      true,
			articleSummaryEnabled: false,
			wantFirecrawlStatus:   "pending",
			wantSummaryStatus:     "complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feed := models.Feed{FirecrawlEnabled: tt.firecrawlEnabled, ArticleSummaryEnabled: tt.articleSummaryEnabled}
			article := service.buildArticleFromEntry(feed, entry)
			if article.FirecrawlStatus != tt.wantFirecrawlStatus {
				t.Errorf("firecrawl status = %q, want %q", article.FirecrawlStatus, tt.wantFirecrawlStatus)
			}
			if article.SummaryStatus != tt.wantSummaryStatus {
				t.Errorf("summary status = %q, want %q", article.SummaryStatus, tt.wantSummaryStatus)
			}
		})
	}
}

// TestResolveFeedIcon_StateMachine locks in the icon source state machine:
// custom is frozen, auto/fallback recompute, RSS image beats favicon, article
// images are never used.
func TestResolveFeedIcon_StateMachine(t *testing.T) {
	tests := []struct {
		name          string
		currentSource string
		parsedImage   string
		siteFavicon   string
		wantIcon      string
		wantSource    string
		wantOk        bool
	}{
		{
			name:          "custom is frozen regardless of candidates",
			currentSource: "custom",
			parsedImage:   "https://img.example/rss.png",
			siteFavicon:   "https://example.com/favicon.ico",
			wantIcon:      "",
			wantSource:    "",
			wantOk:        false,
		},
		{
			name:          "fallback + RSS image -> auto",
			currentSource: "fallback",
			parsedImage:   "https://img.example/rss.png",
			siteFavicon:   "https://example.com/favicon.ico",
			wantIcon:      "https://img.example/rss.png",
			wantSource:    "auto",
			wantOk:        true,
		},
		{
			name:          "fallback + no RSS image + favicon -> auto",
			currentSource: "fallback",
			parsedImage:   "",
			siteFavicon:   "https://example.com/favicon.ico",
			wantIcon:      "https://example.com/favicon.ico",
			wantSource:    "auto",
			wantOk:        true,
		},
		{
			name:          "fallback + neither image nor favicon -> stays fallback mdi:rss",
			currentSource: "fallback",
			parsedImage:   "",
			siteFavicon:   "",
			wantIcon:      "mdi:rss",
			wantSource:    "fallback",
			wantOk:        true,
		},
		{
			name:          "auto can be refreshed to a new image",
			currentSource: "auto",
			parsedImage:   "https://img.example/new.png",
			siteFavicon:   "",
			wantIcon:      "https://img.example/new.png",
			wantSource:    "auto",
			wantOk:        true,
		},
		{
			name:          "empty source treated as fallback (legacy rows)",
			currentSource: "",
			parsedImage:   "https://img.example/x.png",
			siteFavicon:   "",
			wantIcon:      "https://img.example/x.png",
			wantSource:    "auto",
			wantOk:        true,
		},
		// Regression: article cover images must NOT be injected here. This test
		// documents that resolveFeedIcon has no article-image parameter — any
		// caller passing article images would be a bug.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon, source, ok := resolveFeedIcon(tt.currentSource, tt.parsedImage, tt.siteFavicon)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if icon != tt.wantIcon {
				t.Errorf("icon = %q, want %q", icon, tt.wantIcon)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
		})
	}
}
