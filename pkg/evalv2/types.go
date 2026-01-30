package evalv2

import (
	"google.golang.org/genai"
)

// Checkpoint represents a hierarchical evaluation point
type Checkpoint struct {
	ID          string  `json:"id"`
	TextSegment string  `json:"text_segment"`
	Tier        int     `json:"tier"`
	Weight      float64 `json:"weight"`
	Rationale   string  `json:"rationale"`
}

// MetaInfo contains metadata for the context
type MetaInfo struct {
	BusinessGoal            string `json:"business_goal"`
	AudioRealityInference   string `json:"audio_reality_inference"`
	TotalTokenCountEstimate int    `json:"total_token_count_estimate"`
	GroundTruth             string `json:"ground_truth"` // LLM Intermediate Structs for Generation (Arrays instead of Maps)
	QuestionableGT          bool   `json:"questionable_gt"`
	QuestionableReason      string `json:"questionable_reason"`
}

type CheckpointResultLLM struct {
	ID       string `json:"id"`
	Status   string `json:"status"`           // "pass", "fail", or "partial"
	Detected string `json:"detected"`         // text segment identified
	Reason   string `json:"reason,omitempty"` // Reason for failure
}

// PERDetails holds details for Phoneme Error Rate
type PERDetails struct {
	Sub int `json:"sub"`
	Del int `json:"del"`
	Ins int `json:"ins"`
}

// Metrics holds various evaluation metrics
type Metrics struct {
	SScore     float64    `json:"S_score"`
	PScore     float64    `json:"P_score"`
	PERDetails PERDetails `json:"PER_details"`
}

type ModelEvaluationLLM struct {
	Provider          string                `json:"provider"`
	RevisedTranscript string                `json:"revised_transcript"`
	Metrics           Metrics               `json:"metrics"`
	CheckpointResults []CheckpointResultLLM `json:"checkpoint_results"`
	Summary           []string              `json:"summary"`
}

type EvaluationResponseLLM []ModelEvaluationLLM

// ContextResponse represents the output of Step 1 ([id].gt.v2.json)
type ContextResponse struct {
	Meta        MetaInfo     `json:"meta"`
	Checkpoints []Checkpoint `json:"checkpoints"`
}

// ModelEvaluation represents the evaluation result for a single model (Map based)
type ModelEvaluation struct {
	Transcript        string                      `json:"transcript"`
	RevisedTranscript string                      `json:"revised_transcript"`
	Metrics           Metrics                     `json:"metrics"`
	CheckpointResults map[string]CheckpointResult `json:"checkpoint_results"`
	Summary           []string                    `json:"summary"`
}

// EvaluationResponse represents the output of Step 2 ([id].report.v2.json)
type EvaluationResponse struct {
	GroundTruth     string                     `json:"ground_truth"`
	Evaluations     map[string]ModelEvaluation `json:"evaluations"`
	ContextHash     string                     `json:"context_hash,omitempty"`
	ContextSnapshot ContextResponse            `json:"context_snapshot,omitempty"`
}

// CheckpointResult holds the pass/fail status for a single checkpoint
type CheckpointResult struct {
	Status   string `json:"status"`           // "pass", "fail", or "partial"
	Detected string `json:"detected"`         // text segment identified
	Reason   string `json:"reason,omitempty"` // Reason for failure
}

// ... (other types unchanged)

// Helper to define Schema for ContextResponse
func GetContextResponseSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"meta": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"business_goal":              {Type: genai.TypeString},
					"audio_reality_inference":    {Type: genai.TypeString},
					"total_token_count_estimate": {Type: genai.TypeInteger},
					"questionable_gt":            {Type: genai.TypeBoolean},
					"questionable_reason":        {Type: genai.TypeString},
				},
				Required: []string{"business_goal", "audio_reality_inference", "total_token_count_estimate", "questionable_gt"},
			},
			"checkpoints": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id":           {Type: genai.TypeString},
						"text_segment": {Type: genai.TypeString},
						"tier":         {Type: genai.TypeInteger},
						"weight":       {Type: genai.TypeNumber},
						"rationale":    {Type: genai.TypeString},
					},
					Required: []string{"id", "text_segment", "tier", "rationale"},
				},
			},
		},
		Required: []string{"meta", "checkpoints"},
	}
}

// Helper to define Schema for EvaluationResponse (LLM Array Version)
func GetEvaluationResponseSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"provider":           {Type: genai.TypeString},
				"revised_transcript": {Type: genai.TypeString},
				"metrics": {
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"S_score": {Type: genai.TypeNumber},
						"P_score": {Type: genai.TypeNumber},
						"PER_details": {
							Type: genai.TypeObject,
							Properties: map[string]*genai.Schema{
								"sub": {Type: genai.TypeInteger},
								"del": {Type: genai.TypeInteger},
								"ins": {Type: genai.TypeInteger},
							},
							Required: []string{"sub", "del", "ins"},
						},
					},
					Required: []string{"S_score", "P_score", "PER_details"},
				},
				"checkpoint_results": {
					Type: genai.TypeArray, // Array of CheckpointResultLLM
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"id":       {Type: genai.TypeString},
							"status":   {Type: genai.TypeString, Enum: []string{"Pass", "Fail", "Partial"}},
							"detected": {Type: genai.TypeString},
							"reason":   {Type: genai.TypeString},
						},
						Required: []string{"id", "status", "detected"},
					},
				},
				"summary": {
					Type:  genai.TypeArray,
					Items: &genai.Schema{Type: genai.TypeString},
				},
			},
			Required: []string{"provider", "revised_transcript", "metrics", "checkpoint_results", "summary"},
		},
	}
}
