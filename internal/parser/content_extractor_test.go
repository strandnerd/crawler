package parser

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"strandnerd-crawler/internal/config"

	"golang.org/x/net/html"
)

func TestGetPlatformSelectors(t *testing.T) {
	config := &config.Config{UserAgent: "test"}
	client := &http.Client{Timeout: 30 * time.Second}
	extractor := NewContentExtractor(client, config)

	tests := []struct {
		url             string
		expectedSelectors []string
	}{
		{
			url:             "https://techcrunch.com/article",
			expectedSelectors: []string{".wp-block-post-content", ".article-content", "[data-module='ArticleBody']", ".post-content"},
		},
		{
			url:             "https://www.techcrunch.com/article",
			expectedSelectors: []string{".wp-block-post-content", ".article-content", "[data-module='ArticleBody']", ".post-content"},
		},
		{
			url:             "https://medium.com/@user/article",
			expectedSelectors: []string{"article section", "[data-testid='storyContent']", ".story-content", "article div[data-selectable-paragraph]"},
		},
		{
			url:             "https://unknown-site.com/article",
			expectedSelectors: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			selectors := extractor.getPlatformSelectors(test.url)
			
			if test.expectedSelectors == nil {
				if selectors != nil {
					t.Errorf("Expected nil selectors for %s, got %v", test.url, selectors)
				}
				return
			}
			
			if len(selectors) != len(test.expectedSelectors) {
				t.Errorf("Expected %d selectors for %s, got %d", len(test.expectedSelectors), test.url, len(selectors))
				return
			}
			
			for i, expected := range test.expectedSelectors {
				if selectors[i] != expected {
					t.Errorf("Expected selector %s at position %d for %s, got %s", expected, i, test.url, selectors[i])
				}
			}
		})
	}
}

func TestMatchesSelector(t *testing.T) {
	config := &config.Config{UserAgent: "test"}
	client := &http.Client{Timeout: 30 * time.Second}
	extractor := NewContentExtractor(client, config)

	tests := []struct {
		html     string
		selector string
		expected bool
	}{
		{
			html:     `<div class="wp-block-post-content">Content</div>`,
			selector: ".wp-block-post-content",
			expected: true,
		},
		{
			html:     `<div data-testid="article-body">Content</div>`,
			selector: `[data-testid="article-body"]`,
			expected: true,
		},
		{
			html:     `<div data-testid="article-body">Content</div>`,
			selector: `[data-testid='article-body']`,
			expected: true,
		},
		{
			html:     `<div id="article-body">Content</div>`,
			selector: "#article-body",
			expected: true,
		},
		{
			html:     `<article><section>Content</section></article>`,
			selector: "article section",
			expected: true,
		},
		{
			html:     `<div><article><section>Content</section></article></div>`,
			selector: "article section",
			expected: true,
		},
		{
			html:     `<div class="other">Content</div>`,
			selector: ".wp-block-post-content",
			expected: false,
		},
	}

	for i, test := range tests {
		t.Run(test.selector, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("Failed to parse HTML: %v", err)
			}

			// Helper functions for test
			hasClass := func(n *html.Node, className string) bool {
				for _, attr := range n.Attr {
					if attr.Key == "class" && strings.Contains(attr.Val, className) {
						return true
					}
				}
				return false
			}
			
			hasID := func(n *html.Node, idName string) bool {
				for _, attr := range n.Attr {
					if attr.Key == "id" && attr.Val == idName {
						return true
					}
				}
				return false
			}

			// Find the target element in the test HTML based on selector expectation
			var targetNode *html.Node
			var findNode func(*html.Node)
			findNode = func(n *html.Node) {
				if n.Type == html.ElementNode {
					// For descendant selectors, find the deepest matching element
					if strings.Contains(test.selector, " ") {
						parts := strings.Fields(test.selector)
						lastPart := parts[len(parts)-1]
						if (lastPart == "section" && n.Data == "section") ||
						   (strings.HasPrefix(lastPart, ".") && hasClass(n, strings.TrimPrefix(lastPart, "."))) ||
						   (strings.HasPrefix(lastPart, "#") && hasID(n, strings.TrimPrefix(lastPart, "#"))) {
							targetNode = n
							return
						}
					} else {
						// For simple selectors, find the first matching element
						if (n.Data == "div" || n.Data == "section") {
							targetNode = n
							return
						}
					}
				}
				for c := n.FirstChild; c != nil && targetNode == nil; c = c.NextSibling {
					findNode(c)
				}
			}
			findNode(doc)

			if targetNode == nil {
				t.Fatalf("Test %d: Could not find target node in HTML", i)
			}

			result := extractor.matchesSelector(targetNode, test.selector)
			if result != test.expected {
				t.Errorf("Test %d: Expected %v for selector %s on HTML %s, got %v", 
					i, test.expected, test.selector, test.html, result)
			}
		})
	}
}

func TestIsGoodContent(t *testing.T) {
	config := &config.Config{UserAgent: "test"}
	client := &http.Client{Timeout: 30 * time.Second}
	extractor := NewContentExtractor(client, config)

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "Good article content",
			content:  `<p>This is a long article with meaningful content that discusses various topics in detail. It has multiple sentences and provides valuable information to readers. The article continues with more detailed explanations and examples to help readers understand the concepts better.</p>`,
			expected: true,
		},
		{
			name:     "Too short content",
			content:  `<p>Short</p>`,
			expected: false,
		},
		{
			name:     "Navigation heavy content",
			content:  `<div>menu navigation subscribe newsletter follow us social media share this menu navigation subscribe newsletter</div>`,
			expected: false,
		},
		{
			name:     "Empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "Overly marked up content",
			content:  strings.Repeat(`<span><div><p>`, 100) + "Short text" + strings.Repeat(`</p></div></span>`, 100),
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := extractor.isGoodContent(test.content)
			if result != test.expected {
				t.Errorf("Expected %v for content quality check, got %v", test.expected, result)
			}
		})
	}
}