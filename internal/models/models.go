package models

import "time"

// InspirationFeed represents an inspiration feed from the CMS
type InspirationFeed struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	URL                  string    `json:"url"`
	Description          *string   `json:"description"`
	CategoryID           *string   `json:"category_id"`
	IsActive             bool      `json:"is_active"`
	LastCrawledAt        *string   `json:"last_crawled_at"`
	CrawlIntervalMinutes int       `json:"crawl_interval_minutes"`
	CreatedAt            string    `json:"created_at"`
	UpdatedAt            string    `json:"updated_at"`
	Category             *Category `json:"category,omitempty"`
	PostCount            int       `json:"post_count,omitempty"`
}

// Category represents a simplified category for the inspiration feeds
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
	Path string `json:"path"`
}

// InspirationFeedPost represents a post from an inspiration feed
type InspirationFeedPost struct {
	ID                 string           `json:"id"`
	InspirationFeedID  string           `json:"inspiration_feed_id"`
	Title              string           `json:"title"`
	Description        *string          `json:"description"`
	Content            *string          `json:"content"`
	URL                string           `json:"url"`
	Author             *string          `json:"author"`
	PublishedAt        *string          `json:"published_at"`
	GUID               *string          `json:"guid"`
	ImageURL           *string          `json:"image_url"`
	FullContent        *string          `json:"full_content"`
	IsPrimaryReporting *bool            `json:"is_primary_reporting"`
	OriginalSourceName *string          `json:"original_source_name"`
	CreatedAt          string           `json:"created_at"`
	UpdatedAt          string           `json:"updated_at"`
	Feed               *InspirationFeed `json:"feed,omitempty"`
}

// CreateInspirationFeedPostRequest represents the request to create a new inspiration feed post
type CreateInspirationFeedPostRequest struct {
	InspirationFeedID  string  `json:"inspiration_feed_id" binding:"required"`
	Title              string  `json:"title" binding:"required"`
	Description        *string `json:"description"`
	Content            *string `json:"content"`
	URL                string  `json:"url" binding:"required"`
	Author             *string `json:"author"`
	PublishedAt        *string `json:"published_at"`
	GUID               *string `json:"guid"`
	ImageURL           *string `json:"image_url"`
	FullContent        *string `json:"full_content"`
	IsPrimaryReporting *bool   `json:"is_primary_reporting"`
	OriginalSourceName *string `json:"original_source_name"`
}

// RSS parsing types
type RSSFeed struct {
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	Link        string    `xml:"link"`
	Items       []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title          string        `xml:"title"`
	Description    string        `xml:"description"`
	Content        string        `xml:"content"`
	Link           string        `xml:"link"`
	Author         string        `xml:"author"`
	PubDate        string        `xml:"pubDate"`
	GUID           string        `xml:"guid"`
	Enclosure      *Enclosure    `xml:"enclosure"`
	MediaThumbnail *MediaContent `xml:"media:thumbnail"`
	MediaContent   *MediaContent `xml:"media:content"`
}

type Enclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type MediaContent struct {
	URL string `xml:"url,attr"`
}

// RSS Channel wrapper
type RSS struct {
	Channel RSSFeed `xml:"channel"`
}

// Atom feed types
type AtomFeed struct {
	Title   string      `xml:"title"`
	Subtitle string     `xml:"subtitle"`
	Link    []AtomLink  `xml:"link"`
	Entries []AtomEntry `xml:"entry"`
}

type AtomEntry struct {
	Title     string     `xml:"title"`
	Summary   string     `xml:"summary"`
	Content   AtomContent `xml:"content"`
	Link      []AtomLink `xml:"link"`
	Author    AtomAuthor `xml:"author"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	ID        string     `xml:"id"`
	Category  []AtomCategory `xml:"category"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type AtomAuthor struct {
	Name string `xml:"name"`
}

type AtomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type AtomCategory struct {
	Term string `xml:"term,attr"`
}

// CrawlResult represents the result of crawling a feed
type CrawlResult struct {
	FeedID       string
	Success      bool
	Error        error
	PostsFound   int
	PostsAdded   int
	PostsSkipped int
}

// IsDue checks if a feed is due for crawling based on its interval and last crawled time
func (f *InspirationFeed) IsDue() bool {
	if !f.IsActive {
		return false
	}

	if f.LastCrawledAt == nil {
		return true // Never crawled before
	}

	lastCrawled, err := time.Parse(time.RFC3339, *f.LastCrawledAt)
	if err != nil {
		return true // Invalid timestamp, assume due
	}

	interval := time.Duration(f.CrawlIntervalMinutes) * time.Minute
	return time.Since(lastCrawled) >= interval
}

// ContentAnalysisRequest represents a request for GPT content analysis
type ContentAnalysisRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Content     *string `json:"content"`
	FullContent *string `json:"full_content"`
	URL         string  `json:"url"`
}

// ContentAnalysisResponse represents the response from GPT content analysis
type ContentAnalysisResponse struct {
	IsPrimaryReporting bool    `json:"is_primary_reporting"`
	OriginalSourceName *string `json:"original_source_name"`
	Confidence         float64 `json:"confidence"`
	Reasoning          string  `json:"reasoning"`
}
