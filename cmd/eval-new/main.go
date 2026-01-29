package main

import (
	"asr-eval/pkg/evalv2"
	"asr-eval/pkg/llm"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

var (
	datasetDir   string
	caseID       string
	modelContext string
	modelEval    string
)

func main() {
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio samples")
	flag.StringVar(&caseID, "case", "", "Specific case ID to process (optional)")
	flag.StringVar(&modelContext, "gt-analysis-model", "gemini-3-pro-preview", "Model for Context Generation (Step 1)")
	flag.StringVar(&modelEval, "eval-model", "gemini-3-flash-preview", "Model for Evaluation (Step 2)")
	var forceGeneration bool
	flag.BoolVar(&forceGeneration, "force", false, "Force regeneration of Context (Step 1)")
	flag.Parse()

	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY is not set")
	}

	ctx := context.Background()

	// Initialize google.golang.org/genai client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	generator := evalv2.NewGenerator(client, modelContext)
	evaluator := evalv2.NewEvaluator(client, modelEval)

	cases, err := findCases(datasetDir, caseID)
	if err != nil {
		log.Fatalf("Failed to scan cases: %v", err)
	}

	log.Printf("Found %d cases to process", len(cases))

	for _, c := range cases {
		processCase(ctx, c, generator, evaluator, forceGeneration)
	}
}

type Case struct {
	ID          string
	GroundTruth string            // Renamed from GTText
	CtxPath     string            // .gt.v2.json
	Transcripts map[string]string // Renamed from Evals
}

func findCases(root, targetID string) ([]*Case, error) {
	var cases []*Case
	caseMap := make(map[string]*Case)

	files, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			continue
		}

		parts := strings.Split(name, ".")
		if len(parts) < 2 {
			continue
		}
		id := parts[0]

		if targetID != "" && id != targetID {
			continue
		}

		if _, ok := caseMap[id]; !ok {
			caseMap[id] = &Case{ID: id, Transcripts: make(map[string]string)}
		}
		c := caseMap[id]

		if !strings.HasPrefix(name, id+".") {
			continue
		}

		ext := filepath.Ext(name)
		if ext == "" {
			continue
		}

		path := filepath.Join(root, name)

		// Derived path handling: Audio is [id].flac, GT file is [id].gt.json
		// We process them when identifying the specific extension.

		if ext == ".flac" || ext == ".mp3" || ext == ".wav" {
			// Derived path assumption, just ensure we skip adding this as a 'transcript'
			continue
		}

		if strings.Contains(name, ".gt.") || strings.Contains(name, ".eval.") || strings.Contains(name, ".report.") {
			if strings.HasSuffix(name, ".gt.json") {
				// Load GT Text
				if content, err := ioutil.ReadFile(path); err == nil {
					var gtObj struct {
						GroundTruth string `json:"ground_truth"`
					}
					if json.Unmarshal(content, &gtObj) == nil {
						c.GroundTruth = gtObj.GroundTruth
					}
				}
			} else if strings.HasSuffix(name, ".gt.v2.json") {
				c.CtxPath = path
			}
			continue
		}

		provider := strings.TrimPrefix(ext, ".")
		if provider == "txt" {
			parts := strings.Split(name, ".")
			if len(parts) == 3 {
				provider = parts[1]
			}
		}

		content, _ := ioutil.ReadFile(path)
		c.Transcripts[provider] = string(content)
	}

	for _, c := range caseMap {
		// We trust if GroundTruth is present, the case is valid enough to attempt (or check audio existence later)
		if c.GroundTruth != "" {
			cases = append(cases, c)
		}
	}
	return cases, nil
}

func processCase(ctx context.Context, c *Case, generator *evalv2.Generator, evaluator *evalv2.Evaluator, force bool) {
	log.Printf("Processing Case: %s", c.ID)

	// Derive Audio Path
	audioPath := filepath.Join(datasetDir, c.ID+".flac")

	// Step 1: Context Generation
	var contextResp *evalv2.ContextResponse
	ctxFile := filepath.Join(datasetDir, c.ID+".gt.v2.json")

	// Check if already exists and not forced
	shouldGenerate := true
	if !force {
		if _, err := os.Stat(ctxFile); err == nil {
			content, err := ioutil.ReadFile(ctxFile)
			if err == nil {
				if json.Unmarshal(content, &contextResp) == nil {
					log.Println(" > Loaded existing Context")
					shouldGenerate = false
				}
			}
		}
	}

	if shouldGenerate {
		log.Println(" > Generating Context...")
		var err error
		contextResp, err = generator.GenerateContext(ctx, audioPath, c.GroundTruth, c.Transcripts)
		if err != nil {
			log.Printf("ERROR Generating Context for %s: %v", c.ID, err)
			return
		}
		// Save
		bytes, _ := json.MarshalIndent(contextResp, "", "  ")
		ioutil.WriteFile(ctxFile, bytes, 0644)
		log.Println(" > Saved Context")
	}

	// Step 2: Evaluation
	log.Println(" > Running Evaluation...")
	evalResp, err := evaluator.Evaluate(ctx, contextResp, c.Transcripts)
	if err != nil {
		log.Printf("ERROR Evaluating %s: %v", c.ID, err)
		return
	}

	// Save Report V2
	reportFile := filepath.Join(datasetDir, c.ID+".report.v2.json")
	reportBytes, _ := json.MarshalIndent(evalResp, "", "  ")
	ioutil.WriteFile(reportFile, reportBytes, 0644)
	log.Println(" > Saved Report V2")

	// Convert to Report V1
	// Convert to Report V1
	reportV1 := llm.EvalReport{
		GroundTruth: c.GroundTruth,
		EvalResults: make(map[string]llm.EvalResult),
	}

	for provider, eval := range evalResp.Evaluations {
		// Formula: P^0.7 * S^0.3
		// Check for 0 values to avoid NaN/Invalid math if necessary?
		// Typically scores are 0.0-1.0. Pow(0, 0.3) is 0.
		pScore := eval.Metrics.PScore
		sScore := eval.Metrics.SScore

		// Ensure non-negative just in case
		if pScore < 0 {
			pScore = 0
		}
		if sScore < 0 {
			sScore = 0
		}

		finalScore := math.Pow(pScore, 0.7) * math.Pow(sScore, 0.3)

		// Round to 2 decimal places for cleaner output (optional but good for reports)
		finalScore = math.Round(finalScore*100) / 100

		reportV1.EvalResults[provider] = llm.EvalResult{
			Score:             finalScore,
			RevisedTranscript: eval.RevisedTranscript,
			Summary:           eval.Summary,
			Transcript:        eval.Transcript,
		}
	}

	// Save Report V1
	// Filename format: [id].gemini-3-flash-preview.report.json
	modelName := "gemini-3-flash-preview"
	reportFileV1 := filepath.Join(datasetDir, fmt.Sprintf("%s.%s.report.json", c.ID, modelName))

	bytesV1, _ := json.MarshalIndent(reportV1, "", "  ")
	err = ioutil.WriteFile(reportFileV1, bytesV1, 0644)
	if err != nil {
		log.Printf("ERROR writing V1 report for %s: %v", c.ID, err)
	} else {
		log.Printf(" > Saved Report V1: %s", filepath.Base(reportFileV1))
	}
}
