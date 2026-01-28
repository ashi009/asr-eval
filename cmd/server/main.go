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
}

var (
	datasetDir   string
	llmModelFlag string
)

func main() {
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	flag.StringVar(&llmModelFlag, "llm-model", "doubao-seed-1-8-251228", "LLM model to use for evaluation")
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
	http.HandleFunc("/api/config", configHandler)

	port := 8080
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

	targetSuffix := fmt.Sprintf(".%s.result.json", llmModelFlag)
	// oldSuffix := ".result.json" // Removed unused variable

	for _, f := range files {
		name := f.Name()
		// STRICT filtering: only accept [id].[llmModelFlag].result.json
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

			var fileData AIResultFile
			// We only care about the specific model result in that file
			if err := json.Unmarshal(content, &fileData); err == nil && fileData.Results != nil {
				// The result file for a model should ideally contain results FOR that model.
				// But our format matches keys like "volc", "gemini".
				// We display WHO defeated WHO? No, list view just needs best.
				// Actually, if we filter by model, we only care about THAT model's performance?
				// User wants "only show results from specified llm".
				// So "Best Performer" concept changes to just "Score".
				// But let's stick to existing structure: if result exists, it has a score.
				for provider, res := range fileData.Results {
					if res.Score > infoMap[id].maxScore {
						infoMap[id].maxScore = res.Score
						infoMap[id].bestPerformers = []string{provider}
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

	data := &FileData{
		ID:        id,
		Filename:  id,
		Results:   make(map[string]string),
		AIResults: make(map[string]llm.EvalResult),
	}

	targetSuffix := fmt.Sprintf(".%s.result.json", llmModelFlag)

	// Read all related files for this ID
	files, _ := ioutil.ReadDir(datasetDir)
	for _, f := range files {
		name := f.Name()
		if !strings.HasPrefix(name, id) {
			continue
		}

		path := filepath.Join(datasetDir, name)

		if strings.HasSuffix(name, ".eval.json") {
			content, _ := ioutil.ReadFile(path)
			json.Unmarshal(content, &data.Evaluation)
		} else if name == id+targetSuffix {
			// STRICT filtering: Only read results for the ACTIVE model
			content, _ := ioutil.ReadFile(path)

			if data.AIResults == nil {
				data.AIResults = make(map[string]llm.EvalResult)
			}
			if data.EvaluatedTranscripts == nil {
				data.EvaluatedTranscripts = make(map[string]string)
			}

			var fileData AIResultFile
			if err := json.Unmarshal(content, &fileData); err == nil && fileData.Results != nil {
				// Merge results
				for k, v := range fileData.Results {
					data.AIResults[k] = v
					data.EvaluatedTranscripts[k] = v.OriginalTranscript
				}
				if fileData.EvaluatedGroundTruth != "" {
					data.EvaluatedGroundTruth = fileData.EvaluatedGroundTruth
				}
			}
		} else if strings.HasSuffix(name, ".flac") {
			// Audio file, ignore
		} else {
			// Transcript files (e.g. id.volc.txt, id.volc2.txt)
			// Assuming format id.[provider].txt or pure text files
			ext := filepath.Ext(name)
			// Heuristic: if it's not one of the known json extensions and not flac
			if ext != "" && ext != ".json" && ext != ".flac" {
				// provider is derived from filename?
				// e.g. "uuid.volc2" -> provider "volc2"
				// e.g. "uuid.txt" -> provider "txt"?
				// The previous code used `strings.TrimPrefix(ext, ".")` which returns "volc2"
				provider := strings.TrimPrefix(ext, ".")
				content, err := ioutil.ReadFile(path)
				if err == nil {
					data.Results[provider] = string(content)
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

	// Strictly target the file for the current model
	filename := fmt.Sprintf("%s.%s.result.json", id, llmModelFlag)
	path := filepath.Join(datasetDir, filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// If specific file doesn't exist, maybe it's legacy?
		// But in strict mode we probably only want to delete what we see.
		// Let's just try to delete and ignore not exist.
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := os.Remove(path); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete result file: %v", err), http.StatusInternalServerError)
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
		ExistingResults map[string]llm.EvalResult `json:"existing_results"` // Ignored in favor of disk + new eval
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Save Ground Truth
	evalFilename := filepath.Join(datasetDir, req.ID+".eval.json")
	var evalData Evaluation
	if content, err := ioutil.ReadFile(evalFilename); err == nil {
		json.Unmarshal(content, &evalData)
	}
	evalData.GroundTruth = req.GroundTruth
	evalBytes, _ := json.MarshalIndent(evalData, "", "  ")
	ioutil.WriteFile(evalFilename, evalBytes, 0644)

	// 2. Setup LLM Client
	client, err := llm.NewClient(llmModelFlag)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to init LLM client: %v", err), http.StatusInternalServerError)
		return
	}
	evaluator := llm.NewEvaluator(client)

	// 3. Run Evaluation
	var newResults map[string]llm.EvalResult
	if len(req.Results) > 0 {
		newResults, err = evaluator.Evaluate(r.Context(), req.GroundTruth, req.Results)
		if err != nil {
			log.Printf("LLM Eval failed: %v", err)
			http.Error(w, fmt.Sprintf("LLM evaluation failed: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		newResults = make(map[string]llm.EvalResult)
	}

	// 4. Determine Output File
	resultFilename := filepath.Join(datasetDir, fmt.Sprintf("%s.%s.result.json", req.ID, llmModelFlag))

	// 5. Load Existing Results (stateful merge)
	finalResults := make(map[string]llm.EvalResult)

	if fileBytes, err := ioutil.ReadFile(resultFilename); err == nil {
		var fileData AIResultFile
		if err := json.Unmarshal(fileBytes, &fileData); err == nil && fileData.Results != nil {
			finalResults = fileData.Results
		}
	}

	// 6. Merge New Results
	for k, v := range newResults {
		finalResults[k] = v
	}

	// 7. Save
	fileData := AIResultFile{
		EvaluatedGroundTruth: req.GroundTruth,
		Results:              finalResults,
	}
	data, _ := json.MarshalIndent(fileData, "", "  ")
	ioutil.WriteFile(resultFilename, data, 0644)

	// 8. Return merged results (could be partial if user only wanted to see what just happened, but typically specific provider results)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResults)
}
