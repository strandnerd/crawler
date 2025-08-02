package parser

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

// HTMLCleaner provides functionality to clean and sanitize HTML content
type HTMLCleaner struct {
	policy   *bluemonday.Policy
	adRegexp *regexp.Regexp
}

// NewHTMLCleaner creates a new HTML cleaner with predefined cleaning rules
func NewHTMLCleaner() *HTMLCleaner {
	// Create a strict policy that allows common content elements but removes navigation and ads
	policy := bluemonday.NewPolicy()

	// Allow basic text formatting
	policy.AllowElements("p", "br", "strong", "b", "em", "i", "u", "strike", "del", "ins")
	
	// Allow headings
	policy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	
	// Allow lists
	policy.AllowElements("ul", "ol", "li")
	
	// Allow links with limited attributes
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowElements("a")
	
	// Allow images with limited attributes
	policy.AllowAttrs("src", "alt", "title").OnElements("img")
	policy.AllowElements("img")
	
	// Allow quotes and code
	policy.AllowElements("blockquote", "q", "cite", "code", "pre")
	
	// Allow tables
	policy.AllowElements("table", "thead", "tbody", "tr", "td", "th")
	
	// Allow divs and spans but strip most attributes
	policy.AllowElements("div", "span")
	
	// Compile regex for ad detection
	adPattern := `(?i)(advertisement|ad-container|ads|sidebar|nav|navigation|menu|header|footer|comments|social|share|related|popup|overlay|banner|promo|sponsored|widget)`
	adRegexp := regexp.MustCompile(adPattern)

	return &HTMLCleaner{
		policy:   policy,
		adRegexp: adRegexp,
	}
}

// CleanHTML removes unwanted elements and sanitizes HTML content
func (hc *HTMLCleaner) CleanHTML(rawHTML string) string {
	if rawHTML == "" {
		return ""
	}

	// Parse HTML to work with DOM structure
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		// If parsing fails, fall back to basic cleaning
		return hc.basicClean(rawHTML)
	}

	// Remove unwanted elements before sanitization
	hc.removeUnwantedElements(doc)

	// Convert back to HTML string
	var buf strings.Builder
	html.Render(&buf, doc)
	cleanedHTML := buf.String()

	// Apply bluemonday sanitization
	sanitized := hc.policy.Sanitize(cleanedHTML)

	// Additional post-processing
	sanitized = hc.postProcess(sanitized)

	return sanitized
}

// removeUnwantedElements recursively removes navigation, ads, and other unwanted elements
func (hc *HTMLCleaner) removeUnwantedElements(n *html.Node) {
	if n == nil {
		return
	}

	// Check if current node should be removed
	if hc.shouldRemoveElement(n) {
		// Remove this node by unlinking it from its parent
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
		return
	}

	// Process children (iterate carefully since we might be removing nodes)
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		hc.removeUnwantedElements(c)
		c = next
	}
}

// shouldRemoveElement determines if an element should be removed
func (hc *HTMLCleaner) shouldRemoveElement(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	// Remove common navigation and layout elements
	unwantedTags := map[string]bool{
		"nav":    true,
		"aside":  true,
		"footer": true,
		"header": true,
		"script": true,
		"style":  true,
		"noscript": true,
		"iframe": true,
		"object": true,
		"embed":  true,
		"form":   true,
		"input":  true,
		"button": true,
		"select": true,
		"textarea": true,
	}

	if unwantedTags[n.Data] {
		return true
	}

	// Check attributes for ad/navigation indicators
	for _, attr := range n.Attr {
		value := strings.ToLower(attr.Val)
		
		// Check class, id, and other attributes for unwanted patterns
		if attr.Key == "class" || attr.Key == "id" || attr.Key == "role" {
			if hc.adRegexp.MatchString(value) {
				return true
			}
			
			// Additional specific patterns
			unwantedPatterns := []string{
				"social", "share", "comment", "related", "popup", "modal",
				"subscription", "newsletter", "cookie", "gdpr", "privacy",
				"search", "login", "signup", "register", "breadcrumb",
			}
			
			for _, pattern := range unwantedPatterns {
				if strings.Contains(value, pattern) {
					return true
				}
			}
		}
		
		// Remove data tracking attributes
		if strings.HasPrefix(attr.Key, "data-") && 
		   (strings.Contains(attr.Key, "track") || 
		    strings.Contains(attr.Key, "analytics") ||
		    strings.Contains(attr.Key, "ga-")) {
			return true
		}
	}

	return false
}

// postProcess applies final cleaning steps
func (hc *HTMLCleaner) postProcess(content string) string {
	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	content = re.ReplaceAllString(content, " ")
	
	// Remove empty paragraphs and divs (Go regex doesn't support backreferences like \1)
	content = regexp.MustCompile(`<p[^>]*>\s*</p>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<div[^>]*>\s*</div>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<span[^>]*>\s*</span>`).ReplaceAllString(content, "")
	
	// Clean up multiple line breaks
	content = regexp.MustCompile(`(<br\s*/?>\s*){3,}`).ReplaceAllString(content, "<br><br>")
	
	// Remove HTML comments
	content = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(content, "")
	
	// Trim whitespace
	content = strings.TrimSpace(content)
	
	// Limit content length for processing efficiency
	if len(content) > 100000 {
		content = content[:100000] + "..."
	}

	return content
}

// basicClean provides fallback cleaning when HTML parsing fails
func (hc *HTMLCleaner) basicClean(content string) string {
	// Remove script and style tags completely
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	content = scriptRegex.ReplaceAllString(content, "")
	
	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	content = styleRegex.ReplaceAllString(content, "")
	
	// Remove common ad/navigation elements by tag
	unwantedTagsRegex := regexp.MustCompile(`(?i)<(nav|aside|footer|header|form)[^>]*>.*?</\1>`)
	content = unwantedTagsRegex.ReplaceAllString(content, "")
	
	// Apply bluemonday sanitization
	content = hc.policy.Sanitize(content)
	
	return hc.postProcess(content)
}

// ExtractCleanContent combines content extraction and cleaning
func (hc *HTMLCleaner) ExtractCleanContent(rawHTML string) string {
	// First, try to extract main content areas
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return hc.CleanHTML(rawHTML)
	}

	// Look for main content selectors
	contentSelectors := []string{
		"article",
		"main", 
		"[role=main]",
		".content",
		".post-content",
		".entry-content", 
		".article-content",
		".story-body",
		".post-body",
		".article-body",
	}

	for _, selector := range contentSelectors {
		if element := hc.findElementBySelector(doc, selector); element != nil {
			var buf strings.Builder
			html.Render(&buf, element)
			content := buf.String()
			if len(strings.TrimSpace(hc.extractTextContent(content))) > 100 {
				return hc.CleanHTML(content)
			}
		}
	}

	// Fallback to cleaning the entire document
	return hc.CleanHTML(rawHTML)
}

// findElementBySelector finds element by simple selector
func (hc *HTMLCleaner) findElementBySelector(n *html.Node, selector string) *html.Node {
	var result *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Handle attribute selectors like [role=main]
			if strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]") {
				attrPair := strings.Trim(selector, "[]")
				parts := strings.Split(attrPair, "=")
				if len(parts) == 2 {
					attrName := parts[0]
					attrValue := parts[1]
					for _, attr := range n.Attr {
						if attr.Key == attrName && attr.Val == attrValue {
							result = n
							return
						}
					}
				}
			} else if strings.HasPrefix(selector, ".") {
				// Handle class selector
				className := strings.TrimPrefix(selector, ".")
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

// extractTextContent extracts plain text from HTML for length estimation
func (hc *HTMLCleaner) extractTextContent(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	return hc.extractText(doc)
}

// extractText recursively extracts text from HTML node
func (hc *HTMLCleaner) extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		// Skip script and style tags
		if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
			continue
		}
		text.WriteString(hc.extractText(c))
	}
	return text.String()
}