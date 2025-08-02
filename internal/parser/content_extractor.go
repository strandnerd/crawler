package parser

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strandnerd-crawler/internal/config"
	"strings"

	"golang.org/x/net/html"
)

type ContentExtractor struct {
	client      *http.Client
	userAgent   string
	htmlCleaner *HTMLCleaner
}

func NewContentExtractor(client *http.Client, config *config.Config) *ContentExtractor {
	return &ContentExtractor{
		client:      client,
		userAgent:   config.UserAgent,
		htmlCleaner: NewHTMLCleaner(),
	}
}

type ExtractedContent struct {
	ImageURL    string
	FullContent string
}

// ExtractContentFromURL fetches the page and extracts the main content and image
func (ce *ContentExtractor) ExtractContentFromURL(pageURL string) (*ExtractedContent, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", ce.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := ce.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	extracted := &ExtractedContent{}

	// Extract Open Graph image (priority)
	extracted.ImageURL = ce.extractMainImage(doc, pageURL)

	// Extract and clean main content as HTML
	rawContent := ce.extractMainContentHTML(doc)
	extracted.FullContent = ce.htmlCleaner.CleanHTML(rawContent)

	return extracted, nil
}

// extractMainImage attempts to find the main image for the article
func (ce *ContentExtractor) extractMainImage(doc *html.Node, baseURL string) string {
	var imageURL string

	// Priority order: Open Graph image, Twitter card image, then fallback to content images
	selectors := []func(*html.Node) string{
		func(n *html.Node) string { return ce.findMetaProperty(n, "og:image") },
		func(n *html.Node) string { return ce.findMetaProperty(n, "twitter:image") },
		func(n *html.Node) string { return ce.findFirstImageInElement(n, "article") },
		func(n *html.Node) string { return ce.findFirstImageInElement(n, "main") },
		func(n *html.Node) string { return ce.findFirstImageInElement(n, ".content") },
		func(n *html.Node) string { return ce.findFirstImageInElement(n, ".post-content") },
		func(n *html.Node) string { return ce.findFirstImageInElement(n, ".entry-content") },
	}

	for _, selector := range selectors {
		if img := selector(doc); img != "" {
			imageURL = img
			break
		}
	}

	// Convert relative URLs to absolute
	if imageURL != "" {
		imageURL = ce.resolveURL(imageURL, baseURL)
	}

	return imageURL
}

// extractMainContentHTML extracts the main content as HTML
func (ce *ContentExtractor) extractMainContentHTML(doc *html.Node) string {
	// Priority order for content extraction
	selectors := []string{
		"article",
		"main",
		".content",
		".post-content",
		".entry-content",
		".article-content",
		".story-body",
		".post-body",
		"[role=main]",
	}

	for _, selector := range selectors {
		if content := ce.extractHTMLFromSelector(doc, selector); content != "" {
			return ce.cleanHTML(content)
		}
	}

	// Fallback: try to find the largest content block
	return ce.extractLargestContentBlock(doc)
}

// findMetaProperty finds meta tag with specific property
func (ce *ContentExtractor) findMetaProperty(n *html.Node, property string) string {
	var result string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var propertyAttr, contentAttr string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "property", "name":
					propertyAttr = attr.Val
				case "content":
					contentAttr = attr.Val
				}
			}
			if propertyAttr == property && contentAttr != "" {
				result = contentAttr
				return
			}
		}
		for c := n.FirstChild; c != nil && result == ""; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return result
}

// findFirstImageInElement finds first image within a specific element
func (ce *ContentExtractor) findFirstImageInElement(n *html.Node, elementSelector string) string {
	element := ce.findElementBySelector(n, elementSelector)
	if element == nil {
		return ""
	}
	return ce.findFirstImage(element)
}

// findFirstImage finds the first img tag
func (ce *ContentExtractor) findFirstImage(n *html.Node) string {
	var result string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, attr := range n.Attr {
				if attr.Key == "src" && attr.Val != "" {
					result = attr.Val
					return
				}
			}
		}
		for c := n.FirstChild; c != nil && result == ""; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return result
}

// findElementBySelector finds element by simple selector (tag name or class)
func (ce *ContentExtractor) findElementBySelector(n *html.Node, selector string) *html.Node {
	var result *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Handle class selector
			if className, ok := strings.CutPrefix(selector, "."); ok {
				for _, attr := range n.Attr {
					if attr.Key == "class" && strings.Contains(attr.Val, className) {
						result = n
						return
					}
				}
			} else if n.Data == selector {
				// Handle tag name selector
				result = n
				return
			}
		}
		for c := n.FirstChild; c != nil && result == nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return result
}

// extractHTMLFromSelector extracts HTML content from element matching selector
func (ce *ContentExtractor) extractHTMLFromSelector(n *html.Node, selector string) string {
	element := ce.findElementBySelector(n, selector)
	if element == nil {
		return ""
	}
	return ce.renderHTML(element)
}

// renderHTML converts HTML node back to HTML string
func (ce *ContentExtractor) renderHTML(n *html.Node) string {
	var buf strings.Builder
	html.Render(&buf, n)
	return buf.String()
}

// extractLargestContentBlock finds the element with the most content
func (ce *ContentExtractor) extractLargestContentBlock(doc *html.Node) string {
	var maxHTML string
	var maxLength int

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			htmlContent := ce.renderHTML(n)
			textLength := len(strings.TrimSpace(ce.extractTextFromHTML(htmlContent)))
			if textLength > maxLength && textLength > 100 { // Minimum content threshold
				maxHTML = htmlContent
				maxLength = textLength
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return ce.cleanHTML(maxHTML)
}

// extractTextFromHTML extracts plain text from HTML for length calculation
func (ce *ContentExtractor) extractTextFromHTML(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	return ce.extractText(doc)
}

// extractText recursively extracts text from HTML node
func (ce *ContentExtractor) extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		// Skip script and style tags
		if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
			continue
		}
		text.WriteString(ce.extractText(c))
	}
	return text.String()
}

// cleanHTML cleans up extracted HTML content
func (ce *ContentExtractor) cleanHTML(content string) string {
	if content == "" {
		return ""
	}

	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	content = re.ReplaceAllString(content, " ")

	// Trim and limit content length
	content = strings.TrimSpace(content)

	// Limit to reasonable length (e.g., 50,000 characters for HTML)
	if len(content) > 50000 {
		content = content[:50000] + "..."
	}

	return content
}

// resolveURL converts relative URL to absolute URL
func (ce *ContentExtractor) resolveURL(href, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}

	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	return base.ResolveReference(ref).String()
}
