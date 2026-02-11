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

const (
	extFlac     = ".flac"
	extReportV2 = ".report.v2.json"
	extGTV2     = ".gt.v2.json"
	extJSON     = ".json"
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
		if strings.HasSuffix(name, extFlac) {
			id := strings.TrimSuffix(name, extFlac)
			if filesMap[id] == nil {
				filesMap[id] = make(map[string]bool)
			}
			filesMap[id][extFlac] = true
		} else if strings.HasSuffix(name, extReportV2) {
			id := strings.TrimSuffix(name, extReportV2)
			if filesMap[id] == nil {
				filesMap[id] = make(map[string]bool)
			}
			filesMap[id][extReportV2] = true
		} else if strings.HasSuffix(name, extGTV2) {
			id := strings.TrimSuffix(name, extGTV2)
			if filesMap[id] == nil {
				filesMap[id] = make(map[string]bool)
			}
			filesMap[id][extGTV2] = true
		}
	}

	for id, exts := range filesMap {
		if !exts[extFlac] {
			// Skip if no audio file (sanity check)
			continue
		}

		c := &Case{ID: id}

		// Try to load GT first (Precedence)
		if exts[extGTV2] {
			ctx, err := s.loadEvalContext(id)
			if err == nil {
				c.EvalContext = ctx
			}
		}

		// Load Report
		if exts[extReportV2] {
			report, err := s.loadEvalReport(id)
			if err == nil {
				c.ReportV2 = report
				// If no GT loaded yet, use snapshot
				if c.EvalContext == nil && report.ContextSnapshot.Hash != "" {
					c.EvalContext = &report.ContextSnapshot
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

		if name == id+extFlac {
			found = true
			continue
		}

		path := filepath.Join(s.Config.DatasetDir, name)

		if strings.HasSuffix(name, extGTV2) {
			ctx, err := s.loadEvalContext(id)
			if err == nil {
				c.EvalContext = ctx
			}
		} else if strings.HasSuffix(name, extReportV2) {
			report, err := s.loadEvalReport(id)
			if err == nil {
				c.ReportV2 = report
				// If no GT loaded yet, use snapshot
				if c.EvalContext == nil && report.ContextSnapshot.Hash != "" {
					c.EvalContext = &report.ContextSnapshot
				}
			}
		} else {
			// Transcripts
			ext := filepath.Ext(name)
			if ext != extJSON && ext != extFlac {
				content, _ := os.ReadFile(path)
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

// UpdateContext updates the eval context for a case.
func (s *Service) UpdateContext(ctx context.Context, req UpdateContextRequest) (*Case, error) {
	if req.EvalContext == nil {
		return nil, fmt.Errorf("EvalContext is required")
	}

	// Recalculate Hash
	// Update Context file
	if err := s.writeEvalContext(req.ID, req.EvalContext); err != nil {
		return nil, err
	}

	// Invalidate Report (Side effect)
	reportPath := filepath.Join(s.Config.DatasetDir, req.ID+extReportV2)
	_ = os.Remove(reportPath)

	return s.GetCase(ctx, req.ID)
}

func (s *Service) GenerateContext(ctx context.Context, req GenerateContextRequest) (*evalv2.EvalContext, error) {
	if s.GenClient == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}

	evaluator := evalv2.NewEvaluator(s.GenClient, s.Config.GenModel, s.Config.EvalModel)
	audioPath := filepath.Join(s.Config.DatasetDir, req.ID+extFlac)

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

	bytes, _ := json.Marshal(ctxResp)
	hash := md5.Sum(bytes)
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

	contextHash := req.EvalContext.Hash
	resp.ContextSnapshot = *req.EvalContext

	// Save Report (Merge with existing)
	existingReport, err := s.loadEvalReport(req.ID)
	var finalReport *evalv2.EvalReport

	if err == nil && existingReport.ContextSnapshot.Hash == contextHash {
		for provider, result := range resp.Results {
			existingReport.Results[provider] = result
		}
		finalReport = existingReport
	} else {
		finalReport = resp
	}

	if err := s.writeEvalReport(req.ID, finalReport); err != nil {
		return nil, err
	}

	return finalReport, nil
}

func (s *Service) loadEvalReport(id string) (*evalv2.EvalReport, error) {
	filename := filepath.Join(s.Config.DatasetDir, id+extReportV2)
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var report evalv2.EvalReport
	if err := json.Unmarshal(content, &report); err != nil {
		return nil, err
	}
	// Calculate QScores
	for k, v := range report.Results {
		// TODO: remove this once we switch to MER
		if v.Metrics.PScore > 1 {
			v.Metrics.PScore = 0
		}
		v.Metrics.QScore = v.Metrics.CompositeScore()
		report.Results[k] = v
	}
	return &report, nil
}

func (s *Service) loadEvalContext(id string) (*evalv2.EvalContext, error) {
	filename := filepath.Join(s.Config.DatasetDir, id+extGTV2)
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var ctx evalv2.EvalContext
	if err := json.Unmarshal(content, &ctx); err != nil {
		return nil, err
	}
	return &ctx, nil
}

func (s *Service) writeEvalReport(id string, report *evalv2.EvalReport) error {
	filename := filepath.Join(s.Config.DatasetDir, id+extReportV2)
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, bytes, 0644)
}

func (s *Service) writeEvalContext(id string, ctx *evalv2.EvalContext) error {
	filename := filepath.Join(s.Config.DatasetDir, id+extGTV2)
	bytes, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, bytes, 0644)
}
