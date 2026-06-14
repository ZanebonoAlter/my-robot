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
			wantFirecrawlStatus:   "complete",
			wantSummaryStatus:     "pending",
		},
		{
			name:                  "neither enabled: both default",
			firecrawlEnabled:      false,
			articleSummaryEnabled: false,
			wantFirecrawlStatus:   "complete",
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
