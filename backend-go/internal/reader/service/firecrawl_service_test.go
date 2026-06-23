package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestScrapeResponse_ParsesImageMetadata locks in that the scrape response
// struct captures ogImage/twitterImage from Firecrawl's metadata payload.
// Regression guard for the detective-wall image_url backfill.
func TestScrapeResponse_ParsesImageMetadata(t *testing.T) {
	payload := `{
		"success": true,
		"data": {
			"markdown": "# hello",
			"html": "<h1>hello</h1>",
			"metadata": {
				"title": "Example",
				"description": "desc",
				"language": "en",
				"sourceURL": "https://example.com",
				"ogImage": "https://example.com/og.png",
				"twitterImage": "https://example.com/tw.png"
			}
		}
	}`

	var resp ScrapeResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Data.Metadata.OgImage != "https://example.com/og.png" {
		t.Errorf("ogImage = %q, want og.png", resp.Data.Metadata.OgImage)
	}
	if resp.Data.Metadata.TwitterImage != "https://example.com/tw.png" {
		t.Errorf("twitterImage = %q, want tw.png", resp.Data.Metadata.TwitterImage)
	}
}

// TestScrapePage_BackfillsMetadataImage drives the real ScrapePage path
// against a stub Firecrawl endpoint and asserts metadata returned by Firecrawl
// survives the round-trip (mirrors how job_firecrawl.go consumes the result).
func TestScrapePage_BackfillsMetadataImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		formats, _ := body["formats"].([]interface{})
		gotMarkdown := false
		gotHTML := false
		for _, f := range formats {
			switch f {
			case "markdown":
				gotMarkdown = true
			case "html":
				gotHTML = true
			case "metadata":
				t.Errorf("scrape request asked for invalid metadata format: %v", formats)
			}
		}
		if !gotMarkdown || !gotHTML {
			t.Errorf("scrape request formats = %v, want markdown and html", formats)
		}
		_, _ = w.Write([]byte(`{
				"success": true,
			"data": {
				"markdown": "# body",
				"metadata": {"title": "T", "ogImage": "https://cdn.test/cover.jpg"}
			}
		}`))
	}))
	defer srv.Close()

	svc := NewFirecrawlService(&FirecrawlConfig{
		APIUrl: srv.URL, APIKey: "k", Enabled: true, Mode: "scrape", Timeout: 5,
	})

	resp, err := svc.ScrapePage(context.Background(), "https://news.example/x")
	if err != nil {
		t.Fatalf("ScrapePage failed: %v", err)
	}
	if resp.Data.Metadata.OgImage != "https://cdn.test/cover.jpg" {
		t.Errorf("ogImage = %q, want cover.jpg", resp.Data.Metadata.OgImage)
	}
}
