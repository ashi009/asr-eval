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

type FileData struct {
	ID                   string                    `json:"id"`
	Filename             string                    `json:"filename"`
	Results              map[string]string         `json:"results"`
	Evaluation           Evaluation                `json:"evaluation"`
	AIResults            map[string]llm.EvalResult `json:"ai_results"`
	EvaluatedGroundTruth string                    `json:"evaluated_ground_truth"`
	EvaluatedTranscripts map[string]string         `json:"evaluated_transcripts"`
}

type Evaluation struct {
	GroundTruth string `json:"ground_truth"`
	Comment     string `json:"comment"`
}

type AIResultFile struct {
	EvaluatedGroundTruth string                    `json:"evaluated_ground_truth"`
	Results              map[string]llm.EvalResult `json:"results"`
	EvaluatedTranscripts map[string]string         `json:"evaluated_transcripts"`
}

var datasetDir string

func main() {
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
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
	http.HandleFunc("/api/evaluate", evaluateHandler)
	http.HandleFunc("/api/evaluate-llm", evaluateLLMHandler)
	http.HandleFunc("/api/case", getCaseHandler)
	http.HandleFunc("/api/reset-eval", resetEvalHandler)

	port := 8080
	fmt.Printf("Attempting to listen on 127.0.0.1:%d...\n", port)
	fmt.Printf("Using dataset directory: %s\n", datasetDir)
	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
	if err != nil {
		log.Fatalf("Failed to bind to 127.0.0.1:%d: %v\n", port, err)
	}
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

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

		// Check for existing results
		hasEval := false
		var bestPerformers []string
		resultPath := filepath.Join(root, basename+".result.json")
		if _, err := os.Stat(resultPath); err == nil {
			hasEval = true
			// Calculate best performer
			content, err := ioutil.ReadFile(resultPath)
			if err == nil {
				var aiResults map[string]llm.EvalResult

				// Try new format first
				var fileData AIResultFile
				if err := json.Unmarshal(content, &fileData); err == nil && fileData.Results != nil {
					aiResults = fileData.Results
				} else {
					// Fallback to old format (map)
					json.Unmarshal(content, &aiResults)
				}

				if aiResults != nil {
					var maxScore float64 = -1.0
					// First pass: find max score
					for _, res := range aiResults {
						if res.Score > maxScore {
							maxScore = res.Score
						}
					}
					// Second pass: collect all matching max score
					for name, res := range aiResults {
						if res.Score >= maxScore && maxScore >= 0 {
							bestPerformers = append(bestPerformers, name)
						}
					}
				}
			}
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

	data := &FileData{
		ID:        id,
		Filename:  id,
		Results:   make(map[string]string),
		AIResults: make(map[string]llm.EvalResult),
	}

	// Read all related files for this ID
	files, _ := ioutil.ReadDir(datasetDir)
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, id) {
			path := filepath.Join(datasetDir, name)

			if strings.HasSuffix(name, ".eval.json") {
				content, _ := ioutil.ReadFile(path)
				json.Unmarshal(content, &data.Evaluation)
			} else if strings.HasSuffix(name, ".result.json") {
				content, _ := ioutil.ReadFile(path)

				// Try new format
				var fileData AIResultFile
				if err := json.Unmarshal(content, &fileData); err == nil && fileData.Results != nil {
					data.AIResults = fileData.Results
					data.EvaluatedGroundTruth = fileData.EvaluatedGroundTruth
					data.EvaluatedTranscripts = fileData.EvaluatedTranscripts
				} else {
					// Fallback
					json.Unmarshal(content, &data.AIResults)
				}
			} else if strings.HasSuffix(name, ".flac") {
				// Audio file, ignore
			} else {
				// Transcript files
				ext := filepath.Ext(name)
				if ext != "" && ext != ".json" {
					provider := strings.TrimPrefix(ext, ".")
					content, err := ioutil.ReadFile(path)
					if err == nil {
						data.Results[provider] = string(content)
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func evaluateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID         string     `json:"id"`
		Evaluation Evaluation `json:"evaluation"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filename := filepath.Join(datasetDir, req.ID+".eval.json")
	data, _ := json.MarshalIndent(req.Evaluation, "", "  ")
	ioutil.WriteFile(filename, data, 0644)

	w.WriteHeader(http.StatusOK)
}

func resetEvalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	filename := filepath.Join(datasetDir, id+".result.json")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func evaluateLLMHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID              string                    `json:"id"`
		GroundTruth     string                    `json:"ground_truth"`
		Results         map[string]string         `json:"results"`
		ExistingResults map[string]llm.EvalResult `json:"existing_results"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-Save Ground Truth to [ID].eval.json before running eval
	evalFilename := filepath.Join(datasetDir, req.ID+".eval.json")
	var evalData Evaluation
	if content, err := ioutil.ReadFile(evalFilename); err == nil {
		json.Unmarshal(content, &evalData)
	}
	evalData.GroundTruth = req.GroundTruth // Update GT, preserve Comment
	evalBytes, _ := json.MarshalIndent(evalData, "", "  ")
	ioutil.WriteFile(evalFilename, evalBytes, 0644)

	var results map[string]llm.EvalResult
	var err error

	// Only run evaluation if there are results to evaluate
	if len(req.Results) > 0 {
		evaluator := llm.NewEvaluator()
		results, err = evaluator.Evaluate(r.Context(), req.GroundTruth, req.Results)
		if err != nil {
			log.Printf("LLM Eval failed: %v", err)
			http.Error(w, fmt.Sprintf("LLM evaluation failed: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		results = make(map[string]llm.EvalResult)
	}

	// Merge existing results (taking precedence if not overwritten)
	// 1. Start with what's on disk (safest source of truth)
	finalResults := make(map[string]llm.EvalResult)
	evaluatedTranscripts := make(map[string]string)
	var evaluatedGroundTruth string

	filename := filepath.Join(datasetDir, req.ID+".result.json")
	if fileBytes, err := ioutil.ReadFile(filename); err == nil {
		// Try new format
		var fileData AIResultFile
		if err := json.Unmarshal(fileBytes, &fileData); err == nil && fileData.Results != nil {
			finalResults = fileData.Results
			evaluatedGroundTruth = fileData.EvaluatedGroundTruth
			evaluatedTranscripts = fileData.EvaluatedTranscripts
		} else {
			// Old format
			json.Unmarshal(fileBytes, &finalResults)
		}
	}

	if evaluatedTranscripts == nil {
		evaluatedTranscripts = make(map[string]string)
	}

	// 2. Merge frontend provided existing results (optional, but respects request payload)
	for k, v := range req.ExistingResults {
		finalResults[k] = v
	}

	// 3. Overwrite with new evaluations
	for k, v := range results {
		finalResults[k] = v
		// Also update the evaluated transcript for this service
		if transcript, ok := req.Results[k]; ok {
			evaluatedTranscripts[k] = transcript
		}
	}

	// Update processed GT to current one
	evaluatedGroundTruth = req.GroundTruth

	// Persist Results in new format
	fileData := AIResultFile{
		EvaluatedGroundTruth: evaluatedGroundTruth,
		Results:              finalResults,
		EvaluatedTranscripts: evaluatedTranscripts,
	}

	data, _ := json.MarshalIndent(fileData, "", "  ")
	ioutil.WriteFile(filename, data, 0644)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResults)
}
