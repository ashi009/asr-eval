package main

import (
	"asr-eval/pkg/evalv2"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/genai"

	"github.com/joho/godotenv"
)

type Case struct {
	ID          string              `json:"id"`
	GroundTruth string              `json:"ground_truth"`           // From [id].gt.json
	Transcripts map[string]string   `json:"transcripts"`            // From [id].[provider].txt
	EvalContext *evalv2.EvalContext `json:"eval_context,omitempty"` // From [id].gt.v2.json
	ReportV2    *evalv2.EvalReport  `json:"report_v2,omitempty"`    // From [id].report.v2.json
}

var (
	datasetDir       string
	genModelFlag     string
	evalModelFlag    string
	enabledProviders = map[string]bool{
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
		"txt":          false,
	}
)

func main() {
	var port int
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	flag.StringVar(&genModelFlag, "gen-model", "gemini-3-pro-preview", "LLM model to use for context generation")
	flag.StringVar(&evalModelFlag, "eval-model", "gemini-3-flash-preview", "LLM model to use for evaluation")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.Parse()

	_ = godotenv.Load()

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the requested file exists, serve it
		path := "./static" + r.URL.Path
		if _, err := os.Stat(path); err == nil && r.URL.Path != "/" {
			fs.ServeHTTP(w, r)
			return
		}
		// Otherwise, serve index.html for SPA routing
		http.ServeFile(w, r, "./static/index.html")
	}))

	// Serve audio files
	audioFs := http.FileServer(http.Dir(datasetDir))
	http.Handle("/audio/", http.StripPrefix("/audio/", audioFs))

	http.HandleFunc("/api/cases", recoveryMiddleware(listCasesHandler))
	http.HandleFunc("/api/case", recoveryMiddleware(getCaseHandler))
	// User didn't explicitly ask to change endpoint path, but consistency implies it.
	// But frontend calls /reset-eval. I should update frontend if I change endpoint.
	// Let's keep endpoint for now or change it?
	// Recommendation: Change endpoint to /api/reset-report for consistency.

	http.HandleFunc("/api/evaluate-v2", recoveryMiddleware(evaluateV2Handler))
	http.HandleFunc("/api/generate-context", recoveryMiddleware(generateContextHandler))
	http.HandleFunc("/api/save-context", recoveryMiddleware(saveContextHandler))

	http.HandleFunc("/api/reset-eval", recoveryMiddleware(resetReportHandler))
	http.HandleFunc("/api/save-gt", recoveryMiddleware(saveGTHandler))
	http.HandleFunc("/api/config", recoveryMiddleware(configHandler))

	fmt.Printf("Attempting to listen on 127.0.0.1:%d...\n", port)
	fmt.Printf("Using dataset directory: %s\n", datasetDir)
	fmt.Printf("Gen Model: %s\n", genModelFlag)
	fmt.Printf("Eval Model: %s\n", evalModelFlag)
	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
	if err != nil {
		log.Fatalf("Failed to bind to 127.0.0.1:%d: %v\n", port, err)
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"gen_model":         genModelFlag,
		"eval_model":        evalModelFlag,
		"enabled_providers": enabledProviders,
	})
}

func recoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC: %v", err)
				http.Error(w, "Internal Server Error (Panic)", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

func listCasesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := scanFiles(datasetDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func scanFiles(root string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	files, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}

	type caseInfo struct {
		hasEval        bool
		bestPerformers []string
		maxScore       int
		// New fields for stats
		tokenCount     int
		evaluations    map[string]interface{}
		questionableGT bool
	}
	infoMap := make(map[string]*caseInfo)

	targetSuffix := ".report.v2.json"

	for _, f := range files {
		name := f.Name()
		// STRICT filtering: only accept [id].report.v2.json
		if strings.HasSuffix(name, targetSuffix) {
			id := strings.TrimSuffix(name, targetSuffix)
			if infoMap[id] == nil {
				infoMap[id] = &caseInfo{hasEval: true, maxScore: -1}
			}

			// Read scores to find winners and collect stats
			content, err := ioutil.ReadFile(filepath.Join(root, name))
			if err != nil {
				continue
			}

			var report evalv2.EvalReport
			if err := json.Unmarshal(content, &report); err == nil && report.Results != nil {
				// 1. Get Token Count and questionable_gt (with fallback)
				tokenCount := report.ContextSnapshot.Meta.TotalTokenCountEstimate
				questionableGT := report.ContextSnapshot.Meta.QuestionableGT
				if tokenCount <= 0 {
					// Fallback: Try to read from [id].gt.v2.json
					gtFilename := id + ".gt.v2.json"
					gtPath := filepath.Join(root, gtFilename)
					if gtContent, err := ioutil.ReadFile(gtPath); err == nil {
						var ctx evalv2.EvalContext
						if err := json.Unmarshal(gtContent, &ctx); err == nil {
							tokenCount = ctx.Meta.TotalTokenCountEstimate
							questionableGT = ctx.Meta.QuestionableGT
						}
					}
				}

				// 2. Prepare evaluations map (subset)
				evals := make(map[string]interface{})

				for provider, res := range report.Results {
					// Always compute QScore dynamically
					res.Metrics.QScore = res.Metrics.CompositeScore()

					// Store for stats (ONLY Metrics)
					evals[provider] = map[string]interface{}{
						"metrics": res.Metrics,
					}

					// Only consider enabled providers for best performers
					if enabled, ok := enabledProviders[provider]; !ok || !enabled {
						continue
					}
					score := res.Metrics.QScore
					if score > infoMap[id].maxScore {
						infoMap[id].maxScore = score
						infoMap[id].bestPerformers = []string{provider}
					} else if score == infoMap[id].maxScore {
						infoMap[id].bestPerformers = append(infoMap[id].bestPerformers, provider)
					}
				}

				infoMap[id].tokenCount = tokenCount
				infoMap[id].evaluations = evals
				infoMap[id].questionableGT = questionableGT
			}
		}
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		name := d.Name()
		if filepath.Ext(name) != ".flac" {
			return nil
		}

		basename := strings.TrimSuffix(name, ".flac")

		hasEval := false
		var bestPerformers []string
		var tokenCount int
		var evaluations map[string]interface{}
		var questionableGT bool

		if info, ok := infoMap[basename]; ok {
			hasEval = info.hasEval
			bestPerformers = info.bestPerformers
			tokenCount = info.tokenCount
			evaluations = info.evaluations
			questionableGT = info.questionableGT
		}

		// Also check gt.v2.json for non-evaluated cases
		if !hasEval {
			gtFilename := basename + ".gt.v2.json"
			gtPath := filepath.Join(root, gtFilename)
			if gtContent, err := ioutil.ReadFile(gtPath); err == nil {
				var ctx evalv2.EvalContext
				if err := json.Unmarshal(gtContent, &ctx); err == nil {
					questionableGT = ctx.Meta.QuestionableGT
				}
			}
		}

		// Construct partial Case object structure
		caseItem := map[string]interface{}{
			"id":              basename,
			"has_ai":          hasEval,
			"best_performers": bestPerformers,
		}

		if questionableGT {
			caseItem["questionable_gt"] = true
		}

		if hasEval {
			// Inject report_v2 with context_snapshot and evaluations
			reportV2 := map[string]interface{}{
				"context_snapshot": map[string]interface{}{
					"meta": map[string]interface{}{
						"total_token_count_estimate": tokenCount,
					},
				},
				"evaluations": evaluations,
			}
			caseItem["report_v2"] = reportV2
		}

		results = append(results, caseItem)

		return nil
	})

	return results, err
}

func getCaseHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	data := &Case{
		ID:          id,
		Transcripts: make(map[string]string),
	}

	// Read all related files for this ID
	files, _ := ioutil.ReadDir(datasetDir)
	for _, f := range files {
		name := f.Name()
		if !strings.HasPrefix(name, id) {
			continue
		}
		path := filepath.Join(datasetDir, name)

		if strings.HasSuffix(name, ".gt.json") {
			content, _ := ioutil.ReadFile(path)
			var gt struct {
				GroundTruth string `json:"ground_truth"`
			}
			if err := json.Unmarshal(content, &gt); err == nil {
				data.GroundTruth = gt.GroundTruth
			}
		} else if name == id+".gt.v2.json" {
			content, _ := ioutil.ReadFile(path)
			var ctx evalv2.EvalContext
			if err := json.Unmarshal(content, &ctx); err == nil {
				data.EvalContext = &ctx
			}
		} else if name == id+".report.v2.json" {
			// v2 reports are also model-specific? User didn't specify, but v1 is.
			// Let's assume v2 report is also likely model-specific or maybe just one global report?
			// The design says "[id].report.v2.json", implies single report or maybe we should use model?
			// Giving that v2 is "Evaluation V2", let's stick to one file for now as per design text.
			// But wait, if we change models we might want different reports.
			// However, for now let's follow the ".report.v2.json" naming from design.
			content, _ := ioutil.ReadFile(path)
			var report evalv2.EvalReport
			if err := json.Unmarshal(content, &report); err == nil {
				// Always compute Q_score dynamically (never stored in file)
				for provider, res := range report.Results {
					res.Metrics.QScore = res.Metrics.CompositeScore()
					report.Results[provider] = res
				}
				data.ReportV2 = &report
			}
		} else if strings.HasSuffix(name, ".flac") {
			// Ignore
		} else {
			// Restore Transcript Logic
			ext := filepath.Ext(name)
			if ext != "" && ext != ".json" && ext != ".flac" && !strings.Contains(ext, "v2") { // Avoid reading v2 json as transcript
				provider := strings.TrimPrefix(ext, ".")
				content, err := ioutil.ReadFile(path)
				if err == nil {
					data.Transcripts[provider] = string(content)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func resetReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// Strictly target the file for the current model
	filename := fmt.Sprintf("%s.report.v2.json", id)
	path := filepath.Join(datasetDir, filename)
	log.Printf("RESET: Request for ID=%s, Path=%s", id, path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("RESET: File not found, skipping delete: %s", path)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := os.Remove(path); err != nil {
		log.Printf("RESET: ERROR deleting file %s: %v", path, err)
		http.Error(w, fmt.Sprintf("Failed to delete result file: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("RESET: Successfully deleted file: %s", path)

	w.WriteHeader(http.StatusOK)
}

func saveGTHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
		GT string `json:"ground_truth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	filename := filepath.Join(datasetDir, req.ID+".gt.json")
	log.Printf("SAVE-GT: Request for ID=%s, Path=%s", req.ID, filename)

	// Read existing to preserve other fields if any match GroundThroughFile
	// Currently GroundTruthFile only has GroundTruth.
	// We just overwrite/create.

	data := struct {
		GroundTruth string `json:"ground_truth"`
	}{
		GroundTruth: req.GT,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("SAVE-GT: ERROR marshaling JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := ioutil.WriteFile(filename, bytes, 0644); err != nil {
		log.Printf("SAVE-GT: ERROR writing file %s: %v", filename, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("SAVE-GT: Successfully updated GT for ID=%s", req.ID)

	w.WriteHeader(http.StatusOK)
}

func generateContextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID          string            `json:"id"`
		GroundTruth string            `json:"ground_truth"`
		Transcripts map[string]string `json:"transcripts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save GT first (optional but good for consistency)
	if req.GroundTruth != "" {
		gtFilename := filepath.Join(datasetDir, req.ID+".gt.json")
		gtData := struct {
			GroundTruth string `json:"ground_truth"`
		}{GroundTruth: req.GroundTruth}
		if bytes, err := json.MarshalIndent(gtData, "", "  "); err == nil {
			_ = ioutil.WriteFile(gtFilename, bytes, 0644)
		}
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(r.Context(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to init LLM client: %v", err), http.StatusInternalServerError)
		return
	}
	// The original code used evalv2.NewGenerator(client, llmModelFlag).
	// So we change to evalv2.NewEvaluator(client, llmModelFlag).
	generator := evalv2.NewEvaluator(client, genModelFlag, evalModelFlag)

	audioPath := filepath.Join(datasetDir, req.ID+".flac")
	ctxResp, usage, err := generator.GenerateContext(r.Context(), audioPath, req.GroundTruth, req.Transcripts)
	if err != nil {
		log.Printf("GEN-CTX: Error %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if usage != nil {
		log.Printf("GEN-CTX: Usage: %d tokens", usage.TotalTokenCount)
	}

	// Calculate Hash for the generated context
	ctxBytes, _ := json.Marshal(ctxResp)
	hash := md5.Sum(ctxBytes)
	ctxResp.Hash = hex.EncodeToString(hash[:])

	// Stateless: Do NOT save to file yet. User must explicitly save.
	// filename := filepath.Join(datasetDir, req.ID+".gt.v2.json")
	// bytes, _ := json.MarshalIndent(ctxResp, "", "  ")
	// if err := ioutil.WriteFile(filename, bytes, 0644); err != nil {
	// 	http.Error(w, "Failed to save context file", http.StatusInternalServerError)
	// 	return
	// }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctxResp)
}

func saveContextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string              `json:"id"`
		Context *evalv2.EvalContext `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filename := filepath.Join(datasetDir, req.ID+".gt.v2.json")
	bytes, _ := json.MarshalIndent(req.Context, "", "  ")
	if err := ioutil.WriteFile(filename, bytes, 0644); err != nil {
		http.Error(w, "Failed to save context", http.StatusInternalServerError)
		return
	}

	// Reset (delete) the evaluation report for this case
	reportDetails := filepath.Join(datasetDir, req.ID+".report.v2.json")
	if err := os.Remove(reportDetails); err != nil && !os.IsNotExist(err) {
		log.Printf("SAVE-CTX: Warning: Failed to delete stale report %s: %v", reportDetails, err)
		// Don't fail the request, just log
	} else {
		log.Printf("SAVE-CTX: Cleared stale report for ID=%s", req.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func evaluateV2Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID          string              `json:"id"`
		EvalContext *evalv2.EvalContext `json:"eval_context"`
		Transcripts map[string]string   `json:"transcripts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.EvalContext == nil {
		http.Error(w, "Evaluation Context is required for V2 evaluation", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(r.Context(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to init LLM client: %v", err), http.StatusInternalServerError)
		return
	}
	evaluator := evalv2.NewEvaluator(client, genModelFlag, evalModelFlag)

	resp, usage, err := evaluator.Evaluate(r.Context(), req.EvalContext, req.Transcripts)
	if err != nil {
		log.Printf("EVAL-V2: Error %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if usage != nil {
		log.Printf("EVAL-V2: Usage: %d tokens", usage.TotalTokenCount)
	}

	// Calculate Hash and Embed Snapshot
	ctxBytes, _ := json.Marshal(req.EvalContext)
	hash := md5.Sum(ctxBytes)
	contextHash := hex.EncodeToString(hash[:])
	resp.ContextHash = contextHash
	resp.ContextSnapshot = *req.EvalContext

	// Save Report (with merging)
	filename := filepath.Join(datasetDir, req.ID+".report.v2.json")
	var finalReport *evalv2.EvalReport

	if existingBytes, err := os.ReadFile(filename); err == nil {
		var existingReport evalv2.EvalReport
		if err := json.Unmarshal(existingBytes, &existingReport); err == nil {
			if existingReport.ContextHash == contextHash {
				// Same context, merge results
				for provider, result := range resp.Results {
					existingReport.Results[provider] = result
				}
				finalReport = &existingReport
			}
		}
	}

	if finalReport == nil {
		// No existing report or different context, use new one
		finalReport = resp
	}

	bytes, _ := json.MarshalIndent(finalReport, "", "  ")
	if err := os.WriteFile(filename, bytes, 0644); err != nil {
		http.Error(w, "Failed to save report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalReport)
}
