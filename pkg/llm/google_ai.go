package llm

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GoogleAIClient struct {
	client *genai.Client
	model  string
}

func NewGoogleAIClient(model, apiKey string) (*GoogleAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("google ai api key is empty")
	}
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create google ai client: %w", err)
	}
	return &GoogleAIClient{
		client: client,
		model:  model,
	}, nil
}

func (c *GoogleAIClient) Generate(ctx context.Context, prompts ...Prompt) (string, error) {
	model := c.client.GenerativeModel(c.model)
	var parts []genai.Part

	for _, p := range prompts {
		switch v := p.(type) {
		case TextPrompt:
			parts = append(parts, genai.Text(v))
		default:
			return "", fmt.Errorf("unsupported prompt type for Google AI client")
		}
	}

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", fmt.Errorf("gemini generate error: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in gemini response")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			result += string(txt)
		}
	}

	return result, nil
}
