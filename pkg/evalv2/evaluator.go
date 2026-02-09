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

func (e *Evaluator) EvaluateV2(ctx context.Context, contextData *EvalContext, transcripts map[string]string) (*EvalReport2, *genai.GenerateContentResponseUsageMetadata, error) {
	// Use V2 Prompt
	p, err := buildEvaluatePromptV2(evaluatePromptData{
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

	// 1. Define Intermediate Structs for LLM (must use Slices, not Maps)
	type llmCheckpointResultV2 struct {
		ID       string           `json:"id"`
		Status   CheckpointStatus `json:"status" jsonscheme:"enum:Pass,Fail,Partial"`
		Detected string           `json:"detected"`         // text segment identified
		Reason   string           `json:"reason,omitempty"` // Reason for failure
	}

	type llmEvalResultV2 struct {
		Provider          string                  `json:"provider"`
		RevisedTranscript string                  `json:"revised_transcript"`
		CheckpointResults []llmCheckpointResultV2 `json:"checkpoint_results"`
		PhoneticAnalysis  PhoneticAnalysis        `json:"phonetic_analysis"`
		Summary           []string                `json:"summary"`
	}

	type llmEvalReportV2 []llmEvalResultV2

	var raw llmEvalReportV2
	usage, err := e.generateJSON(ctx, e.evalModel, req, cfg, &raw)
	if err != nil {
		return nil, usage, err
	}

	// 2. Convert to Final Report (converting Slice -> Map)
	resp := &EvalReport2{
		Results: make(map[string]EvalResult2),
	}

	for _, item := range raw {
		// Convert Checkpoints Slice -> Map
		cps := make(map[string]CheckpointResult)
		for _, cp := range item.CheckpointResults {
			cps[cp.ID] = CheckpointResult{
				Status:   cp.Status,
				Detected: cp.Detected,
				Reason:   cp.Reason,
			}
		}

		// Create EvalResult2
		// Note: Metrics will be populated after calculation
		resultV2 := EvalResult2{
			Transcript:        transcripts[item.Provider],
			RevisedTranscript: item.RevisedTranscript,
			CheckpointResults: cps,
			PhoneticAnalysis:  item.PhoneticAnalysis,
			Summary:           item.Summary,
		}

		// Calculate Metrics in Go using the constructed ResultV2
		metrics := e.calculateMetrics(&resultV2, contextData)
		resultV2.Metrics = metrics

		resp.Results[item.Provider] = resultV2
	}

	return resp, usage, nil
}

func (e *Evaluator) calculateMetrics(item *EvalResult2, ctx *EvalContext) EvalMetrics {
	// 1. Calculate S-Score
	passedWeight := 0.0
	totalWeight := 0.0

	// Create a map for fast lookup of result status
	resMap := item.CheckpointResults

	for _, cp := range ctx.Checkpoints {
		totalWeight += cp.Weight
		if res, ok := resMap[cp.ID]; ok {
			switch res.Status {
			case StatusPass:
				passedWeight += cp.Weight
			case StatusPartial:
				// Only for Tier 2/3 (enforced by LLM prompt usually, but good to check)
				passedWeight += cp.Weight * 0.5
			case StatusFail:
				passedWeight += 0.0
			}
		}
	}

	sScore := 0.0
	if totalWeight > 0 {
		sScore = passedWeight / totalWeight
	}

	// 2. Calculate P-Score (PER)
	// Reference is the "Audio Reality Inference"
	// We'll estimate N (number of words) by splitting by space roughly
	// For production, a better tokenizer might be needed, but space split is standard for WER/PER approx.
	// Actually, context.Meta.AudioRealityInference might be CJK, so simple space split isn't enough.
	// However, for this refactor, we assume space-separated words or characters depending on language.
	// Let's use a simple tokenizer: strings.Fields
	// Let's use a simple tokenizer: strings.Fields
	// If it's English, fields is better. If CJK, rune count.
	// Since we don't have a language detector here easily, and the prompt asks for "Audio Reality Inference",
	// let's assume we count *characters* for P-Score denominator if it looks like CJK, or *words* if English.
	// For simplicity in this V2 step, let's just use a naive token count provided by the prompt data if available,
	// OR just use rune count for now as a baseline for robustness.
	// WAIT: content.Meta.TotalTokenCountEstimate is available! Use that?
	// It says "approximated count of tokens". Let's use that as the denominator N.
	N := float64(ctx.Meta.TotalTokenCountEstimate)
	if N <= 0 {
		N = 1.0 // Prevent division by zero
	}

	ins := len(item.PhoneticAnalysis.Insertions)
	del := len(item.PhoneticAnalysis.Deletions)
	sub := len(item.PhoneticAnalysis.Substitutions)

	per := float64(ins+del+sub) / N
	pScore := 1.0 - per
	if pScore < 0 {
		pScore = 0
	}

	return EvalMetrics{
		SScore: sScore,
		PScore: pScore,
		// QScore is calculated on the fly by the struct method
		PhoneticDetails: PhoneticDetails{
			Ins: ins,
			Del: del,
			Sub: sub,
		},
	}
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
