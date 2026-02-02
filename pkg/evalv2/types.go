package evalv2

import (
	"reflect"
	"strings"

	"google.golang.org/genai"
)

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

// llmEvalReport is the raw array from LLM (unexported)
type llmEvalReport []llmEvalResult

// llmEvalResult is the raw result from LLM for a transcript (unexported)
type llmEvalResult struct {
	Provider          string                `json:"provider"`
	RevisedTranscript string                `json:"revised_transcript"`
	Metrics           EvalMetrics           `json:"metrics"`
	CheckpointResults []llmCheckpointResult `json:"checkpoint_results"`
	Summary           []string              `json:"summary"`
}

// CheckpointStatus represents the pass/fail status of a checkpoint
type CheckpointStatus string

const (
	StatusPass    CheckpointStatus = "Pass"
	StatusFail    CheckpointStatus = "Fail"
	StatusPartial CheckpointStatus = "Partial"
)

// llmCheckpointResult is the raw result from LLM (unexported)
type llmCheckpointResult struct {
	ID       string           `json:"id"`
	Status   CheckpointStatus `json:"status" schema:"enum:Pass,Fail,Partial"` // "Pass", "Fail", or "Partial"
	Detected string           `json:"detected"`                               // text segment identified
	Reason   string           `json:"reason,omitempty"`                       // Reason for failure
}

// CheckpointResult holds the pass/fail status for a single checkpoint
type CheckpointResult struct {
	Status   CheckpointStatus `json:"status"`           // "Pass", "Fail", or "Partial"
	Detected string           `json:"detected"`         // text segment identified
	Reason   string           `json:"reason,omitempty"` // Reason for failure
}

// reflectSchema converts a Go type to a genai.Schema using reflection.
func reflectSchema(t reflect.Type) *genai.Schema {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			return &genai.Schema{
				Type:  genai.TypeArray,
				Items: reflectSchema(t.Elem()),
			}
		}
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		schema := &genai.Schema{
			Type:       genai.TypeObject,
			Properties: make(map[string]*genai.Schema),
		}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}
			name := strings.Split(jsonTag, ",")[0]
			propSchema := reflectSchema(field.Type)

			// Handle custom 'schema' tag for enums or other constraints
			if schemaTag := field.Tag.Get("schema"); schemaTag != "" {
				parts := strings.Split(schemaTag, ";")
				for _, part := range parts {
					if strings.HasPrefix(part, "enum:") {
						enumVals := strings.Split(strings.TrimPrefix(part, "enum:"), ",")
						propSchema.Enum = enumVals
					}
				}
			}

			schema.Properties[name] = propSchema
			if !strings.Contains(jsonTag, "omitempty") {
				schema.Required = append(schema.Required, name)
			}
		}
		return schema
	case reflect.String:
		return &genai.Schema{Type: genai.TypeString}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &genai.Schema{Type: genai.TypeInteger}
	case reflect.Float32, reflect.Float64:
		return &genai.Schema{Type: genai.TypeNumber}
	case reflect.Bool:
		return &genai.Schema{Type: genai.TypeBoolean}
	case reflect.Map:
		// Map is treated as Object but we can't strongly type individual keys/values in standard Schema without Properties.
		// genai.Schema doesn't seem to have a formal "AdditionalProperties" field for generic maps in this simple SDK version.
		return &genai.Schema{Type: genai.TypeObject}
	default:
		return &genai.Schema{Type: genai.TypeString}
	}
}
