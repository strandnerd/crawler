package crawler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"strandnerd-crawler/internal/client"
	"strandnerd-crawler/internal/config"
	"strandnerd-crawler/internal/llm"
	"strandnerd-crawler/internal/models"
	"strandnerd-crawler/internal/parser"
)

// Service handles the crawling logic
type Service struct {
	cmsClient             *client.CMSClient
	rssParser             *parser.RSSParser
	cache                 *FeedCache
	llmClient             *llm.Client
	enableContentAnalysis bool
	enableHTMLCleanup     bool
}

func getAndDisplayPublicIP(client *http.Client) {
	// Configure proxy if provided (same logic as content extractor)

	// Make request to icanhazip.com
	resp, err := client.Get("https://ipv4.icanhazip.com")
	if err != nil {
		log.Printf("Warning: Failed to get public IP address: %v", err)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Warning: Failed to read IP response: %v", err)
		return
	}

	// Clean up the IP address (remove newlines/whitespace)
	ipAddress := strings.TrimSpace(string(body))

	if ipAddress == "" {
		log.Printf("Warning: Received empty IP address response")
		return
	}

	// Print IP address and clickable link
	log.Printf("Crawler Public IP: %s", ipAddress)
	log.Printf("IP Details: https://whatismyipaddress.com/ip/%s", ipAddress)
}

// NewService creates a new crawler service
func NewService(cmsClient *client.CMSClient, cfg *config.Config) *Service {
	log.Printf("🔧 Service Config - EnableContentAnalysis: %v, OpenAIAPIKey: %s",
		cfg.EnableContentAnalysis,
		func() string {
			if cfg.OpenAIAPIKey != "" {
				return "***provided***"
			}
			return "empty"
		}())

	var llmClient *llm.Client
	if cfg.EnableContentAnalysis && cfg.OpenAIAPIKey != "" {
		llmClient = llm.NewClient(cfg.OpenAIAPIKey)
		log.Printf("✅ LLM client created successfully with rate limiting and 5-minute timeout")
	} else {
		log.Printf("❌ LLM client not created - enabled: %v, key_provided: %v",
			cfg.EnableContentAnalysis, cfg.OpenAIAPIKey != "")
	}

	cralwerClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Panic if cfg.ProxyAuth or cfg.ProxyHost are not set. Crawler MUST have proxy set
	if cfg.ProxyAuth == "" || cfg.ProxyHost == "" {
		log.Panicf("ProxyAuth and ProxyHost must be set in config for the crawler to run")
	}

	// Configure proxy for content extraction (external crawling)
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s@%s", cfg.ProxyAuth, cfg.ProxyHost))
	if err != nil {
		panic(fmt.Errorf("failed to parse proxy URL: %w", err))
	}
	cralwerClient.Transport = &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	getAndDisplayPublicIP(cralwerClient)

	return &Service{
		cmsClient:             cmsClient,
		rssParser:             parser.NewRSSParser(cralwerClient, cfg),
		cache:                 NewFeedCache(5 * time.Minute), // Cache for 5 minutes
		llmClient:             llmClient,
		enableContentAnalysis: cfg.EnableContentAnalysis,
		enableHTMLCleanup:     false, // Removed config field, set to false
	}
}

// CrawlAllDueFeeds crawls all feeds that are due for crawling
func (s *Service) CrawlAllDueFeeds() ([]models.CrawlResult, error) {
	// Get all feeds from CMS (this will be cached)
	feeds, err := s.cache.GetFeeds(s.cmsClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get feeds: %w", err)
	}

	// Filter feeds that are due for crawling
	var dueFeeds []models.InspirationFeed
	for _, feed := range feeds {
		if feed.IsDue() {
			dueFeeds = append(dueFeeds, feed)
		}
	}

	if len(dueFeeds) == 0 {
		return []models.CrawlResult{}, nil
	}

	log.Printf("Found %d feeds due for crawling", len(dueFeeds))

	// Crawl feeds with limited concurrency
	results := make([]models.CrawlResult, len(dueFeeds))
	semaphore := make(chan struct{}, 3) // Max 3 concurrent crawls
	var wg sync.WaitGroup

	for i, feed := range dueFeeds {
		wg.Add(1)
		go func(index int, f models.InspirationFeed) {
			defer wg.Done()

			semaphore <- struct{}{} // Acquire
			result := s.crawlSingleFeed(&f)
			<-semaphore // Release

			results[index] = *result
		}(i, feed)
	}

	wg.Wait()
	return results, nil
}

// CrawlFeed crawls a specific feed by ID
func (s *Service) CrawlFeed(feedID string) (*models.CrawlResult, error) {
	// Get the specific feed from CMS
	feed, err := s.cmsClient.GetInspirationFeedByID(feedID)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed %s: %w", feedID, err)
	}

	return s.crawlSingleFeed(feed), nil
}

// crawlSingleFeed crawls a single feed and returns the result
func (s *Service) crawlSingleFeed(feed *models.InspirationFeed) *models.CrawlResult {
	result := &models.CrawlResult{
		FeedID:  feed.ID,
		Success: false,
	}

	log.Printf("Crawling feed: %s (%s)", feed.Name, feed.URL)

	// Parse the RSS feed
	rssFeed, err := s.rssParser.ParseFeed(feed.URL)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse feed: %w", err)
		return result
	}

	result.PostsFound = len(rssFeed.Items)
	log.Printf("Found %d items in feed %s", result.PostsFound, feed.Name)

	if result.PostsFound == 0 {
		result.Success = true
		return result
	}

	// Convert RSS items to inspiration posts
	posts := parser.ConvertToInspirationPosts(feed.ID, rssFeed.Items, s.rssParser.GetContentExtractor())

	// Get existing posts to check for duplicates
	existingPosts, err := s.cmsClient.GetInspirationPosts(feed.ID, 100)
	if err != nil {
		log.Printf("Warning: failed to get existing posts for feed %s: %v", feed.ID, err)
		existingPosts = []models.InspirationFeedPost{} // Continue with empty list
	}

	// Create a map of existing GUIDs for faster lookup
	existingGUIDs := make(map[string]bool)
	for _, post := range existingPosts {
		if post.GUID != nil {
			existingGUIDs[*post.GUID] = true
		}
	}

	// Create new posts, skipping duplicates
	for _, post := range posts {
		// Skip if duplicate
		if post.GUID != nil && existingGUIDs[*post.GUID] {
			result.PostsSkipped++
			continue
		}

		// Analyze content with GPT if enabled
		if s.enableContentAnalysis && s.llmClient != nil {
			log.Printf("🔍 Analyzing content for post: %s", post.Title)

			analysisReq := &models.ContentAnalysisRequest{
				Title:       post.Title,
				Description: post.Description,
				Content:     post.Content,
				FullContent: post.FullContent,
				URL:         post.URL,
			}

			analysis, err := s.llmClient.AnalyzeContent(analysisReq)
			if err != nil {
				log.Printf("Warning: content analysis failed for post '%s': %v", post.Title, err)
				// Set defaults for failed analysis - assume referenced reporting to be conservative
				falseVal := false
				post.IsPrimaryReporting = &falseVal
				post.OriginalSourceName = nil
			} else {
				// Apply analysis results
				post.IsPrimaryReporting = &analysis.IsPrimaryReporting
				post.OriginalSourceName = analysis.OriginalSourceName

				log.Printf("✅ Content analysis for '%s': primary=%v, source=%v, confidence=%.2f, reasoning=%s",
					post.Title, analysis.IsPrimaryReporting,
					func() string {
						if analysis.OriginalSourceName != nil {
							return *analysis.OriginalSourceName
						}
						return "none"
					}(), analysis.Confidence, analysis.Reasoning)

				// Debug: Log what we're about to send to CMS
				log.Printf("Post after analysis - Title: %s, IsPrimaryReporting: %v, OriginalSourceName: %v",
					post.Title,
					func() string {
						if post.IsPrimaryReporting != nil {
							return fmt.Sprintf("%v", *post.IsPrimaryReporting)
						}
						return "nil"
					}(),
					func() string {
						if post.OriginalSourceName != nil {
							return *post.OriginalSourceName
						}
						return "nil"
					}())
			}
		} else {
			log.Printf("❌ Content analysis disabled - enabled=%v, client_available=%v for post: %s",
				s.enableContentAnalysis, s.llmClient != nil, post.Title)

			// When LLM analysis is disabled, assume most articles are primary reporting unless proven otherwise
			// This is more balanced than always assuming referenced reporting
			trueVal := true
			post.IsPrimaryReporting = &trueVal
			post.OriginalSourceName = nil
			log.Printf("🔧 LLM analysis disabled - defaulting to primary reporting (can be overridden if obvious references found)")
		}

		// Create the post in CMS
		_, err := s.cmsClient.CreateInspirationFeedPost(post)
		if err != nil {
			log.Printf("Failed to create post '%s' for feed %s: %v", post.Title, feed.Name, err)
			result.PostsSkipped++
			continue
		}

		result.PostsAdded++
		log.Printf("Added post: %s", post.Title)
	}

	// Update the feed's last crawled timestamp
	if err := s.cmsClient.UpdateFeedLastCrawledAt(feed.ID); err != nil {
		log.Printf("Warning: failed to update last crawled timestamp for feed %s: %v", feed.ID, err)
	}

	result.Success = true
	log.Printf("Completed crawling feed %s: %d found, %d added, %d skipped",
		feed.Name, result.PostsFound, result.PostsAdded, result.PostsSkipped)

	return result
}

// FeedCache caches feeds to avoid hitting the CMS API too frequently
type FeedCache struct {
	feeds      []models.InspirationFeed
	lastUpdate time.Time
	ttl        time.Duration
	mutex      sync.RWMutex
}

// NewFeedCache creates a new feed cache
func NewFeedCache(ttl time.Duration) *FeedCache {
	return &FeedCache{
		ttl: ttl,
	}
}

// GetFeeds returns cached feeds or fetches them if cache is expired
func (c *FeedCache) GetFeeds(cmsClient *client.CMSClient) ([]models.InspirationFeed, error) {
	c.mutex.RLock()
	if time.Since(c.lastUpdate) < c.ttl && len(c.feeds) > 0 {
		feeds := c.feeds
		c.mutex.RUnlock()
		return feeds, nil
	}
	c.mutex.RUnlock()

	// Cache expired or empty, fetch new data
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Double-check in case another goroutine updated while we were waiting
	if time.Since(c.lastUpdate) < c.ttl && len(c.feeds) > 0 {
		return c.feeds, nil
	}

	log.Println("Refreshing feeds cache...")
	feeds, err := cmsClient.GetInspirationFeeds()
	if err != nil {
		return nil, err
	}

	c.feeds = feeds
	c.lastUpdate = time.Now()
	log.Printf("Cache refreshed with %d feeds", len(feeds))

	return feeds, nil
}

// ProcessQueueRequests polls the CMS queue and processes any pending requests
func (s *Service) ProcessQueueRequests() error {
	log.Println("Checking for queue requests...")

	request, err := s.cmsClient.PollCrawlRequest()
	if err != nil {
		return fmt.Errorf("failed to poll for requests: %w", err)
	}

	// No requests available
	if request == nil {
		return nil
	}

	log.Printf("Processing queue request: %s (type: %s)", request.ID, request.Type)

	var results []models.CrawlResult

	switch request.Type {
	case "single":
		if request.FeedID == nil {
			log.Printf("Invalid single crawl request: missing feed ID")
			break
		}
		result, err := s.CrawlFeed(*request.FeedID)
		if err != nil {
			log.Printf("Failed to crawl feed %s: %v", *request.FeedID, err)
		} else {
			results = append(results, *result)
		}

	case "all":
		allResults, err := s.CrawlAllDueFeeds()
		if err != nil {
			log.Printf("Failed to crawl all due feeds: %v", err)
		} else {
			results = allResults
		}

	default:
		log.Printf("Unknown request type: %s", request.Type)
	}

	// Acknowledge the request (whether successful or not)
	if err := s.cmsClient.AcknowledgeRequest(request.ID); err != nil {
		log.Printf("Warning: failed to acknowledge request %s: %v", request.ID, err)
	}

	// Log results
	if len(results) > 0 {
		totalAdded := 0
		successCount := 0
		for _, result := range results {
			if result.Success {
				successCount++
				totalAdded += result.PostsAdded
			}
		}

		log.Printf("Queue request completed: %d feeds processed, %d successful, %d posts added",
			len(results), successCount, totalAdded)
	}

	return nil
}
