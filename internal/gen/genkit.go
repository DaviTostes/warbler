package gen

import (
	"boteco/internal/gen/tools"
	"context"
	"iter"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

var Tools []ai.ToolRef

func InitGenkit() (*genkit.Genkit, error) {
	g := genkit.Init(context.Background(),
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
	)

	Tools = []ai.ToolRef{
		tools.WebSearchTool(g),
		tools.CreateEventTool(g),
		tools.FetchEvents(g),
	}

	return g, nil
}

func Generate(g *genkit.Genkit, system, prompt string, tools []ai.ToolRef,
	outputFormat any, messages []*ai.Message) (string, error) {
	resp, err := genkit.Generate(
		context.Background(), g,
		ai.WithModelName("googleai/gemini-3.1-pro-preview"),
		ai.WithSystem(system),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...),
		ai.WithMaxTurns(25),
		ai.WithMessages(messages...),
	)

	return resp.Text(), err
}

func GenerateStream(g *genkit.Genkit, system, prompt string, tools []ai.ToolRef,
	outputFormat any, messages []*ai.Message) iter.Seq2[*ai.ModelStreamValue, error] {
	resp := genkit.GenerateStream(
		context.Background(), g,
		ai.WithModelName("googleai/gemini-3.1-pro-preview"),
		ai.WithSystem(system),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...),
		ai.WithMessages(messages...),
	)

	return resp
}
