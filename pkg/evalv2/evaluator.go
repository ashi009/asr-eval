package evalv2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"reflect"

	"os"

	"google.golang.org/genai"
)

type Evaluator struct {
	client    *genai.Client
	genModel  string
	evalModel string
}

func NewEvaluator(client *genai.Client, genModel, evalModel string) *Evaluator {
	return &Evaluator{
		client:    client,
		genModel:  genModel,
		evalModel: evalModel,
	}
}

func (e *Evaluator) GenerateContext(ctx context.Context, audioPath string, groundTruth string, transcripts map[string]string) (*EvalContext, *genai.GenerateContentResponseUsageMetadata, error) {
	// 1. Prepare Audio Part
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	ext := filepath.Ext(audioPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "audio/flac" // Default to flac as per dataset
	}

	// 2. Prepare Text Prompt
	prompt, err := buildGenerateContextPrompt(generateContextPromptData{
		GroundTruth: groundTruth,
		Transcripts: transcripts,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build context prompt: %w", err)
	}

	// 3. Call LLM
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(prompt),
				genai.NewPartFromBytes(audioData, mimeType),
			},
		},
	}

	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
		},
	}

	var result EvalContext
	usage, err := e.generateJSON(ctx, e.genModel, contents, config, &result)
	if err != nil {
		return nil, usage, err
	}

	// 5. Post-process: Inject Ground Truth and Normalize Weights
	result.Meta.GroundTruth = groundTruth

	totalWeight := 0.0
	for _, cp := range result.Checkpoints {
		totalWeight += cp.Weight
	}
	if totalWeight > 0 {
		for i := range result.Checkpoints {
			result.Checkpoints[i].Weight /= totalWeight
		}
	}

	return &result, usage, nil
}

func (e *Evaluator) Evaluate(ctx context.Context, contextData *EvalContext, transcripts map[string]string) (*EvalReport, *genai.GenerateContentResponseUsageMetadata, error) {
	prompt, err := buildEvaluatePrompt(evaluatePromptData{
		EvalContext: contextData,
		Transcripts: transcripts,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build eval prompt: %w", err)
	}

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(prompt),
			},
		},
	}

	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevelLow,
		},
	}

	var resultLLM llmEvalReport
	usage, err := e.generateJSON(ctx, e.evalModel, contents, config, &resultLLM)
	if err != nil {
		return nil, usage, err
	}

	// Convert back to Map-based EvaluationResponse -> EvalReport
	finalResult := &EvalReport{
		Results: make(map[string]EvalResult),
	}

	for _, item := range resultLLM {
		cpResults := make(map[string]CheckpointResult)
		for _, cp := range item.CheckpointResults {
			cpResults[cp.ID] = CheckpointResult{
				Status:   cp.Status,
				Detected: cp.Detected,
				Reason:   cp.Reason,
			}
		}

		finalResult.Results[item.Provider] = EvalResult{
			Transcript:        transcripts[item.Provider],
			RevisedTranscript: item.RevisedTranscript,
			Metrics:           item.Metrics,
			CheckpointResults: cpResults,
			Summary:           item.Summary,
		}
	}

	return finalResult, usage, nil
}

func (e *Evaluator) generateJSON(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig, target interface{}) (*genai.GenerateContentResponseUsageMetadata, error) {
	// Automatically generate schema and set JSON response type
	config.ResponseMIMEType = "application/json"
	config.ResponseSchema = reflectSchema(reflect.TypeOf(target))

	// Log the prompt for debugging
	for i, content := range contents {
		for j, part := range content.Parts {
			if part.Text != "" {
				slog.Debug("LLM Prompt", "content_index", i, "part_index", j, "text", part.Text)
			}
		}
	}

	resp, err := e.client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	usage := resp.UsageMetadata
	if usage != nil {
		slog.Info("LLM Usage",
			slog.Int("prompt_tokens", int(usage.PromptTokenCount)),
			slog.Int("thought_tokens", int(usage.ThoughtsTokenCount)),
			slog.Int("output_tokens", int(usage.CandidatesTokenCount)),
			slog.Int("total_tokens", int(usage.TotalTokenCount)))
	}

	// Log full raw response for debugging (includes thoughts, etc.)
	if raw, err := resp.MarshalJSON(); err == nil {
		slog.Debug("LLM Raw Response", "json", string(raw))
	}

	respStr := resp.Text()
	if err := json.Unmarshal([]byte(respStr), target); err != nil {
		return usage, fmt.Errorf("failed to parse JSON: %w\nResponse: %s", err, respStr)
	}

	return usage, nil
}
