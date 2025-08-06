package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"strandnerd-crawler/internal/models"
)

// CrawlRequest represents a crawl request from the queue
type CrawlRequest struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	FeedID    *string   `json:"feed_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Priority  int       `json:"priority"`
}

// CMSClient handles communication with the CMS API
type CMSClient struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

// NewCMSClient creates a new CMS API client
func NewCMSClient(baseURL, accessToken string) *CMSClient {
	return &CMSClient{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetInspirationFeeds fetches all inspiration feeds from the CMS
func (c *CMSClient) GetInspirationFeeds() ([]models.InspirationFeed, error) {
	url := fmt.Sprintf("%s/api/v1/crawler/inspiration_feeds", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var feeds []models.InspirationFeed
	if err := json.NewDecoder(resp.Body).Decode(&feeds); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return feeds, nil
}

// GetInspirationFeedByID fetches a specific inspiration feed by ID
func (c *CMSClient) GetInspirationFeedByID(feedID string) (*models.InspirationFeed, error) {
	url := fmt.Sprintf("%s/api/v1/crawler/inspiration_feeds/%s", c.baseURL, feedID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("feed not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var feed models.InspirationFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &feed, nil
}

// CreateInspirationFeedPost creates a new inspiration feed post in the CMS
func (c *CMSClient) CreateInspirationFeedPost(post *models.CreateInspirationFeedPostRequest) (*models.InspirationFeedPost, error) {
	url := fmt.Sprintf("%s/api/v1/crawler/inspiration_feed_posts", c.baseURL)

	jsonData, err := json.Marshal(post)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var createdPost models.InspirationFeedPost
	if err := json.NewDecoder(resp.Body).Decode(&createdPost); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &createdPost, nil
}

// GetInspirationPosts fetches existing inspiration posts to check for duplicates
func (c *CMSClient) GetInspirationPosts(feedID string, limit int) ([]models.InspirationFeedPost, error) {
	url := fmt.Sprintf("%s/api/v1/crawler/inspiration_feed_posts?feed_id=%s&limit=%d", c.baseURL, feedID, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var posts []models.InspirationFeedPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return posts, nil
}

// UpdateFeedLastCrawledAt updates the last crawled timestamp for a feed
func (c *CMSClient) UpdateFeedLastCrawledAt(feedID string) error {
	url := fmt.Sprintf("%s/api/v1/crawler/inspiration_feeds/%s/last-crawled", c.baseURL, feedID)

	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PollCrawlRequest polls for crawl requests from the CMS queue
func (c *CMSClient) PollCrawlRequest() (*CrawlRequest, error) {
	url := fmt.Sprintf("%s/api/v1/crawler/requests/poll", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// No content means no requests available
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var request CrawlRequest
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &request, nil
}

// AcknowledgeRequest acknowledges completion of a crawl request
func (c *CMSClient) AcknowledgeRequest(requestID string) error {
	url := fmt.Sprintf("%s/api/v1/crawler/requests/%s", c.baseURL, requestID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
