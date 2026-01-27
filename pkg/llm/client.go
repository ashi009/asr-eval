package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

type EvalResult struct {
	Score             float64  `json:"score"`
	RevisedTranscript string   `json:"revised_transcript"`
	Summary           []string `json:"summary"` // Max 3 points
}

type Evaluator struct {
	client *arkruntime.Client
	model  string
}

func NewEvaluator() *Evaluator {
	apiKey := os.Getenv("ARK_API_KEY")
	if apiKey == "" {
		// Log warning
	}
	client := arkruntime.NewClientWithApiKey(
		apiKey,
		// arkruntime.WithBaseUrl("https://ark.cn-beijing.volces.com/api/v3"), // Removed as it causes 400 error
	)
	return &Evaluator{
		client: client,
		model:  "doubao-seed-1-8-251228",
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, groundTruth string, transcripts map[string]string) (map[string]EvalResult, error) {
	prompt := fmt.Sprintf(`You are an expert ASR (Automatic Speech Recognition) evaluator.
Compare the "transcript" against the provided "ground_truth".

Evaluation Rules:
1. Annotation Support:
   - Ground truth may contain annotations in the format "text(annotation)".
   - Example: "那个43(forty-three)" means the audio said "forty-three" but was transcribed as "43".
   - Treat the annotation as an acceptable alternative or clarifying pronunciation. If the transcript matches the annotation OR the text before it, it is correct.

2. Homophone Tolerance:
   - Be tolerant of homophones (same Pinyin) if the error is common or semantically understandable.
   - Example: "反应" vs "反映". If the usage suggests "反映" but "反应" is recognized, treat it as a minor or non-issue depending on clarity.
   - Do NOT penalize strictly for common homophone errors unless they drastically change meaning or are rare.

3. Ignore Filler Words:
   - Ignore conversational filler words and tone particles in the transcript if they don't affect meaning.
   - Examples to ignore: "诶", "唉", "嗯", "呃", "啊".
   - Their presence or absence should not lower the score.

Task:
1. Score [0.0, 1.0]: Rate the quality based on meaning match.
   - USE HIGH RESOLUTION (e.g. 0.85, 0.87, 0.92, 0.999). Refrain from using round numbers like 0.8 or 0.9 unless necessary.
   - High score: Key info matches ground truth (considering annotations/homophones), no hallucinations, no missing key info.
   - Low score: Meaning deviation, hallucinations, or lost context.
   - Differentiate carefully between minor errors (0.95) and perfect matches (1.0).
2. Revised Transcript: Rewrite the transcript to exactly match the ground truth's meaning and phrasing where valid.
   - Fix obvious ASR errors.
   - Apply the annotation logic (resolve to the standard form).
   - Do NOT just copy the ground truth if the transcript is completely unrelated (in that case score is 0).
   - The goal is to show what the transcript *should* have been if it were perfect.
3. Summary: List at most 3 bullet points explaining the score (e.g. "Missed entity 'Project X'", "Hallucinated polite phrases", "Accepted homophone 'foo'").

Output JSON ONLY:
{
  "provider_name": {
    "score": <float>,
    "revised_transcript": "<string>",
    "summary": ["<point1>", "<point2>", "<point3>"]
  },
  ...
}

Ground Truth: "%s"

Transcripts:
%s

Return JSON map matching the input providers. No markdown.`, groundTruth, formatTranscripts(transcripts))

	req := &responses.ResponsesRequest{
		Model: e.model,
		Input: &responses.ResponsesInput{
			Union: &responses.ResponsesInput_ListValue{
				ListValue: &responses.InputItemList{ListValue: []*responses.InputItem{{
					Union: &responses.InputItem_InputMessage{
						InputMessage: &responses.ItemInputMessage{
							Role: responses.MessageRole_user,
							Content: []*responses.ContentItem{
								{
									Union: &responses.ContentItem_Text{
										Text: &responses.ContentItemText{
											Type: responses.ContentItemType_input_text,
											Text: prompt,
										},
									},
								},
							},
						},
					},
				}}},
			},
		},
	}

	resp, err := e.client.CreateResponses(ctx, req, arkruntime.WithProjectName("eval-transcript"))
	if err != nil {
		return nil, fmt.Errorf("ark API error: %w", err)
	}

	// Debug logging
	respBytes, _ := json.Marshal(resp)
	fmt.Printf("DEBUG: Raw Ark Response: %s\n", string(respBytes))

	if len(resp.Output) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	// Find message content in outputs
	var content string
	for _, item := range resp.Output {
		if msg := item.GetOutputMessage(); msg != nil && len(msg.Content) > 0 {
			if textContent := msg.Content[0].GetText(); textContent != nil {
				content = textContent.Text
				break
			}
		}
	}

	if content == "" {
		return nil, fmt.Errorf("no text content found in model response")
	}

	// Clean up markdown code blocks if present to ensure valid JSON
	if len(content) > 3 && content[:3] == "```" {
		start := 0
		end := len(content)
		for i, c := range content {
			if c == '{' {
				start = i
				break
			}
		}
		for i := len(content) - 1; i >= 0; i-- {
			if c := content[i]; c == '}' {
				end = i + 1
				break
			}
		}
		if start < end {
			content = content[start:end]
		}
	}

	var results map[string]EvalResult
	err = json.Unmarshal([]byte(content), &results)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON from LLM: %v. Content: %s", err, content)
	}

	return results, nil
}
