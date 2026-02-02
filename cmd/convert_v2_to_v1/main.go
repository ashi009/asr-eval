package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"asr-eval/pkg/evalv2"
	"asr-eval/pkg/llm"
)

func main() {
	datasetDir := flag.String("dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	modelName := flag.String("model-name", "gemini-3-flash-preview", "Model name to use in output filename")
	flag.Parse()

	files, err := ioutil.ReadDir(*datasetDir)
	if err != nil {
		log.Fatalf("Failed to read dir: %v", err)
	}

	convertedCount := 0
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".report.v2.json") {
			id := strings.TrimSuffix(name, ".report.v2.json")
			v2Path := filepath.Join(*datasetDir, name)
			v1Path := filepath.Join(*datasetDir, fmt.Sprintf("%s.%s.report.json", id, *modelName))

			// Read V2
			content, err := ioutil.ReadFile(v2Path)
			if err != nil {
				log.Printf("[%s] Failed to read V2 report: %v", id, err)
				continue
			}

			var v2Resp evalv2.EvalReport
			if err := json.Unmarshal(content, &v2Resp); err != nil {
				log.Printf("[%s] Failed to parse V2 report: %v", id, err)
				continue
			}

			// Map to V1
			v1Report := llm.EvalReport{
				GroundTruth: v2Resp.ContextSnapshot.Meta.GroundTruth,
				EvalResults: make(map[string]llm.EvalResult),
			}

			for provider, eval := range v2Resp.Results {
				v1Report.EvalResults[provider] = llm.EvalResult{
					Score:             eval.Metrics.SScore,
					RevisedTranscript: eval.RevisedTranscript,
					Summary:           eval.Summary,
					Transcript:        eval.Transcript,
				}
			}

			// Save V1
			bytes, _ := json.MarshalIndent(v1Report, "", "  ")
			if err := ioutil.WriteFile(v1Path, bytes, 0644); err != nil {
				log.Printf("[%s] Failed to save V1 report: %v", id, err)
			} else {
				convertedCount++
				log.Printf("[%s] Converted to V1", id)
			}
		}
	}

	log.Printf("Converted %d files.", convertedCount)
}
