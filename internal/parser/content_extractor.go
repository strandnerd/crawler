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
	platformSelectors map[string][]string
}

func NewContentExtractor(client *http.Client, config *config.Config) *ContentExtractor {
	return &ContentExtractor{
		client:      client,
		userAgent:   config.UserAgent,
		htmlCleaner: NewHTMLCleaner(),
		platformSelectors: initializePlatformSelectors(),
	}
}

// initializePlatformSelectors defines content selectors for specific platforms
func initializePlatformSelectors() map[string][]string {
	return map[string][]string{
		// TechCrunch
		"techcrunch.com": {
			".wp-block-post-content",
			".article-content",
			"[data-module='ArticleBody']",
			".post-content",
		},
		// Medium
		"medium.com": {
			"article section",
			"[data-testid='storyContent']",
			".story-content",
			"article div[data-selectable-paragraph]",
		},
		// The Verge
		"theverge.com": {
			".duet--article--article-body",
			"[data-testid='ArticleBodyWrapper']",
			".c-entry-content",
			".l-article-content",
		},
		// Ars Technica
		"arstechnica.com": {
			".post-content",
			"[itemprop='articleBody']",
			".article-content",
		},
		// Wired
		"wired.com": {
			"[data-testid='BodyWrapper']",
			".article__chunks",
			".content-header + div",
			"[data-testid='ArticleBodyWrapper']",
		},
		// Engadget
		"engadget.com": {
			"[data-module='ArticleBody']",
			".article-text",
			".o-article_body",
		},
		// TechRadar
		"techradar.com": {
			"[data-testid='article-body']",
			"#article-body",
			".text-copy",
		},
		// ZDNet
		"zdnet.com": {
			".storyBody",
			"[data-module='ArticleBody']",
			".content",
		},
		// BBC News
		"bbc.com": {
			"[data-component='text-block']",
			".story-body__inner",
			"[data-testid='article-text']",
		},
		// CNN
		"cnn.com": {
			".zn-body__paragraph",
			"[data-testid='article-content']",
			".l-container",
		},
		// Reuters
		"reuters.com": {
			"[data-testid='paragraph']",
			".ArticleBodyWrapper",
			".StandardArticleBody",
		},
		// The Guardian
		"theguardian.com": {
			"[data-gu-name='body']",
			".content__article-body",
			"#maincontent",
		},
		// New York Times
		"nytimes.com": {
			"section[name='articleBody']",
			".StoryBodyCompanionColumn",
			"[data-testid='articleBody']",
		},
		// Washington Post
		"washingtonpost.com": {
			"[data-testid='article-body']",
			".article-body",
			"#article-body",
		},
		// Wall Street Journal
		"wsj.com": {
			"[data-module='ArticleBody']",
			".wsj-snippet-body",
			".article-content",
		},
		// Forbes
		"forbes.com": {
			".article-body",
			"[data-testid='article-body']",
			".body-container",
		},
		// Hacker News (YCombinator)
		"news.ycombinator.com": {
			".comment",
			".commtext",
		},
		// Reddit (for posts that might be crawled)
		"reddit.com": {
			"[data-testid='post-content']",
			".md",
			"[data-click-id='text']",
		},
		// GitHub Blog
		"github.blog": {
			".post-content",
			"[data-testid='article-body']",
			".markdown-body",
		},
		// Stack Overflow Blog
		"stackoverflow.blog": {
			".s-prose",
			".post-content",
			"[itemprop='text']",
		},
		// Dev.to
		"dev.to": {
			"[data-article-id] .crayons-article__body",
			".article-body",
			"#article-body",
		},
		// Substack
		"substack.com": {
			".markup",
			"[data-testid='post-content']",
			".post-content",
		},
		// Blogger/Blogspot
		"blogspot.com": {
			".post-body",
			".entry-content",
			"[itemprop='articleBody']",
		},
		// WordPress.com
		"wordpress.com": {
			".entry-content",
			".post-content",
			"[data-testid='post-content']",
		},
		// Mashable
		"mashable.com": {
			"[data-testid='article-body']",
			".article-content",
			".blueprint",
		},
		// VentureBeat
		"venturebeat.com": {
			".article-content",
			"[data-module='ArticleBody']",
			".the-content",
		},
		// 9to5Mac, 9to5Google, etc.
		"9to5mac.com": {
			".post-content",
			"[data-testid='post-content']",
			".entry-content",
		},
		"9to5google.com": {
			".post-content",
			"[data-testid='post-content']",
			".entry-content",
		},
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
	rawContent := ce.extractMainContentHTMLWithURL(doc, pageURL)
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

// extractMainContentHTMLWithURL extracts the main content as HTML using platform-specific selectors with known URL
func (ce *ContentExtractor) extractMainContentHTMLWithURL(doc *html.Node, pageURL string) string {
	// Try platform-specific selectors first using the known URL
	if platformSelectors := ce.getPlatformSelectors(pageURL); len(platformSelectors) > 0 {
		for _, selector := range platformSelectors {
			if content := ce.extractHTMLFromSelector(doc, selector); content != "" {
				// Verify content quality before returning
				if ce.isGoodContent(content) {
					return ce.cleanHTML(content)
				}
			}
		}
	}

	// Fallback to document-based extraction
	return ce.extractMainContentHTML(doc)
}

// extractMainContentHTML extracts the main content as HTML using generic selectors
func (ce *ContentExtractor) extractMainContentHTML(doc *html.Node) string {
	// Try to extract the base URL from the document to determine platform
	baseURL := ce.extractBaseURL(doc)
	
	// Try platform-specific selectors if we found a URL in the document
	if baseURL != "" {
		if platformSelectors := ce.getPlatformSelectors(baseURL); len(platformSelectors) > 0 {
			for _, selector := range platformSelectors {
				if content := ce.extractHTMLFromSelector(doc, selector); content != "" {
					// Verify content quality before returning
					if ce.isGoodContent(content) {
						return ce.cleanHTML(content)
					}
				}
			}
		}
	}

	// Fallback to generic selectors
	genericSelectors := []string{
		"article",
		"main",
		".content",
		".post-content",
		".entry-content",
		".article-content",
		".story-body",
		".post-body",
		"[role=main]",
		"[itemprop='articleBody']",
		".article-body",
		".post-body-content",
		".article",
		".blog-post",
	}

	for _, selector := range genericSelectors {
		if content := ce.extractHTMLFromSelector(doc, selector); content != "" {
			if ce.isGoodContent(content) {
				return ce.cleanHTML(content)
			}
		}
	}

	// Last resort: try to find the largest content block
	return ce.extractLargestContentBlock(doc)
}

// extractBaseURL extracts the base URL from HTML document
func (ce *ContentExtractor) extractBaseURL(doc *html.Node) string {
	// Look for base tag first
	if baseTag := ce.findElementBySelector(doc, "base"); baseTag != nil {
		for _, attr := range baseTag.Attr {
			if attr.Key == "href" {
				return attr.Val
			}
		}
	}
	
	// Look for canonical URL in meta tags
	var f func(*html.Node) string
	f = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "link" {
			var rel, href string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "rel":
					rel = attr.Val
				case "href":
					href = attr.Val
				}
			}
			if rel == "canonical" && href != "" {
				return href
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := f(c); result != "" {
				return result
			}
		}
		return ""
	}
	
	return f(doc)
}

// getPlatformSelectors returns platform-specific selectors for a URL
func (ce *ContentExtractor) getPlatformSelectors(urlStr string) []string {
	if urlStr == "" {
		return nil
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}
	
	hostname := strings.ToLower(parsedURL.Hostname())
	
	// Direct hostname match
	if selectors, exists := ce.platformSelectors[hostname]; exists {
		return selectors
	}
	
	// Try removing www. prefix
	if strings.HasPrefix(hostname, "www.") {
		cleanHostname := strings.TrimPrefix(hostname, "www.")
		if selectors, exists := ce.platformSelectors[cleanHostname]; exists {
			return selectors
		}
	}
	
	// Try subdomain matching for known platforms
	for domain, selectors := range ce.platformSelectors {
		if strings.HasSuffix(hostname, "."+domain) || strings.HasSuffix(hostname, domain) {
			return selectors
		}
	}
	
	return nil
}

// isGoodContent evaluates if extracted content is meaningful
func (ce *ContentExtractor) isGoodContent(content string) bool {
	if content == "" {
		return false
	}
	
	// Extract text content for analysis
	textContent := ce.extractTextFromHTML(content)
	textContent = strings.TrimSpace(textContent)
	
	// Minimum length check
	if len(textContent) < 100 {
		return false
	}
	
	// Check for reasonable word count (more than just navigation)
	words := strings.Fields(textContent)
	if len(words) < 20 {
		return false
	}
	
	// Check content-to-HTML ratio (avoid overly marked-up content)
	if len(content) > 0 && float64(len(textContent))/float64(len(content)) < 0.1 {
		return false
	}
	
	// Avoid navigation-heavy content
	lowercaseText := strings.ToLower(textContent)
	navWords := []string{"menu", "navigation", "subscribe", "newsletter", "follow us", "social media", "share this"}
	navWordCount := 0
	for _, navWord := range navWords {
		navWordCount += strings.Count(lowercaseText, navWord)
	}
	
	// If more than 20% of the unique words are navigation-related, it's probably not article content
	if float64(navWordCount)/float64(len(words)) > 0.2 {
		return false
	}
	
	return true
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

// findElementBySelector finds element by selector (supports tag, class, id, and attribute selectors)
func (ce *ContentExtractor) findElementBySelector(n *html.Node, selector string) *html.Node {
	var result *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if ce.matchesSelector(n, selector) {
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

// matchesSelector checks if a node matches a given selector
func (ce *ContentExtractor) matchesSelector(n *html.Node, selector string) bool {
	selector = strings.TrimSpace(selector)
	
	// Handle attribute selectors like [data-testid='article-body'] or [role=main]
	if strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]") {
		attrPart := strings.Trim(selector, "[]")
		
		// Split on = to get attribute and value
		if strings.Contains(attrPart, "=") {
			parts := strings.SplitN(attrPart, "=", 2)
			attrName := strings.TrimSpace(parts[0])
			attrValue := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			
			for _, attr := range n.Attr {
				if attr.Key == attrName && attr.Val == attrValue {
					return true
				}
			}
		} else {
			// Just check for attribute existence
			for _, attr := range n.Attr {
				if attr.Key == strings.TrimSpace(attrPart) {
					return true
				}
			}
		}
		return false
	}
	
	// Handle ID selector
	if idName, ok := strings.CutPrefix(selector, "#"); ok {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == idName {
				return true
			}
		}
		return false
	}
	
	// Handle class selector
	if className, ok := strings.CutPrefix(selector, "."); ok {
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				classes := strings.Fields(attr.Val)
				for _, cls := range classes {
					if cls == className {
						return true
					}
				}
			}
		}
		return false
	}
	
	// Handle space-separated selectors (like "article section")
	if strings.Contains(selector, " ") {
		return ce.matchesDescendantSelector(n, selector)
	}
	
	// Handle tag name selector
	return n.Data == selector
}

// matchesDescendantSelector handles space-separated selectors like "article section"
func (ce *ContentExtractor) matchesDescendantSelector(n *html.Node, selector string) bool {
	parts := strings.Fields(selector)
	if len(parts) < 2 {
		return false
	}
	
	// Find if current node matches the last part (avoid infinite recursion)
	lastSelector := parts[len(parts)-1]
	if !ce.matchesSimpleSelector(n, lastSelector) {
		return false
	}
	
	// Check if any ancestor matches the previous selectors
	if len(parts) == 2 {
		// Simple case: just check if any ancestor matches the first selector
		ancestorSelector := parts[0]
		current := n.Parent
		
		for current != nil {
			if current.Type == html.ElementNode {
				if ce.matchesSimpleSelector(current, ancestorSelector) {
					return true
				}
			}
			current = current.Parent
		}
	} else {
		// Complex case: check if any ancestor matches the complex ancestor selector
		ancestorSelector := strings.Join(parts[:len(parts)-1], " ")
		current := n.Parent
		
		for current != nil {
			if current.Type == html.ElementNode {
				if ce.matchesDescendantSelector(current, ancestorSelector) {
					return true
				}
			}
			current = current.Parent
		}
	}
	
	return false
}

// matchesSimpleSelector checks if a node matches a simple (non-descendant) selector
func (ce *ContentExtractor) matchesSimpleSelector(n *html.Node, selector string) bool {
	selector = strings.TrimSpace(selector)
	
	// Handle attribute selectors like [data-testid='article-body'] or [role=main]
	if strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]") {
		attrPart := strings.Trim(selector, "[]")
		
		// Split on = to get attribute and value
		if strings.Contains(attrPart, "=") {
			parts := strings.SplitN(attrPart, "=", 2)
			attrName := strings.TrimSpace(parts[0])
			attrValue := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			
			for _, attr := range n.Attr {
				if attr.Key == attrName && attr.Val == attrValue {
					return true
				}
			}
		} else {
			// Just check for attribute existence
			for _, attr := range n.Attr {
				if attr.Key == strings.TrimSpace(attrPart) {
					return true
				}
			}
		}
		return false
	}
	
	// Handle ID selector
	if idName, ok := strings.CutPrefix(selector, "#"); ok {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == idName {
				return true
			}
		}
		return false
	}
	
	// Handle class selector
	if className, ok := strings.CutPrefix(selector, "."); ok {
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				classes := strings.Fields(attr.Val)
				for _, cls := range classes {
					if cls == className {
						return true
					}
				}
			}
		}
		return false
	}
	
	// Handle tag name selector
	return n.Data == selector
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
