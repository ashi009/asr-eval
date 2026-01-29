package evalv2

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

type Evaluator struct {
	client *genai.Client
	model  string
}

func NewEvaluator(client *genai.Client, model string) *Evaluator {
	return &Evaluator{
		client: client,
		model:  model,
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, contextData *ContextResponse, transcripts map[string]string) (*EvaluationResponse, error) {
	// Serialize context to JSONStr
	contextBytes, _ := json.MarshalIndent(contextData, "", "  ")
	contextStr := string(contextBytes)

	transcriptText := ""
	for k, v := range transcripts {
		transcriptText += fmt.Sprintf("Provider [%s]: %s\n", k, v)
	}

	prompt := fmt.Sprintf(`You are an ASR Evaluation Engine.

Goals:
1. **Business Scoring ($S$)**: Evaluate transcripts based on the "Checkpoints" in the Context.
   - For each checkpoint (S1, S2...), determine if the transcript successfully captures it (Pass/Fail).
   - If Failed, provide a brief "reason" (e.g., "Misheard 'forty' as 'four'").
   - $S$ Score = Weighted sum of passed checkpoints / Total weight.

其中 CheckpointResult 的 Status 取值逻辑：

* **Pass**: 1.0
* **Fail**: 0.0
* **Partial** (仅限 Tier 2/3): 0.5 （Tier 1 严禁使用 Partial）

2. **Acoustic Scoring ($P$)**: Evaluate phonetic fidelity ($P = 1 - \text{PER}$).
   - Compare transcript vs "audio_reality_inference" in Context.
   - Calculate PER (Word Error Rate concept but for Phonetic match).
3. **UI Diff Support**: Generate "revised_transcript".
   - REPLACE words that triggered Tier 1/2 Fail with correct words from Context.
   - KEEP fillers/stutters if they don't break checkpoints.
4. **Summary**: Provide a list of at most 3 SHORT strings explaining the major factors for the score (e.g. "Missed Critical Entity S2", "High Phonetic Accuracy").

Context for Evaluation:
%s

Transcripts to Evaluate:
%s

Evaluate all transcripts one by one.
`, contextStr, transcriptText)

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(prompt),
			},
		},
	}

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   GetEvaluationResponseSchema(),
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingLevel:   genai.ThinkingLevelLow,
		},
	}

	resp, err := e.client.Models.GenerateContent(ctx, e.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if resp.UsageMetadata != nil {
		fmt.Printf("Token usage: Prompt=%d, Thoughts=%d, Output=%d, Total=%d\n",
			resp.UsageMetadata.PromptTokenCount,
			resp.UsageMetadata.ThoughtsTokenCount,
			resp.UsageMetadata.CandidatesTokenCount,
			resp.UsageMetadata.TotalTokenCount)
	}

	b, _ := resp.MarshalJSON()
	fmt.Printf("RESP: %s\n", b)

	respStr := resp.Text()

	var resultLLM EvaluationResponseLLM
	if err := json.Unmarshal([]byte(respStr), &resultLLM); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nResponse: %s", err, respStr)
	}

	// Convert back to Map-based EvaluationResponse
	finalResult := &EvaluationResponse{
		Evaluations: make(map[string]ModelEvaluation),
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

		finalResult.Evaluations[item.Provider] = ModelEvaluation{
			Transcript:        transcripts[item.Provider],
			RevisedTranscript: item.RevisedTranscript,
			Metrics:           item.Metrics,
			CheckpointResults: cpResults,
			Summary:           item.Summary,
		}
	}

	return finalResult, nil
}
