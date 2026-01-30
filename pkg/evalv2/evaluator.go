package evalv2

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

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

func (e *Evaluator) GenerateContext(ctx context.Context, audioPath string, groundTruth string, transcripts map[string]string) (*ContextResponse, error) {
	// 1. Prepare Audio Part
	audioData, err := ioutil.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	ext := filepath.Ext(audioPath)
	mimeType := "audio/flac" // Default to flac as per dataset
	if ext == ".mp3" {
		mimeType = "audio/mp3"
	} else if ext == ".wav" {
		mimeType = "audio/wav"
	}

	// 2. Prepare Text Prompt
	transcriptText := ""
	for k, v := range transcripts {
		transcriptText += fmt.Sprintf("Provider [%s]: %s\n", k, v)
	}

	prompt := fmt.Sprintf(`You are an expert ASR Data Analyst.
Analyze the provided Audio, Ground Truth (GT), and Candidate Transcripts to create a "Context for Evaluation". GT may contain annotations for the previous word/phrase in the format of "(annotation)" or "<annotation>".

Ground Truth: %s

Candidate Transcripts:
%s

Task:
1. Define the **Business Goal**: concise summary of the user's intent based **STRICTLY AND SOLELY** on the provided Ground Truth (GT) text.
   - Do **NOT** use information from the Audio or Transcripts for this field.
   - Even if the GT seems incomplete or contradicts the Audio, you **MUST** describe the intent derived **only** from the GT text.
   - This goal serves as the "Intended Request" baseline.
2. key **Audio Reality Inference**: Reconstruct the full audio text including fillers, stutters, and hesitation sounds that might be missing in GT but present in Audio. This is the "Phonetic Truth". Non-GT transcripts may contain phonetic simliar gibberish due to code-change, don't use them verbatim, but infer the correct words.
3. Estimate **Total Token Count**: approximate count of tokens in the audio reality.
4. Define **Checkpoints** (Hierarchical):
   - Analyze the GT and Business intent.
   - Break down the GT into segments, ensuring **EVERY** sentence and phrase in the GT is covered by at least one checkpoint.
   - **Complete Coverage Policy**: Do not skip parts of the GT. If a sentence is trivial, assign it to Tier 3 with very low weight (e.g., 0.05), but it MUST include it.
   - Provide a unique ID (S1, S2...), the text segment, tier (1,2,3), weight (0.0-1.0), and rationale.
5. **Questionable GT?**:
   - Do you think the provided Ground Truth is questionable (e.g., contains obvious typos, missing words, or is completely wrong compared to the Audio/Audio Reality)?
   - If yes, set "questionable_gt" to true and provide a reason in "questionable_reason".


### Guidance for Tiers and Weights

**1. How to Define Tier? (Qualitative)**
Ask: "If this information is wrong, will the business goal fail?"

* **Tier 1: Core Resolution (Critical) — Wrong = Fatal**
  - **Criteria**: Failure here means business failure, money loss, or complaints.
  - **Keywords**: Amounts, Quantities, Core Item Names, Action Commands (Refund/Urgent), Negations (Do NOT want).
  - **Policy**: **Zero Tolerance**. No partial credit.

* **Tier 2: Context Anchors (Important) — Wrong = Confusing**
  - **Criteria**: Wrong info causes hesitation or extra verification time, but business might still succeed.
  - **Keywords**: Package names, Names, Time references (yesterdays), Adjectives.
  - **Policy**: **Tolerance Allowed**. Homophones accepted if no ambiguity.

* **Tier 3: Interaction (Low) — Wrong = Rough**
  - **Criteria**: Politeness or fillers. Wrong affects experience, not result.
  - **Keywords**: Hello, Thanks, Umm, Ah.
  - **Policy**: **Low Attention**.

**2. How to Assign Weight? (Quantitative)**
Follow a **Dynamic Balance** where Total Weight = 1.0.

1.  **Tier 1 (60%%-70%%)**: Critical items get the biggest slice. Single item: 0.20-0.30.
2.  **Tier 2 (20%%-30%%)**: Fill the gap. Single item: 0.10-0.15.
3.  **Tier 3 (~10%%)**: Minor items. Single item: ~0.05.
`, groundTruth, transcriptText)

	log.Println(prompt)

	// 3. Call LLM
	// Construct Content with Parts (Text + Blob)
	// Using generic generation, assuming single content block for prompt
	// But SDK expects []*genai.Content.
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(prompt),
				genai.NewPartFromBytes(audioData, mimeType),
			},
		},
	}

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   GetContextResponseSchema(),
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
		},
	}

	resp, err := e.client.Models.GenerateContent(ctx, e.genModel, contents, config)
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
	fmt.Printf("%s\n", b)

	respStr := resp.Text()
	var result ContextResponse
	if err := json.Unmarshal([]byte(respStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nResponse: %s", err, respStr)
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

	return &result, nil
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

41: 其中 CheckpointResult 的 Status 取值逻辑：

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

	resp, err := e.client.Models.GenerateContent(ctx, e.evalModel, contents, config)
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
