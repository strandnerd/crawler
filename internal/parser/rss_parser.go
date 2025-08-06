package parser

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"strandnerd-crawler/internal/config"
	"strandnerd-crawler/internal/models"
)

// RSSParser handles parsing RSS feeds
type RSSParser struct {
	httpClient       *http.Client
	userAgent        string
	contentExtractor *ContentExtractor
}

// NewRSSParser creates a new RSS parser
func NewRSSParser(client *http.Client, config *config.Config) *RSSParser {
	return &RSSParser{
		httpClient:       client,
		contentExtractor: NewContentExtractor(client, config),
	}
}

// ParseFeed fetches and parses an RSS feed from the given URL
func (p *RSSParser) ParseFeed(feedURL string) (*models.RSSFeed, error) {
	// Create request
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	// Fetch the feed
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Detect feed type by checking the root element
	bodyStr := string(body)
	
	if strings.Contains(bodyStr, `xmlns="http://www.w3.org/2005/Atom"`) || strings.Contains(bodyStr, "<feed") {
		// Parse as Atom feed
		var atomFeed models.AtomFeed
		if err := xml.Unmarshal(body, &atomFeed); err != nil {
			return nil, fmt.Errorf("failed to parse Atom feed: %w", err)
		}
		
		// Convert Atom to RSS format for consistent processing
		return convertAtomToRSS(&atomFeed), nil
	} else {
		// Parse as RSS feed
		var rss models.RSS
		if err := xml.Unmarshal(body, &rss); err != nil {
			return nil, fmt.Errorf("failed to parse RSS: %w", err)
		}
		
		return &rss.Channel, nil
	}
}

// GetContentExtractor returns the content extractor instance
func (p *RSSParser) GetContentExtractor() *ContentExtractor {
	return p.contentExtractor
}

// ConvertToInspirationPosts converts RSS items to inspiration feed posts
func ConvertToInspirationPosts(feedID string, rssItems []models.RSSItem, contentExtractor *ContentExtractor) []*models.CreateInspirationFeedPostRequest {
	var posts []*models.CreateInspirationFeedPostRequest

	for _, item := range rssItems {
		post := &models.CreateInspirationFeedPostRequest{
			InspirationFeedID: feedID,
			Title:             cleanString(item.Title),
			URL:               cleanString(item.Link),
		}

		// Handle description
		if item.Description != "" {
			desc := cleanString(item.Description)
			post.Description = &desc
		}

		// Handle RSS content as fallback
		if item.Content != "" {
			content := cleanString(item.Content)
			post.Content = &content
		}

		// Handle author
		if item.Author != "" {
			author := cleanString(item.Author)
			post.Author = &author
		}

		// Handle published date
		if item.PubDate != "" {
			// Try to parse the date and convert to RFC3339
			if parsedTime, err := parseRSSDate(item.PubDate); err == nil {
				pubDate := parsedTime.Format(time.RFC3339)
				post.PublishedAt = &pubDate
			}
		}

		// Handle GUID
		if item.GUID != "" {
			guid := cleanString(item.GUID)
			post.GUID = &guid
		} else {
			// Use URL as GUID if no GUID provided
			guid := cleanString(item.Link)
			post.GUID = &guid
		}

		// Extract content and image from the actual webpage
		if post.URL != "" {
			if extracted, err := contentExtractor.ExtractContentFromURL(post.URL); err == nil {
				// Use extracted Open Graph image as priority
				if extracted.ImageURL != "" {
					post.ImageURL = &extracted.ImageURL
				}

				// Use extracted HTML content as full content
				if extracted.FullContent != "" {
					post.FullContent = &extracted.FullContent
				}
			}
		}

		// Fallback to RSS-embedded images if webpage extraction failed
		if post.ImageURL == nil {
			if item.MediaThumbnail != nil && item.MediaThumbnail.URL != "" {
				imageURL := cleanString(item.MediaThumbnail.URL)
				post.ImageURL = &imageURL
			} else if item.MediaContent != nil && item.MediaContent.URL != "" {
				imageURL := cleanString(item.MediaContent.URL)
				post.ImageURL = &imageURL
			} else if item.Enclosure != nil && item.Enclosure.URL != "" && isImageType(item.Enclosure.Type) {
				imageURL := cleanString(item.Enclosure.URL)
				post.ImageURL = &imageURL
			}
		}

		// Only add posts with both title and URL
		if post.Title != "" && post.URL != "" {
			posts = append(posts, post)
		}
	}

	return posts
}

// cleanString removes extra whitespace and HTML tags
func cleanString(s string) string {
	// Remove leading/trailing whitespace
	s = strings.TrimSpace(s)

	// Basic HTML tag removal (simple approach)
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")

	// Remove other HTML tags (very basic, just remove < and > content)
	for strings.Contains(s, "<") && strings.Contains(s, ">") {
		start := strings.Index(s, "<")
		end := strings.Index(s[start:], ">")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}

	// Normalize whitespace
	s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	return strings.TrimSpace(s)
}

// parseRSSDate tries to parse common RSS and Atom date formats
func parseRSSDate(dateStr string) (time.Time, error) {
	// Common RSS and Atom date formats
	formats := []string{
		time.RFC3339,  // "2006-01-02T15:04:05Z07:00" (Atom standard)
		time.RFC3339Nano, // "2006-01-02T15:04:05.999999999Z07:00" (Atom with nanoseconds)
		time.RFC1123Z, // "Mon, 02 Jan 2006 15:04:05 -0700" (RSS standard)
		time.RFC1123,  // "Mon, 02 Jan 2006 15:04:05 MST"
		"2006-01-02T15:04:05-07:00", // Atom without Z
		"2006-01-02T15:04:05.000Z", // Atom with milliseconds
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// convertAtomToRSS converts an Atom feed to RSS format for consistent processing
func convertAtomToRSS(atomFeed *models.AtomFeed) *models.RSSFeed {
	rss := &models.RSSFeed{
		Title:       atomFeed.Title,
		Description: atomFeed.Subtitle,
		Items:       make([]models.RSSItem, 0, len(atomFeed.Entries)),
	}

	// Find the alternate link for the feed
	for _, link := range atomFeed.Link {
		if link.Rel == "alternate" || link.Rel == "" {
			rss.Link = link.Href
			break
		}
	}

	// Convert entries to RSS items
	for _, entry := range atomFeed.Entries {
		item := models.RSSItem{
			Title:       entry.Title,
			Description: entry.Summary,
			Content:     entry.Content.Value,
			Author:      entry.Author.Name,
			GUID:        entry.ID,
		}

		// Handle published date - prefer published over updated
		if entry.Published != "" {
			item.PubDate = entry.Published
		} else if entry.Updated != "" {
			item.PubDate = entry.Updated
		}

		// Find the alternate link for the entry
		for _, link := range entry.Link {
			if link.Rel == "alternate" || link.Rel == "" {
				item.Link = link.Href
				break
			}
		}

		rss.Items = append(rss.Items, item)
	}

	return rss
}

// isImageType checks if the given MIME type is an image
func isImageType(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(mimeType), "image/")
}
