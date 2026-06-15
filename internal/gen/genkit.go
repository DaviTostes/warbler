package gen

import (
	"boteco/internal/config"
	"boteco/internal/gen/tools"
	"context"
	"fmt"
	"iter"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

var Tools []ai.ToolRef
var model string

var validProviders = map[string]func(config.Config) (*genkit.Genkit, string){
	"gemini": func(c config.Config) (*genkit.Genkit, string) {
		return genkit.Init(context.Background(),
			genkit.WithPlugins(&googlegenai.GoogleAI{
				APIKey: c.Gemini.ApiKey,
			})), c.Gemini.Model
	},
}

func InitGenkit() (*genkit.Genkit, error) {
	c, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	fn, ok := validProviders[c.Default]
	if !ok {
		return nil, fmt.Errorf("Invalid provider")
	}

	var g *genkit.Genkit
	g, model = fn(c)

	Tools = []ai.ToolRef{
		tools.WebSearchTool(g),

		tools.CreateEventTool(g),
		tools.DeleteEventTool(g),
		tools.FetchEvents(g),

		tools.CreateMemoryTool(g),
		tools.DeleteMemoryTool(g),
		tools.FetchMemories(g),
	}

	return g, nil
}

func Generate(g *genkit.Genkit, system, prompt string, tools []ai.ToolRef, messages []*ai.Message) (string, error) {
	resp, err := genkit.Generate(
		context.Background(), g,
		ai.WithModelName(model),
		ai.WithSystem(system),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...),
		ai.WithMaxTurns(25),
		ai.WithMessages(messages...),
	)

	return resp.Text(), err
}

func GenerateStream(g *genkit.Genkit, system, prompt string, tools []ai.ToolRef, messages []*ai.Message) iter.Seq2[*ai.ModelStreamValue, error] {
	resp := genkit.GenerateStream(
		context.Background(), g,
		ai.WithModelName(model),
		ai.WithSystem(system),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...),
		ai.WithMessages(messages...),
		ai.WithMaxTurns(25),
	)

	return resp
}
