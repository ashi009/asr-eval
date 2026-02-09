package evalv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

func toJson(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func prefix(prefix, s string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

var funcMap = template.FuncMap{
	"json":   toJson,
	"prefix": prefix,
}

// Prompt template for GenerateContext
var generateContextPromptTemplate = template.Must(template.New("genContext").Funcs(funcMap).Parse(`
You are an expert ASR Data Analyst and expert phonetician specializing in forensic linguistics and dialectology.

Analyze the provided Audio, Ground Truth (GT), and Candidate Transcripts to create a "Context for Evaluation".

### Ground Truth

It may contain annotations for the previous word/phrase in the format of "(annotation)" or "<annotation>".

{{.GroundTruth | prefix "> "}}

### Candidate Transcripts

{{range $k, $v := .Transcripts}}- {{$k}}
{{$v | prefix "  >"}}
{{end}}

### Task

1. Define the **Business Goal**: concise summary of the user's intent based **STRICTLY AND SOLELY** on the provided Ground Truth (GT) text.
   - Do **NOT** use information from the Audio or Transcripts for this field.
   - Even if the GT seems incomplete or contradicts the Audio, you **MUST** describe the intent derived **only** from the GT text.
   - This goal serves as the "Intended Request" baseline.
1. **Audio Reality Inference**:
   - Reconstruct the full audio text including fillers, stutters, and hesitation sounds that might be missing in GT but present in Audio. This is the "Phonetic Truth".
   - Non-GT transcripts may contain phonetic simliar gibberish due to code-change, don't use them verbatim, but infer the correct words.
1. Define **Checkpoints** (Hierarchical):
   - Analyze the GT and Business intent.
   - Break down the GT into segments, ensuring **EVERY** sentence and phrase in the GT is covered by at least one checkpoint.
   - **Complete Coverage Policy**: Do not skip parts of the GT. If a sentence is trivial, assign it to Tier 3 with very low weight (e.g., 0.05), but it MUST include it.
   - **Strict Ordering Policy**: Checkpoints MUST follow the exact order of appearance in the GT. Text segments must NOT be rearranged.
   - **Verbatim Policy**: The text segment MUST be an exact verbatim substring from the GT. Do not paraphrase.
   - **Rationale Policy**: The rationale MUST be concise and clear about the criterion for giving the final score. It should explain why this checkpoint is important and what constitutes a pass.
   - Provide a unique ID (S1, S2...), the starting timestamp in ms, the text segment, tier (1,2,3), weight (0.0-1.0), and rationale.
1. **Questionable GT?**:
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

1.  **Tier 1 (60%-70%)**: Critical items get the biggest slice. Single item: 0.20-0.30.
2.  **Tier 2 (20%-30%)**: Fill the gap. Single item: 0.10-0.15.
3.  **Tier 3 (~10%)**: Minor items. Single item: ~0.05.
`))

// Prompt template for Evaluate
var evaluatePromptTemplate = template.Must(template.New("evaluate").Funcs(funcMap).Parse(`
You are an ASR Evaluation Engine.

Goals:
1. **Business Scoring ($S$)**: Evaluate transcripts based on the "Checkpoints" in the Context.
   - For each checkpoint (S1, S2...), determine if the transcript successfully captures it (Pass/Fail).
   - If Failed, provide a brief "reason" (e.g., "Misheard 'forty' as 'four'").
   - $S$ Score = Weighted sum of passed checkpoints / Total weight.

2. **CheckpointResult Status Logic**:
   - **Pass**: 1.0
   - **Fail**: 0.0
   - **Partial** (Only for Tier 2/3): 0.5 (Tier 1 is strictly forbidden to use Partial)

3. **Acoustic
   - Compare transcript vs "audio_reality_inference" in Context.
   - Calculate PER (Phonetic Error Rate).
4. **UI Diff Support**: Generate "revised_transcript".
   - REPLACE words that triggered Tier 1/2 Fail with correct words from Context.
   - KEEP fillers/stutters if they don't break checkpoints.
5. **Summary**: Provide a list of at most 3 SHORT strings explaining the major factors for the score
   - for $S$ include Checkpoint IDs, eg. "Missed Critical Entity S2/S4"
   - for $P$, eg. "High Phonetic Accuracy"

Context for Evaluation:
{{.EvalContext | json}}

Transcripts to Evaluate:
{{range $k, $v := .Transcripts}}Provider [{{$k}}]: {{$v}}
{{end}}

Evaluate all transcripts one by one.
`))

// generateContextPromptData holds data for the context generation prompt
type generateContextPromptData struct {
	GroundTruth string
	Transcripts map[string]string
}

// buildGenerateContextPrompt constructs the prompt string for context generation
func buildGenerateContextPrompt(d generateContextPromptData) (string, error) {
	var buf bytes.Buffer
	if err := generateContextPromptTemplate.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("failed to execute genContext template: %w", err)
	}
	return buf.String(), nil
}

// evaluatePromptData holds data for the evaluation prompt
type evaluatePromptData struct {
	EvalContext *EvalContext
	Transcripts map[string]string
}

// buildEvaluatePrompt constructs the prompt string for evaluation
func buildEvaluatePrompt(d evaluatePromptData) (string, error) {
	var buf bytes.Buffer
	if err := evaluatePromptTemplate.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("failed to execute evaluate template: %w", err)
	}
	return buf.String(), nil
}

// Prompt template for Evaluate V2 (Programmatic Scoring)
var evaluatePromptTemplateV2 = template.Must(template.New("evaluateV2").Funcs(funcMap).Parse(`
You are an expert ASR Evaluation Engine.

Your goal is to compare Candidate Transcripts against the provided "Context for Evaluation".

You will NOT calculate scores directly. Instead, you will provide structural analysis data for the evaluation program to process.

Task:
1. **Analyze Checkpoints**:
   - For each checkpoint (S1, S2...) defined in the Context, determine if the transcript successfully captures it.
   - Status: "Pass", "Fail", or "Partial" (Partial is ONLY for Tier 2/3).
   - detected: The actual text segment found in the transcript that matches the checkpoint.
   - reason: If not Pass, explain why.

2. **Analyze Phonetic Accuracy (PER)**:
   - Compare the transcript against the "audio_reality_inference" (Phonetic Truth) from the Context.
   - Identify specific error chunks:
     - **Insertions**: Extra words/sounds in transcript not in valid audio reality.
     - **Deletions**: Words/sounds in audio reality missing from transcript.
     - **Substitutions**: Words misheard/replaced.
   - Provide these as lists of strings (exact text chunks).

3. **Generate Revised Transcript**:
   - Create a version of the transcript that fixes Tier 1/2 failures using the correct words from Context.
   - Retain verified fillers/stutters.

4. **Summary**:
   - Provide 3 short bullet points summarizing the key findings (e.g., "Missed S2 (Critical)", "High Phonetic Fidelity").

Context for Evaluation:
{{.EvalContext | json}}

Transcripts to Evaluate:
{{range $k, $v := .Transcripts}}Provider [{{$k}}]: {{$v}}
{{end}}

Evaluate all transcripts one by one. Output JSON matching the LLMEvalResult structure.
`))

// buildEvaluatePromptV2 constructs the prompt string for evaluation V2
func buildEvaluatePromptV2(d evaluatePromptData) (string, error) {
	var buf bytes.Buffer
	if err := evaluatePromptTemplateV2.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("failed to execute evaluateV2 template: %w", err)
	}
	return buf.String(), nil
}
