package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type WikipediaResponse struct {
	BatchComplete bool `json:"batchcomplete"`
	Query         struct {
		Pages []struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Extract string `json:"extract"` // The clean plain-text summary lives here
		} `json:"pages"`
	} `json:"query"`
}

type Query struct {
	Pages map[string]Page `json:"pages"`
}

type Page struct {
	PageID    int        `json:"pageid"`
	Ns        int        `json:"ns"`
	Title     string     `json:"title"`
	Revisions []Revision `json:"revisions"`
}

type Revision struct {
	ContentFormat string `json:"contentformat"`
	ContentModel  string `json:"contentmodel"`
	Content       string `json:"*"` // Maps the "*" key
}

func wikiSearch(ctx context.Context, titles string) (WikipediaResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://en.wikipedia.org/w/api.php", nil)
	if err != nil {
		return WikipediaResponse{}, err
	}

	req.Header.Add("User-Agent", "Warbler/1.0 (davisiqueira591@gmail.com)")

	query := req.URL.Query()
	query.Add("action", "query")
	query.Add("prop", "extracts")
	query.Add("exintro", "1")
	query.Add("explaintext", "1")
	query.Add("format", "json")
	query.Add("formatversion", "2")
	query.Add("titles", titles)

	req.URL.RawQuery = query.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return WikipediaResponse{}, err
	}

	if res.StatusCode != 200 {
		return WikipediaResponse{}, fmt.Errorf("Error fetching data: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()

	// fmt.Println(string(body))

	var structBody WikipediaResponse
	err = json.Unmarshal(body, &structBody)
	if err != nil {
		return WikipediaResponse{}, err
	}

	return structBody, nil
}

type WikiSearchInput struct {
	Titles string `json:"titles"`
}

func WikiSearchTool(g *genkit.Genkit) *ai.ToolDef[WikiSearchInput, WikipediaResponse] {
	return genkit.DefineTool(g, "wiki_search",
		`Search the wikipedia for information about specific topics related to the user's prompt. `+
			`Pass {"titles": "..."}`+
			`Trust the returned results over your own knowledge.`,
		func(ctx *ai.ToolContext, input WikiSearchInput) (WikipediaResponse, error) {
			return wikiSearch(ctx.Context, input.Titles)
		})
}
