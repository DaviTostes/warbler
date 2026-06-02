package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func WebSearch(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("empty query")
	}

	endpoint := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) Gecko/1.0")

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

	results := parseDDG(string(body))
	if len(results) == 0 {
		log.Printf("DDG zero results, status=%d body_len=%d head=%.200q", resp.StatusCode, len(body), string(body))
	}
	if len(results) > 5 {
		results = results[:5]
	}

	out, err := json.Marshal(results)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

var (
	reResult = regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>.*?<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)
	reTag    = regexp.MustCompile(`<[^>]+>`)
	reSpaces = regexp.MustCompile(`\s+`)
)

func parseDDG(html string) []SearchResult {
	matches := reResult.FindAllStringSubmatch(html, -1)
	results := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		u := cleanDDGURL(m[1])
		title := stripHTML(m[2])
		snippet := stripHTML(m[3])
		if u == "" || title == "" {
			continue
		}
		results = append(results, SearchResult{Title: title, URL: u, Snippet: snippet})
	}
	return results
}

func stripHTML(s string) string {
	s = reTag.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func cleanDDGURL(raw string) string {
	if strings.HasPrefix(raw, "//duckduckgo.com/l/") || strings.HasPrefix(raw, "/l/") {
		if u, err := url.Parse(raw); err == nil {
			if real := u.Query().Get("uddg"); real != "" {
				if decoded, err := url.QueryUnescape(real); err == nil {
					return decoded
				}
			}
		}
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}

type WebSearchInput struct {
	Query      string          `json:"query,omitempty"`
	Parameters *WebSearchInner `json:"parameters,omitempty"`
}

type WebSearchInner struct {
	Query string `json:"query"`
}

func WebSearchTool(g *genkit.Genkit) *ai.ToolDef[WebSearchInput, string] {
	return genkit.DefineTool(g, "web_search",
		"Search the web for current or time-sensitive information. Pass {\"query\": \"...\"}.",
		func(ctx *ai.ToolContext, input WebSearchInput) (string, error) {
			q := input.Query
			if q == "" && input.Parameters != nil {
				q = input.Parameters.Query
			}

			result, err := WebSearch(ctx.Context, q)
			if err != nil {
				return "", err
			}

			return result, nil
		})
}
