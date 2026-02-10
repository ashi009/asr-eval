package workspace

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"asr-eval/pkg/evalv2"

	"google.golang.org/genai"
)

type ServiceConfig struct {
	DatasetDir       string
	GenModel         string
	EvalModel        string
	EnabledProviders map[string]bool
}

// DefaultServiceConfig returns the default configuration for the service.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		DatasetDir: "transcripts_and_audios",
		GenModel:   "gemini-3-pro-preview",
		EvalModel:  "gemini-3-flash-preview",
		EnabledProviders: map[string]bool{
			"volc":         false,
			"volc_ctx":     false,
			"volc_ctx_rt":  false,
			"volc2_ctx":    false,
			"volc2_ctx_rt": true,
			"qwen_ctx_rt":  true,
			"ifly":         true,
			"ifly_mq":      true,
			"ifly_en":      false,
			"iflybatch":    false,
			"dg":           true,
			"snx":          true,
			"snxrt":        true,
			"snxrt_v4":     true,
			"ist_basic":    true,
			"txt":          true,
		},
	}
}

type Service struct {
	Config    ServiceConfig
	GenClient *genai.Client
}

func NewService(config ServiceConfig, client *genai.Client) *Service {
	return &Service{
		Config:    config,
		GenClient: client,
	}
}

// ListCases scans the directory and returns summary Case objects.
func (s *Service) ListCases(ctx context.Context) ([]*Case, error) {
	var results []*Case
	dir := s.Config.DatasetDir

	// We scan for .report.v2.json and .gt.v2.json and .flac
	// Simplified scanning logic:
	// 1. Identify all unique case IDs (basename of .flac files)
	// 2. For each ID, check if ReportV2 or GT exists and populate a lightweight Case object.

	// Optimization: Read all directory entries once
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filesMap := make(map[string]map[string]bool) // id -> extension -> true
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".flac") {
			id := strings.TrimSuffix(name, ".flac")
			if filesMap[id] == nil {
				filesMap[id] = make(map[string]bool)
			}
			filesMap[id][".flac"] = true
		} else if strings.HasSuffix(name, ".report.v2.json") {
			id := strings.TrimSuffix(name, ".report.v2.json")
			if filesMap[id] == nil {
				filesMap[id] = make(map[string]bool)
			}
			filesMap[id][".report.v2.json"] = true
		} else if strings.HasSuffix(name, ".gt.v2.json") {
			id := strings.TrimSuffix(name, ".gt.v2.json")
			if filesMap[id] == nil {
				filesMap[id] = make(map[string]bool)
			}
			filesMap[id][".gt.v2.json"] = true
		}
	}

	for id, exts := range filesMap {
		if !exts[".flac"] {
			// Skip if no audio file (sanity check)
			continue
		}

		c := &Case{ID: id}

		// Try to load ReportV2 first (most comprehensive)
		if exts[".report.v2.json"] {
			content, err := os.ReadFile(filepath.Join(dir, id+".report.v2.json"))
			if err == nil {
				var report evalv2.EvalReport
				if json.Unmarshal(content, &report) == nil {
					// Calculate QScores
					for k, v := range report.Results {
						v.Metrics.QScore = v.Metrics.CompositeScore()
						report.Results[k] = v
					}
					c.ReportV2 = &report
					// We also populate EvalContext from the snapshot if available
					if report.ContextSnapshot.Meta.TotalTokenCountEstimate > 0 {
						c.EvalContext = &report.ContextSnapshot
					}
				}
			}
		}

		// If no report or context not fully populated, check GT
		if c.EvalContext == nil && exts[".gt.v2.json"] {
			content, err := os.ReadFile(filepath.Join(dir, id+".gt.v2.json"))
			if err == nil {
				var ctx evalv2.EvalContext
				if json.Unmarshal(content, &ctx) == nil {
					c.EvalContext = &ctx
				}
			}
		}

		results = append(results, c)
	}

	// Sort by ID to ensure stable order
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results, nil
}

// GetCase returns full details for a case
func (s *Service) GetCase(ctx context.Context, id string) (*Case, error) {
	c := &Case{
		ID:          id,
		Transcripts: make(map[string]string),
	}

	// 1. Load Transcripts
	// We scan the directory for [id].[provider].txt or similar patterns
	// Actually, pattern is [id].[provider] (no extension? or .txt?)
	// Based on `main.go` snippet: `ext != "" && ext != ".json" && ext != ".flac"`

	files, err := os.ReadDir(s.Config.DatasetDir)
	if err != nil {
		return nil, err
	}

	found := false
	for _, f := range files {
		name := f.Name()
		if !strings.HasPrefix(name, id+".") {
			continue
		}

		if name == id+".flac" {
			found = true
			continue
		}

		path := filepath.Join(s.Config.DatasetDir, name)
		content, _ := os.ReadFile(path)

		if strings.HasSuffix(name, ".gt.v2.json") {
			var ctx evalv2.EvalContext
			if json.Unmarshal(content, &ctx) == nil {
				c.EvalContext = &ctx
			}
		} else if strings.HasSuffix(name, ".report.v2.json") {
			var report evalv2.EvalReport
			if json.Unmarshal(content, &report) == nil {
				for provider, res := range report.Results {
					res.Metrics.QScore = res.Metrics.CompositeScore()
					report.Results[provider] = res
				}
				c.ReportV2 = &report
			}
		} else {
			// Transcripts
			// Assumption: anything else starting with id. is a transcript
			// except known extensions
			ext := filepath.Ext(name)
			if ext != ".json" && ext != ".flac" {
				// Provider is the extension without dot? Or the part after ID?
				// Original logic: `provider := strings.TrimPrefix(ext, ".")`
				// But wait, if file is `id.provider`, ext is `.provider`.
				provider := strings.TrimPrefix(ext, ".")
				c.Transcripts[provider] = string(content)
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("case not found: %s", id)
	}

	return c, nil
}

// UpdateCase updates a case based on FieldMask.
// Replaces SaveGT and SaveContext.
// UpdateContext updates the eval context for a case.
// Replaces UpdateCase.
func (s *Service) UpdateContext(ctx context.Context, req UpdateContextRequest) (*Case, error) {
	if req.EvalContext == nil {
		return nil, fmt.Errorf("EvalContext is required")
	}

	// Update Context file
	filename := filepath.Join(s.Config.DatasetDir, req.ID+".gt.v2.json")
	bytes, _ := json.MarshalIndent(req.EvalContext, "", "  ")
	if err := os.WriteFile(filename, bytes, 0644); err != nil {
		return nil, err
	}

	// Invalidate Report (Side effect)
	reportPath := filepath.Join(s.Config.DatasetDir, req.ID+".report.v2.json")
	_ = os.Remove(reportPath)

	return s.GetCase(ctx, req.ID)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (s *Service) GenerateContext(ctx context.Context, req GenerateContextRequest) (*evalv2.EvalContext, error) {
	// If GroundTruth is provided, we temporarily save it or just use it?
	// The request has GroundTruth.
	// Should we save it? The previous logic saved it.
	// "Stateless" generation implies we just return it, but maybe we need to save GT to valid file.
	// Let's save GT if provided, as it's a prerequisite.

	if req.GroundTruth != "" {
		// Internal call to save GT
		filename := filepath.Join(s.Config.DatasetDir, req.ID+".gt.json")
		data := struct {
			GroundTruth string `json:"ground_truth"`
		}{GroundTruth: req.GroundTruth}
		bytes, _ := json.MarshalIndent(data, "", "  ")
		_ = os.WriteFile(filename, bytes, 0644)
	}

	if s.GenClient == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}

	evaluator := evalv2.NewEvaluator(s.GenClient, s.Config.GenModel, s.Config.EvalModel)
	audioPath := filepath.Join(s.Config.DatasetDir, req.ID+".flac")

	// Load transcripts from disk
	c, err := s.GetCase(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load case: %w", err)
	}
	transcripts := c.Transcripts

	ctxResp, _, err := evaluator.GenerateContext(ctx, audioPath, req.GroundTruth, transcripts)
	if err != nil {
		return nil, err
	}

	// Calculate Hash
	ctxBytes, _ := json.Marshal(ctxResp)
	hash := md5.Sum(ctxBytes)
	ctxResp.Hash = hex.EncodeToString(hash[:])

	return ctxResp, nil
}

func (s *Service) Evaluate(ctx context.Context, req EvaluateRequest) (*evalv2.EvalReport, error) {
	if s.GenClient == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}

	if req.EvalContext == nil {
		return nil, fmt.Errorf("EvalContext is required")
	}

	evaluator := evalv2.NewEvaluator(s.GenClient, s.Config.GenModel, s.Config.EvalModel)

	// Load Transcripts
	c, err := s.GetCase(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load case: %w", err)
	}

	// Filter transcripts based on ProviderIDs
	transcripts := make(map[string]string)
	if len(req.ProviderIDs) > 0 {
		for _, pid := range req.ProviderIDs {
			if t, ok := c.Transcripts[pid]; ok {
				transcripts[pid] = t
			}
		}
	} else {
		transcripts = c.Transcripts
	}

	resp, _, err := evaluator.Evaluate(ctx, req.EvalContext, transcripts)
	if err != nil {
		return nil, err
	}

	// Calculate Context Hash
	ctxBytes, _ := json.Marshal(req.EvalContext)
	hash := md5.Sum(ctxBytes)
	contextHash := hex.EncodeToString(hash[:])
	resp.ContextHash = contextHash
	resp.ContextSnapshot = *req.EvalContext

	// Save Report (Merge with existing)
	filename := filepath.Join(s.Config.DatasetDir, req.ID+".report.v2.json")
	var finalReport *evalv2.EvalReport

	if existingBytes, err := os.ReadFile(filename); err == nil {
		var existingReport evalv2.EvalReport
		if json.Unmarshal(existingBytes, &existingReport) == nil {
			if existingReport.ContextHash == contextHash {
				for provider, result := range resp.Results {
					existingReport.Results[provider] = result
				}
				finalReport = &existingReport
			}
		}
	}

	if finalReport == nil {
		finalReport = resp
	}

	bytes, _ := json.MarshalIndent(finalReport, "", "  ")
	if err := os.WriteFile(filename, bytes, 0644); err != nil {
		return nil, err
	}

	return finalReport, nil
}
