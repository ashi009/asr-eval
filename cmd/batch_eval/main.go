package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"asr-eval/pkg/evalv2"
)

func main() {
	datasetDir := flag.String("dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	llmModel := flag.String("llm-model", "gemini-3-flash-preview", "LLM model to use")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent workers")
	flag.Parse()

	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	files, err := ioutil.ReadDir(*datasetDir)
	if err != nil {
		log.Fatalf("Failed to read dir: %v", err)
	}

	var mu sync.Mutex
	var processedCount int

	// Find all context files
	contextFiles := []string{}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".gt.v2.json") {
			contextFiles = append(contextFiles, f.Name())
		}
	}

	log.Printf("Found %d context files", len(contextFiles))

	paramsChan := make(chan string, len(contextFiles))
	var wg sync.WaitGroup

	worker := func(id int) {
		defer wg.Done()

		evaluator := evalv2.NewEvaluator(client, "", *llmModel)

		for ctxFileName := range paramsChan {
			id := strings.TrimSuffix(ctxFileName, ".gt.v2.json")
			ctxPath := filepath.Join(*datasetDir, ctxFileName)
			reportPath := filepath.Join(*datasetDir, id+".report.v2.json")
			gtPath := filepath.Join(*datasetDir, id+".gt.json")

			// Load Context
			ctxContent, err := ioutil.ReadFile(ctxPath)
			if err != nil {
				log.Printf("[%s] Error reading context: %v", id, err)
				continue
			}

			var ctxResp evalv2.ContextResponse
			if err := json.Unmarshal(ctxContent, &ctxResp); err != nil {
				log.Printf("[%s] Error parsing context: %v", id, err)
				continue
			}

			// Check Questionable
			if ctxResp.Meta.QuestionableGT {
				reason := ctxResp.Meta.QuestionableReason
				log.Printf("[%s] Questionable GT: %s", id, reason)

				// Load current GT to append note
				var currentGT string
				exists := false

				if content, err := ioutil.ReadFile(gtPath); err == nil {
					var gtObj struct {
						GroundTruth string `json:"ground_truth"`
					}
					if err := json.Unmarshal(content, &gtObj); err == nil {
						currentGT = gtObj.GroundTruth
						exists = true
					}
				}

				// If not in gt.json, try to find it from txt or transcript (to create the file)
				if !exists {
					// Logic from batch_gen: check .txt or first transcript
					txtPath := filepath.Join(*datasetDir, id+".txt")
					if content, err := ioutil.ReadFile(txtPath); err == nil {
						currentGT = string(content)
					} else {
						// Traverse files to find transcript
						for _, f := range files {
							name := f.Name()
							if strings.HasPrefix(name, id+".") && strings.HasSuffix(name, ".txt") && name != id+".txt" {
								if c, err := ioutil.ReadFile(filepath.Join(*datasetDir, name)); err == nil {
									currentGT = string(c)
									break
								}
							}
						}
					}
				}

				if currentGT == "" {
					log.Printf("[%s] Could not find original GT to tag", id)
					continue
				}

				// Append note if not already there
				tag := "\n\n[Review Needed]: "
				if !strings.Contains(currentGT, tag) {
					newGT := currentGT + tag + reason

					// Save to gt.json
					gtObj := map[string]string{"ground_truth": newGT}
					bytes, _ := json.MarshalIndent(gtObj, "", "  ")
					if err := ioutil.WriteFile(gtPath, bytes, 0644); err != nil {
						log.Printf("[%s] Failed to update GT: %v", id, err)
					} else {
						log.Printf("[%s] Updated GT with review note", id)
					}
				} else {
					log.Printf("[%s] Already tagged", id)
				}
				continue
			}

			// If not questionable -> Run Eval
			// Check if already done? User said run all. Safe to overwrite.
			// checking report existence to avoid re-run if script restarts
			if _, err := os.Stat(reportPath); err == nil {
				// log.Printf("[%s] Report exists, skipping (remove this check to force re-run)", id)
				// continue
			}

			log.Printf("[%s] Running Eval...", id)

			// Gather transcripts (COPY from batch_gen)
			// Gather transcripts (Corrected logic matches server/main.go)
			transcripts := make(map[string]string)
			for _, f := range files {
				name := f.Name()
				if !strings.HasPrefix(name, id+".") {
					continue
				}

				ext := filepath.Ext(name)
				// Skip metadata, audio, and v2 files
				if ext == ".json" || ext == ".flac" || strings.Contains(ext, "v2") {
					continue
				}

				// Provider is the extension without dot
				// e.g. "id.ifly" -> provider "ifly"
				// "id.txt" -> provider "txt"
				if ext != "" {
					provider := strings.TrimPrefix(ext, ".")
					content, err := ioutil.ReadFile(filepath.Join(*datasetDir, name))
					if err == nil {
						transcripts[provider] = string(content)
					}
				}
			}

			if len(transcripts) == 0 {
				log.Printf("[%s] No transcripts found", id)
				continue
			}

			// Evaluate
			// We need a timeout context?
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			result, err := evaluator.Evaluate(ctx, &ctxResp, transcripts)
			cancel()

			if err != nil {
				log.Printf("[%s] Eval Failed: %v", id, err)
				continue
			}

			// Save Report
			// We need to inject "ground_truth" into the report struct if needed by UI?
			// The EvaluationResponse struct in types.go:
			// type EvaluationResponse struct { Evaluations map[string]ModelEvaluation ... }
			// UI might look at case.eval_report.ground_truth?
			// CaseDetail.tsx: const evalGT = evalReport?.ground_truth;

			// evaluator.go returns *EvaluationResponse.
			// Let's check types.go for EvaluationResponse definition.
			// It probably matches what's saved.

			// Add GT to the result to match UI expectation if it's missing from evaluator output
			// Evaluator.Evaluate returns EvaluationResponse which has Evalutations map.
			// Wait, types.go:
			// type EvaluationResponse struct { GroundTruth string; Evaluations ... } ?
			// I need to check types.go content from step 21/viewed files.
			// Step 21 summary says: "ModelEvaluationLLM, ContextResponse ... EvaluationResponse".
			// Let's assume I need to set GroundTruth in the saved JSON if the evaluator doesn't do it.
			// Evaluator code (step 85) sets Evaluations map but doesn't seem to set GroundTruth field on finalResult?
			// Line 105: finalResult := &EvaluationResponse{ Evaluations: make(...) }
			// It does NOT set GroundTruth. I must set it manually.

			result.GroundTruth = ctxResp.Meta.GroundTruth // Use GT from Context

			bytes, _ := json.MarshalIndent(result, "", "  ")
			if err := ioutil.WriteFile(reportPath, bytes, 0644); err != nil {
				log.Printf("[%s] Failed to save report: %v", id, err)
			} else {
				mu.Lock()
				processedCount++
				mu.Unlock()
				log.Printf("[%s] Saved Report", id)
			}
		}
	}

	log.Printf("Starting %d workers...", *concurrency)
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	for _, name := range contextFiles {
		paramsChan <- name
	}
	close(paramsChan)

	wg.Wait()
	log.Printf("Done. Processed %d evaluations.", processedCount)
}
