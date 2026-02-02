package evalv2

// EvalContext represents the output of Step 1 ([id].gt.v2.json)
type EvalContext struct {
	Meta        ContextMeta  `json:"meta"`
	Checkpoints []Checkpoint `json:"checkpoints"`
}

// EvalReport represents the output of Step 2 ([id].report.v2.json)
type EvalReport struct {
	Results         map[string]EvalResult `json:"results"`
	ContextHash     string                `json:"context_hash,omitempty"`
	ContextSnapshot EvalContext           `json:"context_snapshot,omitempty"`
}

// ContextMeta contains metadata for the context
type ContextMeta struct {
	BusinessGoal            string `json:"business_goal"`
	AudioRealityInference   string `json:"audio_reality_inference"`
	TotalTokenCountEstimate int    `json:"total_token_count_estimate"`
	GroundTruth             string `json:"ground_truth"`
	QuestionableGT          bool   `json:"questionable_gt"`
	QuestionableReason      string `json:"questionable_reason"`
}

// Checkpoint represents a hierarchical evaluation point
type Checkpoint struct {
	ID          string  `json:"id"`
	TextSegment string  `json:"text_segment"`
	Tier        int     `json:"tier"`
	Weight      float64 `json:"weight"`
	Rationale   string  `json:"rationale"`
}

// EvalResult represents the evaluation result for a single model (Map based)
type EvalResult struct {
	Transcript        string                      `json:"transcript"`
	RevisedTranscript string                      `json:"revised_transcript"`
	Metrics           EvalMetrics                 `json:"metrics"`
	CheckpointResults map[string]CheckpointResult `json:"checkpoint_results"`
	Summary           []string                    `json:"summary"`
}

// EvalMetrics holds various evaluation metrics
type EvalMetrics struct {
	SScore          float64         `json:"S_score"`
	PScore          float64         `json:"P_score"`
	PhoneticDetails PhoneticDetails `json:"PER_details"`
}

// PhoneticDetails holds details for Phoneme Error Rate
type PhoneticDetails struct {
	Sub int `json:"sub"`
	Del int `json:"del"`
	Ins int `json:"ins"`
}

// CheckpointStatus represents the pass/fail status of a checkpoint
type CheckpointStatus string

const (
	StatusPass    CheckpointStatus = "Pass"
	StatusFail    CheckpointStatus = "Fail"
	StatusPartial CheckpointStatus = "Partial"
)

// CheckpointResult holds the pass/fail status for a single checkpoint
type CheckpointResult struct {
	Status   CheckpointStatus `json:"status"`
	Detected string           `json:"detected"`         // text segment identified
	Reason   string           `json:"reason,omitempty"` // Reason for failure
}
