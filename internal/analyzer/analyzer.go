package analyzer

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sananguliyev/webpage-analyzer/internal/model"
)

// DetectHTMLVersion inspects raw HTML bytes for a DOCTYPE declaration.
func DetectHTMLVersion(body []byte) string {
	upper := strings.ToUpper(string(body[:min(len(body), 512)]))
	upper = strings.TrimSpace(upper)

	if !strings.HasPrefix(upper, "<!DOCTYPE") {
		return "Unknown"
	}

	switch {
	case strings.Contains(upper, "<!DOCTYPE HTML>"):
		return "HTML5"
	case strings.Contains(upper, "XHTML 1.1"):
		return "XHTML 1.1"
	case strings.Contains(upper, "XHTML 1.0 STRICT"):
		return "XHTML 1.0 Strict"
	case strings.Contains(upper, "XHTML 1.0 TRANSITIONAL"):
		return "XHTML 1.0 Transitional"
	case strings.Contains(upper, "XHTML 1.0 FRAMESET"):
		return "XHTML 1.0 Frameset"
	case strings.Contains(upper, "HTML 4.01 TRANSITIONAL"):
		return "HTML 4.01 Transitional"
	case strings.Contains(upper, "HTML 4.01 FRAMESET"):
		return "HTML 4.01 Frameset"
	case strings.Contains(upper, "HTML 4.01"):
		return "HTML 4.01 Strict"
	default:
		return "Unknown"
	}
}

// Analyze parses HTML body and extracts page analysis metrics.
func Analyze(body []byte, baseURL *url.URL) (*model.PageAnalysis, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Check for <base href="..."> override
	effectiveBase := baseURL
	if baseHref, exists := doc.Find("base[href]").First().Attr("href"); exists {
		if parsed, err := url.Parse(baseHref); err == nil {
			effectiveBase = baseURL.ResolveReference(parsed)
		}
	}

	analysis := &model.PageAnalysis{
		HTMLVersion: DetectHTMLVersion(body),
		Title:       extractTitle(doc),
		Headings:    countHeadings(doc),
	}

	links := extractLinks(doc, effectiveBase, baseURL)
	analysis.Links = links
	analysis.TotalLinks = len(links)

	for _, l := range links {
		if l.IsInternal {
			analysis.InternalLinks++
		} else {
			analysis.ExternalLinks++
		}
	}

	analysis.HasLoginForm = detectLoginForm(doc)

	return analysis, nil
}

func extractTitle(doc *goquery.Document) string {
	return strings.TrimSpace(doc.Find("title").First().Text())
}

func countHeadings(doc *goquery.Document) map[string]int {
	headings := make(map[string]int)
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		count := doc.Find(tag).Length()
		if count > 0 {
			headings[tag] = count
		}
	}
	return headings
}

func extractLinks(doc *goquery.Document, resolveBase, classifyBase *url.URL) []model.LinkResult {
	seen := make(map[string]bool)
	var links []model.LinkResult

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		href = strings.TrimSpace(href)
		if href == "" {
			return
		}

		// Skip non-HTTP schemes
		lower := strings.ToLower(href)
		if strings.HasPrefix(lower, "mailto:") ||
			strings.HasPrefix(lower, "tel:") ||
			strings.HasPrefix(lower, "javascript:") {
			return
		}

		// Skip fragment-only links
		if strings.HasPrefix(href, "#") {
			return
		}

		parsed, err := url.Parse(href)
		if err != nil {
			return
		}

		resolved := resolveBase.ResolveReference(parsed)
		// Normalize: strip fragment
		resolved.Fragment = ""
		fullURL := resolved.String()

		if seen[fullURL] {
			return
		}
		seen[fullURL] = true

		isInternal := strings.EqualFold(resolved.Hostname(), classifyBase.Hostname())

		links = append(links, model.LinkResult{
			URL:        fullURL,
			IsInternal: isInternal,
		})
	})

	return links
}

func detectLoginForm(doc *goquery.Document) bool {
	found := false
	doc.Find("input").Each(func(_ int, s *goquery.Selection) {
		inputType, _ := s.Attr("type")
		if strings.EqualFold(inputType, "password") {
			found = true
		}
	})
	return found
}
