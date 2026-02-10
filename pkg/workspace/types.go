package workspace

import "asr-eval/pkg/evalv2"

// Case represents a workspace case.
// AIP-121: Resources should be defined by their data, not by view-specific fields if possible.
// We remove redundant fields that can be derived from EvalContext or ReportV2.
type Case struct {
	ID string `json:"id"`

	// Data Fields
	// These are the source of truth.
	// In List view, these might be partially populated or masked.
	// In Get view, these should be fully populated.
	// GroundTruth is accessed via EvalContext or ReportV2.
	Transcripts map[string]string `json:"transcripts,omitempty"`

	// Complex Objects
	EvalContext *evalv2.EvalContext `json:"eval_context,omitempty"`
	ReportV2    *evalv2.EvalReport  `json:"report_v2,omitempty"`
}

// Config returns the server configuration.
type Config struct {
	GenModel         string          `json:"gen_model"`
	EvalModel        string          `json:"eval_model"`
	EnabledProviders map[string]bool `json:"enabled_providers"`
}

// UpdateContextRequest for POST /api/cases/{id}:updateContext
// Replaces generic UpdateCase.
type UpdateContextRequest struct {
	ID          string              `json:"-"`
	EvalContext *evalv2.EvalContext `json:"eval_context"`
}

// GenerateContextRequest for POST /api/cases/{id}:generateContext
// Custom method.
type GenerateContextRequest struct {
	ID          string `json:"-"` // Extracted from URL
	GroundTruth string `json:"ground_truth"`
}

// EvaluateRequest for POST /api/cases/{id}:evaluate
// Custom method.
type EvaluateRequest struct {
	ID          string              `json:"-"` // Extracted from URL
	EvalContext *evalv2.EvalContext `json:"eval_context"`
	ProviderIDs []string            `json:"provider_ids"`
}
