package gen

import (
	"boteco/internal/gen/tools"
	"context"
	"fmt"
	"iter"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
)

var Tools []ai.ToolRef

func InitGenkit() (*genkit.Genkit, error) {
	apiKey, exists := os.LookupEnv("OPENAI_API_KEY")
	if !exists {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	g := genkit.Init(context.Background(),
		genkit.WithPlugins(&openai.OpenAI{APIKey: apiKey}),
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
		ai.WithModelName("openai/gpt-5-nano"),
		ai.WithSystem(system),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...),
		ai.WithMessages(messages...),
	)

	return resp.Text(), err
}

func GenerateStream(g *genkit.Genkit, system, prompt string, tools []ai.ToolRef,
	outputFormat any, messages []*ai.Message) iter.Seq2[*ai.ModelStreamValue, error] {
	resp := genkit.GenerateStream(
		context.Background(), g,
		ai.WithModelName("openai/gpt-5-nano"),
		ai.WithSystem(system),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...),
		ai.WithMessages(messages...),
	)

	return resp
}
