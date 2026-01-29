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

type Generator struct {
	client *genai.Client
	model  string
}

func NewGenerator(client *genai.Client, model string) *Generator {
	return &Generator{
		client: client,
		model:  model,
	}
}

func (g *Generator) GenerateContext(ctx context.Context, audioPath string, groundTruth string, transcripts map[string]string) (*ContextResponse, error) {
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
Analyze the provided Audio, Ground Truth (GT), and Candidate Transcripts to create a "Context for Evaluation". GT may contain annotations for the previous word/phrase with "(annotation)" or "<annotation>".

Ground Truth: %s

Candidate Transcripts:
%s

Task:
1. Define the **Business Goal**: concise summary of the user's intent based solely on the GT.
2. key **Audio Reality Inference**: Reconstruct the full audio text including fillers, stutters, and hesitation sounds that might be missing in GT but present in Audio. This is the "Phonetic Truth". Non-GT transcripts may contain phonetic simliar gibberish due to code-change, don't use them verbatim, but infer the correct words.
3. Estimate **Total Token Count**: approximate count of tokens in the audio reality.
4. Define **Checkpoints** (Hierarchical):
   - Analyze the GT and Business intent.
   - Break down the GT into critical segments.
   - Provide a unique ID (S1, S2...), the text segment, tier (1,2,3), weight (0.0-1.0), and rationale.

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

	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if resp.UsageMetadata != nil {
		fmt.Printf("Token usage: Prompt=%d, Toughts=%d, Output=%d, Total=%d\n",
			resp.UsageMetadata.PromptTokenCount,
			resp.UsageMetadata.ThoughtsTokenCount,
			resp.UsageMetadata.CandidatesTokenCount,
			resp.UsageMetadata.TotalTokenCount)
	}

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
