package llm

import (
	"context"
	"fmt"
	"os"
)

// Prompt is a marker interface for different types of prompt parts.
type Prompt interface {
	isPrompt()
}

// TextPrompt represents a text part of a prompt.
type TextPrompt string

func (p TextPrompt) isPrompt() {}

// LLMClient is the generic interface for interacting with LLM providers.
type LLMClient interface {
	Generate(ctx context.Context, prompts ...Prompt) (string, error)
}

// ClientFactory is a function that creates a new LLMClient.
type ClientFactory func() (LLMClient, error)

// ModelRegistry maps model names to their corresponding ClientFactory.
var ModelRegistry = map[string]ClientFactory{
	// Volcengine Models
	"doubao-pro-4-32k": func() (LLMClient, error) { return NewVolcengineClient("doubao-pro-4-32k", os.Getenv("ARK_API_KEY")) },
	"doubao-seed-1-8-251228": func() (LLMClient, error) {
		return NewVolcengineClient("doubao-seed-1-8-251228", os.Getenv("ARK_API_KEY"))
	},

	// Google AI Models
	"gemini-3-pro-preview": func() (LLMClient, error) {
		return NewGoogleAIClient("gemini-3-pro-preview", os.Getenv("GEMINI_API_KEY"))
	},
	"gemini-3-flash-preview": func() (LLMClient, error) {
		return NewGoogleAIClient("gemini-3-flash-preview", os.Getenv("GEMINI_API_KEY"))
	},
	"gemini-2.5-flash": func() (LLMClient, error) { return NewGoogleAIClient("gemini-2.5-flash", os.Getenv("GEMINI_API_KEY")) },
	"gemini-2.0-flash": func() (LLMClient, error) { return NewGoogleAIClient("gemini-2.0-flash", os.Getenv("GEMINI_API_KEY")) },
	"gemini-1.5-pro":   func() (LLMClient, error) { return NewGoogleAIClient("gemini-1.5-pro", os.Getenv("GEMINI_API_KEY")) },
	"gemini-1.5-flash": func() (LLMClient, error) { return NewGoogleAIClient("gemini-1.5-flash", os.Getenv("GEMINI_API_KEY")) },
}

// NewClient creates a new LLMClient based on the provided model name.
func NewClient(modelName string) (LLMClient, error) {
	if factory, ok := ModelRegistry[modelName]; ok {
		return factory()
	}
	return nil, fmt.Errorf("unknown model: %s", modelName)
}
