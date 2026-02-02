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
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	ext := filepath.Ext(audioPath)
	m := mime.TypeByExtension(ext)
	if m == "" {
		m = "audio/flac" // Default to flac as per dataset
	}

	// 2. Prepare Text Prompt
	p, err := buildGenerateContextPrompt(generateContextPromptData{
		GroundTruth: groundTruth,
		Transcripts: transcripts,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build context prompt: %w", err)
	}

	// 3. Call LLM
	req := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(p),
				genai.NewPartFromBytes(data, m),
			},
		},
	}

	cfg := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
		},
	}

	var resp EvalContext
	usage, err := e.generateJSON(ctx, e.genModel, req, cfg, &resp)
	if err != nil {
		return nil, usage, err
	}

	// 5. Post-process: Inject Ground Truth and Normalize Weights
	resp.Meta.GroundTruth = groundTruth

	sum := 0.0
	for _, cp := range resp.Checkpoints {
		sum += cp.Weight
	}
	if sum > 0 {
		for i := range resp.Checkpoints {
			resp.Checkpoints[i].Weight /= sum
		}
	}

	return &resp, usage, nil
}

func (e *Evaluator) Evaluate(ctx context.Context, contextData *EvalContext, transcripts map[string]string) (*EvalReport, *genai.GenerateContentResponseUsageMetadata, error) {
	p, err := buildEvaluatePrompt(evaluatePromptData{
		EvalContext: contextData,
		Transcripts: transcripts,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build eval prompt: %w", err)
	}

	req := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(p),
			},
		},
	}

	cfg := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevelLow,
		},
	}

	// llmCheckpointResult is the raw result from LLM (unexported)
	type llmCheckpointResult struct {
		ID       string           `json:"id"`
		Status   CheckpointStatus `json:"status" jsonscheme:"enum:Pass,Fail,Partial"`
		Detected string           `json:"detected"`         // text segment identified
		Reason   string           `json:"reason,omitempty"` // Reason for failure
	}

	// llmEvalResult is the raw result from LLM for a transcript (unexported)
	type llmEvalResult struct {
		Provider          string                `json:"provider"`
		RevisedTranscript string                `json:"revised_transcript"`
		Metrics           EvalMetrics           `json:"metrics"`
		CheckpointResults []llmCheckpointResult `json:"checkpoint_results"`
		Summary           []string              `json:"summary"`
	}

	// llmEvalReport is the raw array from LLM (unexported)
	type llmEvalReport []llmEvalResult

	var raw llmEvalReport
	usage, err := e.generateJSON(ctx, e.evalModel, req, cfg, &raw)
	if err != nil {
		return nil, usage, err
	}

	// Convert back to Map-based EvaluationResponse -> EvalReport
	resp := &EvalReport{
		Results: make(map[string]EvalResult),
	}

	for _, item := range raw {
		cps := make(map[string]CheckpointResult)
		for _, cp := range item.CheckpointResults {
			cps[cp.ID] = CheckpointResult{
				Status:   cp.Status,
				Detected: cp.Detected,
				Reason:   cp.Reason,
			}
		}

		resp.Results[item.Provider] = EvalResult{
			Transcript:        transcripts[item.Provider],
			RevisedTranscript: item.RevisedTranscript,
			Metrics:           item.Metrics,
			CheckpointResults: cps,
			Summary:           item.Summary,
		}
	}

	return resp, usage, nil
}

func (e *Evaluator) generateJSON(ctx context.Context, model string, req []*genai.Content, cfg *genai.GenerateContentConfig, resp interface{}) (*genai.GenerateContentResponseUsageMetadata, error) {
	// Automatically generate schema and set JSON response type
	cfg.ResponseMIMEType = "application/json"
	cfg.ResponseSchema = reflectSchema(reflect.TypeOf(resp))

	// Log the prompt for debugging
	for i, content := range req {
		for j, part := range content.Parts {
			if part.Text != "" {
				slog.Debug("LLM Prompt", "content_index", i, "part_index", j, "text", part.Text)
			}
		}
	}

	r, err := e.client.Models.GenerateContent(ctx, model, req, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	usage := r.UsageMetadata
	if usage != nil {
		slog.Info("LLM Usage",
			slog.Int("prompt_tokens", int(usage.PromptTokenCount)),
			slog.Int("thought_tokens", int(usage.ThoughtsTokenCount)),
			slog.Int("output_tokens", int(usage.CandidatesTokenCount)),
			slog.Int("total_tokens", int(usage.TotalTokenCount)))
	}

	// Log full raw response for debugging (includes thoughts, etc.)
	if raw, err := r.MarshalJSON(); err == nil {
		slog.Debug("LLM Raw Response", "json", string(raw))
	}

	respStr := r.Text()
	if err := json.Unmarshal([]byte(respStr), resp); err != nil {
		return usage, fmt.Errorf("failed to parse JSON: %w\nResponse: %s", err, respStr)
	}

	return usage, nil
}
