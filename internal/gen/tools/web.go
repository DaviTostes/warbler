package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"jaytaylor.com/html2text"
)

// === Web Search ===
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func WebSearch(ctx context.Context, query, freshness string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("empty query")
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("kl", "us-en")
	switch freshness {
	case "day":
		q.Set("df", "d")
	case "week":
		q.Set("df", "w")
	case "month":
		q.Set("df", "m")
	case "year":
		q.Set("df", "y")
	}

	endpoint := "https://html.duckduckgo.com/html/?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("DNT", "1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("ddg status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	results := parseDDGHTML(string(body))
	if len(results) > 5 {
		results = results[:5]
	}

	if len(results) == 0 {
		return `{"results":[],"note":"No results parsed. Do not answer from prior knowledge for time-sensitive facts; tell the user the search returned nothing and offer to retry."}`, nil
	}

	out, err := json.Marshal(map[string]any{"results": results})
	if err != nil {
		return "", err
	}

	return string(out), nil
}

var (
	reResultLink = regexp.MustCompile(`(?s)<a\b([^>]*\bclass="result__a"[^>]*)>(.*?)</a>`)
	reSnippet    = regexp.MustCompile(`(?s)<a\b[^>]*\bclass="result__snippet"[^>]*>(.*?)</a>`)
	reHref       = regexp.MustCompile(`href="([^"]+)"`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reSpaces     = regexp.MustCompile(`\s+`)
)

func parseDDGHTML(html string) []SearchResult {
	links := reResultLink.FindAllStringSubmatch(html, -1)
	snippets := reSnippet.FindAllStringSubmatch(html, -1)

	results := make([]SearchResult, 0, len(links))
	si := 0
	for _, m := range links {
		attrs, inner := m[1], m[2]
		hm := reHref.FindStringSubmatch(attrs)
		if hm == nil {
			continue
		}
		link := normalizeURL(hm[1])
		title := stripHTML(inner)
		if link == "" || title == "" {
			continue
		}
		snippet := ""
		if si < len(snippets) {
			snippet = stripHTML(snippets[si][1])
			si++
		}
		results = append(results, SearchResult{Title: title, URL: link, Snippet: snippet})
	}
	return results
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if strings.Contains(raw, "duckduckgo.com/l/") {
		if u, err := url.Parse(raw); err == nil {
			if target := u.Query().Get("uddg"); target != "" {
				return target
			}
		}
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if strings.Contains(raw, "duckduckgo.com") {
			return ""
		}
		return raw
	}
	return ""
}

func stripHTML(s string) string {
	s = reTag.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

type WebSearchInput struct {
	Query     string `json:"query"`
	Freshness string `json:"freshness,omitempty"`
}

func WebSearchTool(g *genkit.Genkit) *ai.ToolDef[WebSearchInput, string] {
	return genkit.DefineTool(g, "web_search",
		`Use this to search the web for current events. `+
			`CRITICAL INSTRUCTIONS: `+
			`1. Formulate specific, keyword-based queries (not full sentences). `+
			`2. If the search returns empty results or irrelevant snippets, do NOT repeat the same query. Try ONE alternative keyword query. `+
			`3. If the second attempt also fails, stop searching and answer based on your existing knowledge or state that you cannot find it.`,
		func(ctx *ai.ToolContext, input WebSearchInput) (string, error) {
			return WebSearch(ctx.Context, input.Query, input.Freshness)
		})
}

// === URL Fetch ===
var fetchClient = &http.Client{Timeout: 15 * time.Second}

func urlFetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("server returned status: %d", res.StatusCode)
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	text, err := html2text.FromString(string(bodyBytes), html2text.Options{TextOnly: true})
	if err != nil {
		return "", fmt.Errorf("failed to parse html: %w", err)
	}

	maxChars := 15000
	if len(text) > maxChars {
		text = text[:maxChars] + "\n\n[Texto truncado por limite de tamanho...]"
	}

	return text, nil
}

type UrlFetchInput struct {
	URL string `json:"url"`
}

func UrlFetchTool(g *genkit.Genkit) *ai.ToolDef[UrlFetchInput, string] {
	return genkit.DefineTool(g, "url_fetch",
		`Use this tool to read the full content of a specific web page. `+
			`Call this ONLY after using 'web_search' if the search snippet does not contain enough information. `+
			`Pass the exact URL you found in the search results.`,
		func(ctx *ai.ToolContext, input UrlFetchInput) (string, error) {
			return urlFetch(ctx.Context, input.URL)
		})
}
