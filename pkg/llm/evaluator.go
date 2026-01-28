package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type EvalResult struct {
	Score              float64  `json:"score"`
	RevisedTranscript  string   `json:"revised_transcript"`
	Summary            []string `json:"summary"`             // Max 3 points
	OriginalTranscript string   `json:"original_transcript"` // The transcript being evaluated
}

type Evaluator struct {
	client LLMClient
}

func NewEvaluator(client LLMClient) *Evaluator {
	return &Evaluator{
		client: client,
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

	content, err := e.client.Generate(ctx, TextPrompt(prompt))
	if err != nil {
		return nil, fmt.Errorf("llm generation failed: %w", err)
	}

	// Clean up markdown code blocks if present to ensure valid JSON
	content = cleanJSONMarkdown(content)

	var partialResults map[string]struct {
		Score             float64  `json:"score"`
		RevisedTranscript string   `json:"revised_transcript"`
		Summary           []string `json:"summary"`
	}
	err = json.Unmarshal([]byte(content), &partialResults)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON from LLM: %v. Content: %s", err, content)
	}

	// Transform to final EvalResult with OriginalTranscript
	results := make(map[string]EvalResult)
	for k, v := range partialResults {
		results[k] = EvalResult{
			Score:              v.Score,
			RevisedTranscript:  v.RevisedTranscript,
			Summary:            v.Summary,
			OriginalTranscript: transcripts[k],
		}
	}

	return results, nil
}

func cleanJSONMarkdown(content string) string {
	content = strings.TrimSpace(content)
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
			return content[start:end]
		}
	}
	return content
}
