package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"

	"syntopica-backend/internal/models"
)

type RSSParser struct {
	client *http.Client
}

func NewRSSParser() *RSSParser {
	return &RSSParser{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

type ParsedFeed struct {
	Title       string
	Description string
	Link        string
	Image       string
	Language    string
	Entries     []ParsedEntry
}

type ParsedEntry struct {
	Title       string
	Link        string
	Description string
	Content     string
	PubDate     *time.Time
	Author      string
	Tags        []string
	ImageURL    string
}

func (p *RSSParser) ParseFeedURL(feedURL string) (*ParsedFeed, error) {
	resp, err := p.client.Get(feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read feed body: %w", err)
	}

	cleaned := sanitizeUTF8(body)

	fp := gofeed.NewParser()
	feed, err := fp.Parse(bytes.NewReader(cleaned))
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	return p.convertGofeedToParsed(feed), nil
}

func sanitizeUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	valid := make([]byte, 0, len(data))
	for i := 0; i < len(data); {
		_, size := utf8.DecodeRune(data[i:])
		switch {
		case size == 1 && data[i] < 0x80:
			valid = append(valid, data[i])
			i++
		case size == 1:
			valid = append(valid, replacement...)
			i++
		default:
			valid = append(valid, data[i:i+size]...)
			i += size
		}
	}
	return valid
}

var replacement = []byte("\uFFFD")

func (p *RSSParser) convertGofeedToParsed(feed *gofeed.Feed) *ParsedFeed {
	parsed := &ParsedFeed{
		Title:       feed.Title,
		Description: feed.Description,
		Link:        feed.Link,
		Image:       extractFeedImage(feed),
		Language:    feed.Language,
		Entries:     make([]ParsedEntry, 0, len(feed.Items)),
	}

	for _, item := range feed.Items {
		entry := ParsedEntry{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Content:     extractContent(item),
			PubDate:     parseDate(item),
			Author:      extractAuthor(item),
			Tags:        extractFeedTags(item),
			ImageURL:    extractItemImage(item),
		}

		if entry.Title == "" {
			entry.Title = "No title"
		}

		parsed.Entries = append(parsed.Entries, entry)
	}

	return parsed
}

func extractFeedImage(feed *gofeed.Feed) string {
	if feed.Image != nil {
		return feed.Image.URL
	}
	return ""
}

func extractContent(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

func parseDate(item *gofeed.Item) *time.Time {
	if item.PublishedParsed != nil {
		cstTime := item.PublishedParsed.In(models.ShanghaiTZ)
		return &cstTime
	}
	if item.UpdatedParsed != nil {
		cstTime := item.UpdatedParsed.In(models.ShanghaiTZ)
		return &cstTime
	}
	return nil
}

func extractAuthor(item *gofeed.Item) string {
	if item.Author != nil {
		return item.Author.Name
	}
	return ""
}

func extractFeedTags(item *gofeed.Item) []string {
	tags := append([]string{}, item.Categories...)
	return tags
}

// extractItemImage resolves the best available preview image for a feed item.
//
// Priority (covers the common image carriers that gofeed itself misses):
//  1. item.Image — gofeed already aggregates enclosure, media:content,
//     itunes:image and the first <img> in content/description here.
//  2. <media:thumbnail> — a known gofeed gap; many news/podcast feeds expose
//     the thumbnail only through this Media RSS element.
//  3. <enclosure type="image/*"> — final fallback for feeds that rely solely
//     on enclosures but whose type gofeed failed to classify into item.Image.
func extractItemImage(item *gofeed.Item) string {
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}
	if url := mediaThumbnailURL(item); url != "" {
		return url
	}
	for _, enc := range item.Enclosures {
		if strings.HasPrefix(enc.Type, "image/") && enc.URL != "" {
			return enc.URL
		}
	}
	return ""
}

// mediaThumbnailURL pulls the first <media:thumbnail url="..."> from the
// Media RSS extension. Gofeed stores Media RSS under Extensions["media"] and
// never surfaces thumbnails on item.Image, so this must be read explicitly.
func mediaThumbnailURL(item *gofeed.Item) string {
	media := item.Extensions["media"]
	thumbs := media["thumbnail"]
	for _, t := range thumbs {
		if u := t.Attrs["url"]; u != "" {
			return u
		}
	}
	return ""
}

// FetchFaviconURL derives a site's favicon URL from its homepage URL.
//
// siteURL should be the content site's homepage (typically the RSS channel
// <link>), NOT the RSS feed URL — using the feed URL would yield the
// aggregator's favicon (feedburner, rsshub, etc.) rather than the content
// site's. Returns "" when the URL can't be parsed so callers keep the feed in
// the fallback state rather than showing a broken image.
func (p *RSSParser) FetchFaviconURL(siteURL string) string {
	parsedURL, err := url.Parse(siteURL)
	if err != nil || parsedURL.Host == "" || parsedURL.Scheme == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s/favicon.ico", parsedURL.Scheme, parsedURL.Host)
}

func (p *RSSParser) FetchFeedMetadata(feedURL string) (title, description string, err error) {
	parsed, err := p.ParseFeedURL(feedURL)
	if err != nil {
		return "", "", err
	}

	return parsed.Title, parsed.Description, nil
}
