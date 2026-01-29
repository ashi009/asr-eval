package main

import (
	"asr-eval/pkg/llm"
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

	"github.com/joho/godotenv"
)

type Case struct {
	ID          string            `json:"id"`
	GroundTruth string            `json:"ground_truth"`          // From [id].gt.json
	Transcripts map[string]string `json:"transcripts"`           // From [id].[provider].txt
	EvalReport  *llm.EvalReport   `json:"eval_report,omitempty"` // From [id].[model].report.json
}

var (
	datasetDir   string
	llmModelFlag string
)

func main() {
	var port int
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	flag.StringVar(&llmModelFlag, "llm-model", "doubao-seed-1-8-251228", "LLM model to use for evaluation")
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

	http.HandleFunc("/api/cases", listFilesHandler)
	http.HandleFunc("/api/evaluate-llm", runEvalHandler)
	http.HandleFunc("/api/case", getCaseHandler)
	// User didn't explicitly ask to change endpoint path, but consistency implies it.
	// But frontend calls /reset-eval. I should update frontend if I change endpoint.
	// Let's keep endpoint for now or change it?
	// Recommendation: Change endpoint to /api/reset-report for consistency.

	http.HandleFunc("/api/reset-eval", resetReportHandler)
	http.HandleFunc("/api/save-gt", saveGTHandler)
	http.HandleFunc("/api/config", configHandler)

	fmt.Printf("Attempting to listen on 127.0.0.1:%d...\n", port)
	fmt.Printf("Using dataset directory: %s\n", datasetDir)
	fmt.Printf("Default LLM Model: %s\n", llmModelFlag)
	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
	if err != nil {
		log.Fatalf("Failed to bind to 127.0.0.1:%d: %v\n", port, err)
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"llm_model": llmModelFlag,
	})
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
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
		maxScore       float64
	}
	infoMap := make(map[string]*caseInfo)

	targetSuffix := fmt.Sprintf(".%s.report.json", llmModelFlag)
	// oldSuffix := ".result.json" // Removed unused variable

	for _, f := range files {
		name := f.Name()
		// STRICT filtering: only accept [id].[llmModelFlag].report.json
		if strings.HasSuffix(name, targetSuffix) {
			id := strings.TrimSuffix(name, targetSuffix)
			if infoMap[id] == nil {
				infoMap[id] = &caseInfo{hasEval: true, maxScore: -1}
			}

			// Read scores to find winners
			content, err := ioutil.ReadFile(filepath.Join(root, name))
			if err != nil {
				continue
			}

			var report llm.EvalReport
			// We only care about the specific model result in that file
			if err := json.Unmarshal(content, &report); err == nil && report.EvalResults != nil {
				for provider, res := range report.EvalResults {
					if res.Score > infoMap[id].maxScore {
						infoMap[id].maxScore = res.Score
						infoMap[id].bestPerformers = []string{provider}
					} else if res.Score == infoMap[id].maxScore {
						infoMap[id].bestPerformers = append(infoMap[id].bestPerformers, provider)
					}
				}
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
		if info, ok := infoMap[basename]; ok {
			hasEval = info.hasEval
			bestPerformers = info.bestPerformers
		}

		results = append(results, map[string]interface{}{
			"id":              basename,
			"has_ai":          hasEval,
			"best_performers": bestPerformers,
		})

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

	targetSuffix := fmt.Sprintf(".%s.report.json", llmModelFlag)

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
		} else if name == id+targetSuffix {
			// STRICT filtering: Only read results for the ACTIVE model
			content, _ := ioutil.ReadFile(path)

			var report llm.EvalReport
			if err := json.Unmarshal(content, &report); err == nil {
				data.EvalReport = &report
			}
		} else if strings.HasSuffix(name, ".flac") {
			// Ignore
		} else {
			// Restore Transcript Logic
			ext := filepath.Ext(name)
			if ext != "" && ext != ".json" && ext != ".flac" {
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
	filename := fmt.Sprintf("%s.%s.report.json", id, llmModelFlag)
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

func runEvalHandler(w http.ResponseWriter, r *http.Request) {
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

	// 1. Save Ground Truth
	gtFilename := filepath.Join(datasetDir, req.ID+".gt.json")
	gtData := struct {
		GroundTruth string `json:"ground_truth"`
	}{
		GroundTruth: req.GroundTruth,
	}
	if gtBytes, err := json.MarshalIndent(gtData, "", "  "); err == nil {
		if err := ioutil.WriteFile(gtFilename, gtBytes, 0644); err != nil {
			log.Printf("EVAL: Failed to save GT for ID=%s: %v", req.ID, err)
		} else {
			log.Printf("EVAL: Saved GT for ID=%s", req.ID)
		}
	}
	// 2. Setup LLM Client
	client, err := llm.NewClient(llmModelFlag)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to init LLM client: %v", err), http.StatusInternalServerError)
		return
	}
	evaluator := llm.NewEvaluator(client)

	// 3. Run Evaluation
	var newResults map[string]llm.EvalResult
	if len(req.Transcripts) > 0 {
		newResults, err = evaluator.Evaluate(r.Context(), req.GroundTruth, req.Transcripts)
		if err != nil {
			log.Printf("LLM Eval failed: %v", err)
			http.Error(w, fmt.Sprintf("LLM evaluation failed: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		newResults = make(map[string]llm.EvalResult)
	}

	// 4. Determine Output File
	resultFilename := filepath.Join(datasetDir, fmt.Sprintf("%s.%s.report.json", req.ID, llmModelFlag))
	log.Printf("EVAL: Running evaluation for ID=%s, Output=%s", req.ID, resultFilename)

	// 5. Load Existing Results (stateful merge)
	finalResults := make(map[string]llm.EvalResult)

	if fileBytes, err := ioutil.ReadFile(resultFilename); err == nil {
		var report llm.EvalReport
		if err := json.Unmarshal(fileBytes, &report); err == nil && report.EvalResults != nil {
			finalResults = report.EvalResults
			log.Printf("EVAL: Loaded %d existing results for ID=%s", len(finalResults), req.ID)
		}
	}

	// 6. Merge New Results
	for k, v := range newResults {
		finalResults[k] = v
	}
	log.Printf("EVAL: Merged new results, total providers: %d", len(finalResults))

	// 7. Save
	report := llm.EvalReport{
		GroundTruth: req.GroundTruth,
		EvalResults: finalResults,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := ioutil.WriteFile(resultFilename, data, 0644); err != nil {
		log.Printf("EVAL: ERROR writing result file %s: %v", resultFilename, err)
	} else {
		log.Printf("EVAL: Successfully saved result file: %s", resultFilename)
	}

	// 8. Return Report (updated to return full report now)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}
