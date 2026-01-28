package llm

import (
	"context"
	"fmt"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

type VolcengineClient struct {
	client *arkruntime.Client
	model  string
}

func NewVolcengineClient(model, apiKey string) (*VolcengineClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("volcengine api key is empty")
	}
	client := arkruntime.NewClientWithApiKey(apiKey)
	return &VolcengineClient{
		client: client,
		model:  model,
	}, nil
}

func (c *VolcengineClient) Generate(ctx context.Context, prompts ...Prompt) (string, error) {
	var content []*responses.ContentItem
	for _, p := range prompts {
		switch v := p.(type) {
		case TextPrompt:
			content = append(content, &responses.ContentItem{
				Union: &responses.ContentItem_Text{
					Text: &responses.ContentItemText{
						Type: responses.ContentItemType_input_text,
						Text: string(v),
					},
				},
			})
		default:
			return "", fmt.Errorf("unsupported prompt type for Volcengine client")
		}
	}

	req := &responses.ResponsesRequest{
		Model: c.model,
		Input: &responses.ResponsesInput{
			Union: &responses.ResponsesInput_ListValue{
				ListValue: &responses.InputItemList{ListValue: []*responses.InputItem{{
					Union: &responses.InputItem_InputMessage{
						InputMessage: &responses.ItemInputMessage{
							Role:    responses.MessageRole_user,
							Content: content,
						},
					},
				}}},
			},
		},
	}

	resp, err := c.client.CreateResponses(ctx, req, arkruntime.WithProjectName("eval-transcript"))
	if err != nil {
		return "", fmt.Errorf("ark API error: %w", err)
	}

	if len(resp.Output) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	// Find message content in outputs
	for _, item := range resp.Output {
		if msg := item.GetOutputMessage(); msg != nil && len(msg.Content) > 0 {
			if textContent := msg.Content[0].GetText(); textContent != nil {
				return textContent.Text, nil
			}
		}
	}

	return "", fmt.Errorf("no text content found in model response")
}
