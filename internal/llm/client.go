package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"strandnerd-crawler/internal/models"
)

// Client handles communication with OpenAI GPT API
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new LLM client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// OpenAI API structures
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AnalyzeContent analyzes content to determine if it's primary reporting and extract original source
func (c *Client) AnalyzeContent(req *models.ContentAnalysisRequest) (*models.ContentAnalysisResponse, error) {
	// Prepare content for analysis
	content := c.prepareContentForAnalysis(req)

	log.Printf("🔍 LLM Analysis - Prepared content (%d chars) for: %s", len(content), req.Title)
	log.Printf("🔍 LLM Analysis - Content preview: %s", func() string {
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	}())

	if len(content) < 10 {
		log.Printf("⚠️ LLM Analysis - Insufficient content for analysis: %s", req.Title)
		return &models.ContentAnalysisResponse{
			IsPrimaryReporting: true, // Default to primary reporting when insufficient content
			OriginalSourceName: nil,
			Confidence:         0.1,
			Reasoning:          "Insufficient content for analysis - defaulting to primary reporting",
		}, nil
	}

	// Create the prompt
	prompt := c.createAnalysisPrompt(content)

	// Add rule-based fallback check before calling LLM
	if ruleBasedResult := c.ruleBasedAnalysis(content); ruleBasedResult != nil {
		log.Printf("🔍 LLM Analysis - Rule-based analysis detected referenced reporting, skipping LLM call")
		return ruleBasedResult, nil
	}

	// Make API request
	chatReq := ChatCompletionRequest{
		Model: "gpt-4o-mini", // Using cheap model as requested
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are an expert journalist and content analyst. Analyze news articles to determine if they are primary reporting or reference other sources. Always respond with valid JSON only.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.3, // Lower temperature for more consistent results
		MaxTokens:   200,
	}

	response, err := c.makeAPIRequest(&chatReq)
	if err != nil {
		log.Printf("❌ LLM Analysis - API request failed: %v, defaulting to primary reporting", err)
		return &models.ContentAnalysisResponse{
			IsPrimaryReporting: true, // Default to primary when API fails
			OriginalSourceName: nil,
			Confidence:         0.1,
			Reasoning:          fmt.Sprintf("LLM API request failed (%v), defaulting to primary reporting", err),
		}, nil
	}

	if len(response.Choices) == 0 {
		log.Printf("❌ LLM Analysis - No response choices from GPT, defaulting to primary reporting")
		return &models.ContentAnalysisResponse{
			IsPrimaryReporting: true, // Default to primary when no response
			OriginalSourceName: nil,
			Confidence:         0.1,
			Reasoning:          "No response from LLM, defaulting to primary reporting",
		}, nil
	}

	// Parse the response
	log.Printf("🔍 LLM Analysis - Raw response: %s", response.Choices[0].Message.Content)
	result, err := c.parseAnalysisResponse(response.Choices[0].Message.Content)
	if err != nil {
		log.Printf("❌ LLM Analysis - Failed to parse response: %v, defaulting to primary reporting", err)
		return &models.ContentAnalysisResponse{
			IsPrimaryReporting: true, // Default to primary when parsing fails
			OriginalSourceName: nil,
			Confidence:         0.1,
			Reasoning:          fmt.Sprintf("Failed to parse LLM response (%v), defaulting to primary reporting", err),
		}, nil
	}

	return result, nil
}

// prepareContentForAnalysis combines title, description, and content for analysis
func (c *Client) prepareContentForAnalysis(req *models.ContentAnalysisRequest) string {
	var parts []string

	if req.Title != "" {
		parts = append(parts, "Title: "+req.Title)
	}

	if req.Description != nil && *req.Description != "" {
		parts = append(parts, "Description: "+*req.Description)
	}

	// Use FullContent if available, otherwise use Content
	var contentText string
	if req.FullContent != nil && *req.FullContent != "" {
		contentText = *req.FullContent
	} else if req.Content != nil && *req.Content != "" {
		contentText = *req.Content
	}

	if contentText != "" {
		// Limit content to first 1500 characters to avoid token limits but get enough context
		if len(contentText) > 1500 {
			contentText = contentText[:1500] + "..."
		}
		parts = append(parts, "Content: "+contentText)
	}

	result := strings.Join(parts, "\n\n")
	return result
}

// createAnalysisPrompt creates the prompt for GPT analysis
func (c *Client) createAnalysisPrompt(content string) string {
	return fmt.Sprintf(`You are a journalism expert. Analyze this news article and determine if it's PRIMARY REPORTING or REFERENCED REPORTING.

**REFERENCED REPORTING** (mark as false) - Article is primarily based on external sources:
- Explicitly cites OTHER news organizations as the main source of information
- Lead paragraph or headline attributes the story to external sources
- Contains phrases like "According to [External Source]", "[Source] reports", "As reported by [Source]"
- Main facts come from another outlet's reporting, not original work
- Uses phrases like "reports say", "sources report", "it was reported" when referring to external sources
- Story would not exist without the external source's original reporting

**PRIMARY REPORTING** (mark as true) - Outlet did original journalism:
- Original interviews, investigation, or direct coverage by the outlet's staff
- Contains exclusive information, original quotes, or firsthand reporting
- Self-references: outlet references its own reporters, coverage, or internal sources
- Original analysis or commentary on events, even if mentioning other sources
- Mixed content: mentions other sources but includes substantial original reporting

**DECISION PRIORITY:**
1. FIRST: Look for explicit attribution to external sources in headlines/lead paragraphs
2. SECOND: Check if the main story facts come from external sources vs. original reporting
3. THIRD: Self-references to same outlet = PRIMARY; references to different outlets = REFERENCED

**EXAMPLES:**
- REFERENCED: "According to Reuters, the company announced..." → {"is_primary_reporting": false, "original_source_name": "Reuters"}
- REFERENCED: "CNN reports that the president said..." → {"is_primary_reporting": false, "original_source_name": "CNN"}
- REFERENCED: "As first reported by Bloomberg, the deal was..." → {"is_primary_reporting": false, "original_source_name": "Bloomberg"}
- REFERENCED: "Sources tell multiple outlets that..." → {"is_primary_reporting": false, "original_source_name": "Unknown"}
- PRIMARY: "Our reporter spoke with the mayor..." → {"is_primary_reporting": true, "original_source_name": null}
- PRIMARY: "TechCrunch has learned that..." → {"is_primary_reporting": true, "original_source_name": null}
- PRIMARY: "In an exclusive interview, the CEO told us..." → {"is_primary_reporting": true, "original_source_name": null}
- PRIMARY: "Analysis: While Reuters reported X, our investigation shows..." → {"is_primary_reporting": true, "original_source_name": null}

Article to analyze:
%s

Respond with ONLY this JSON format (no extra text):
{
  "is_primary_reporting": true,
  "original_source_name": null,
  "confidence": 0.95,
  "reasoning": "Brief explanation of your decision"
}

For referenced reporting, set "original_source_name" to the main source name (e.g. "Reuters", "BBC News", "CNN") or "Unknown" if no specific source is identified.
For primary reporting, set "original_source_name" to null.`, content)
}

// makeAPIRequest makes the HTTP request to OpenAI API
func (c *Client) makeAPIRequest(req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var response ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &response, nil
}

// parseAnalysisResponse parses the GPT response into our structured format
func (c *Client) parseAnalysisResponse(content string) (*models.ContentAnalysisResponse, error) {
	// Try to extract JSON from the response (GPT might include extra text)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}") + 1

	if start == -1 || end == 0 {
		// Log the full response for debugging
		fmt.Printf("LLM Analysis - No JSON found in response: %s\n", content)
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := content[start:end]

	var response struct {
		IsPrimaryReporting bool        `json:"is_primary_reporting"`
		OriginalSourceName interface{} `json:"original_source_name"` // Use interface{} to handle null properly
		Confidence         float64     `json:"confidence"`
		Reasoning          string      `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		fmt.Printf("LLM Analysis - Failed to parse JSON: %s, Error: %v\n", jsonStr, err)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	log.Printf("🔍 LLM Analysis - Parsed response: IsPrimaryReporting=%v, OriginalSourceName=%v, Confidence=%.2f",
		response.IsPrimaryReporting, response.OriginalSourceName, response.Confidence)

	// Handle the original_source_name field properly
	var sourceName *string
	switch v := response.OriginalSourceName.(type) {
	case string:
		if v != "" && v != "null" {
			sourceName = &v
		}
	case nil:
		sourceName = nil
	default:
		// Handle other types gracefully
		sourceName = nil
	}

	result := &models.ContentAnalysisResponse{
		IsPrimaryReporting: response.IsPrimaryReporting,
		OriginalSourceName: sourceName,
		Confidence:         response.Confidence,
		Reasoning:          response.Reasoning,
	}

	// Validate confidence is reasonable (between 0 and 1)
	if result.Confidence < 0 || result.Confidence > 1 {
		log.Printf("🔍 LLM Analysis - Invalid confidence value: %.2f, setting to 0.5", result.Confidence)
		result.Confidence = 0.5
	}

	return result, nil
}

// ruleBasedAnalysis provides a simple rule-based fallback for obvious cases
func (c *Client) ruleBasedAnalysis(content string) *models.ContentAnalysisResponse {
	contentLower := strings.ToLower(content)

	// First check for VERY OBVIOUS referenced reporting cases - these should skip LLM
	obviousReferencedIndicators := []struct {
		phrase string
		source string
	}{
		// Very explicit attribution phrases to EXTERNAL sources at start of content
		{"according to reuters,", "Reuters"},
		{"reuters reports that", "Reuters"},
		{"reuters reported that", "Reuters"},
		{"according to cnn,", "CNN"},
		{"cnn reports that", "CNN"},
		{"cnn reported that", "CNN"},
		{"according to bbc,", "BBC News"},
		{"bbc reports that", "BBC News"},
		{"bbc reported that", "BBC News"},
		{"according to ap,", "Associated Press"},
		{"associated press reports", "Associated Press"},
		{"according to bloomberg,", "Bloomberg"},
		{"bloomberg reports that", "Bloomberg"},
		{"bloomberg reported that", "Bloomberg"},
		{"according to wsj,", "Wall Street Journal"},
		{"according to the wall street journal,", "Wall Street Journal"},
		{"wall street journal reports", "Wall Street Journal"},
		{"according to the new york times,", "New York Times"},
		{"new york times reports", "New York Times"},
		{"first reported by", "Unknown"},
		{"originally reported by", "Unknown"},
	}

	// Check if the content starts with these phrases (more reliable)
	contentStart := contentLower
	if len(contentStart) > 200 {
		contentStart = contentStart[:200] // Check first 200 chars
	}

	for _, indicator := range obviousReferencedIndicators {
		if strings.Contains(contentStart, indicator.phrase) {
			log.Printf("🔍 Rule-based Analysis - Found obvious referenced indicator: '%s' -> Source: %s", indicator.phrase, indicator.source)
			sourceCopy := indicator.source
			return &models.ContentAnalysisResponse{
				IsPrimaryReporting: false,
				OriginalSourceName: &sourceCopy,
				Confidence:         0.9,
				Reasoning:          fmt.Sprintf("Rule-based detection: Found explicit attribution '%s' indicating referenced reporting", indicator.phrase),
			}
		}
	}

	// Check for very clear self-references (but be more selective)
	strongSelfReferenceIndicators := []string{
		"our exclusive interview",
		"our investigation found",
		"our investigation revealed",
		"our reporters found",
		"our team discovered",
		"we exclusively learned",
		"we can exclusively report",
		"exclusive: ",
		"breaking: our ",
	}

	for _, indicator := range strongSelfReferenceIndicators {
		if strings.Contains(contentLower, indicator) {
			log.Printf("🔍 Rule-based Analysis - Found strong self-reference: '%s' -> PRIMARY REPORTING", indicator)
			return &models.ContentAnalysisResponse{
				IsPrimaryReporting: true,
				OriginalSourceName: nil,
				Confidence:         0.9,
				Reasoning:          fmt.Sprintf("Rule-based detection: Found strong self-reference '%s' indicating primary reporting", indicator),
			}
		}
	}

	// Let LLM handle the rest - don't be too aggressive with rule-based analysis
	return nil
}
