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
	BatchComplete string `json:"batchcomplete"`
	Query         Query  `json:"query"`
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
	req, err := http.NewRequestWithContext(ctx, "GET", "https://pt.wikipedia.org/w/api.php", nil)
	if err != nil {
		return WikipediaResponse{}, err
	}

	query := req.URL.Query()
	query.Add("action", "query")
	query.Add("prop", "revisions")
	query.Add("rvprop", "content")
	query.Add("format", "json")
	query.Add("titles", titles)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return WikipediaResponse{}, err
	}

	if res.StatusCode != 200 {
		return WikipediaResponse{}, fmt.Errorf("Error fetching data: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()

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
