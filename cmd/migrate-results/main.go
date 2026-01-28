package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"asr-eval/pkg/llm"
)

// LegacyAIResultFile represents the old file structure
type LegacyAIResultFile struct {
	EvaluatedGroundTruth string                    `json:"evaluated_ground_truth"`
	Results              map[string]llm.EvalResult `json:"results"`
	EvaluatedTranscripts map[string]string         `json:"evaluated_transcripts"`
}

// NewAIResultFile represents the new file structure (same as server but defined here for clarity)
type NewAIResultFile struct {
	EvaluatedGroundTruth string                    `json:"evaluated_ground_truth"`
	Results              map[string]llm.EvalResult `json:"results"`
}

func main() {
	datasetDir := flag.String("dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	targetModel := flag.String("target-model", "doubao-seed-1-8-251228", "Full model name to assign to migrated results")
	flag.Parse()

	log.Printf("Starting migration in %s (Target Model: %s)", *datasetDir, *targetModel)

	err := filepath.Walk(*datasetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		filename := info.Name()
		if !strings.HasSuffix(filename, ".result.json") || strings.HasSuffix(filename, ".bak") {
			return nil
		}

		parts := strings.Split(filename, ".")
		if len(parts) != 3 {
			return nil
		}

		id := parts[0]
		log.Printf("Migrating %s (ID: %s)...", filename, id)

		content, err := ioutil.ReadFile(path)
		if err != nil {
			log.Printf("Error reading %s: %v", path, err)
			return nil
		}

		var legacy LegacyAIResultFile
		var results map[string]llm.EvalResult
		var evaluatedTranscripts map[string]string
		var groundTruth string

		if err := json.Unmarshal(content, &legacy); err == nil && legacy.Results != nil {
			results = legacy.Results
			evaluatedTranscripts = legacy.EvaluatedTranscripts
			groundTruth = legacy.EvaluatedGroundTruth
		} else {
			// Try map-only format
			if err := json.Unmarshal(content, &results); err != nil {
				log.Printf("Error unmarshaling %s as map: %v", path, err)
				return nil
			}
		}

		if len(results) == 0 {
			log.Printf("No results in %s, skipping", path)
			return nil
		}

		// Try to find ground truth in .eval.json if missing
		if groundTruth == "" {
			evalPath := filepath.Join(*datasetDir, id+".eval.json")
			if evalContent, err := ioutil.ReadFile(evalPath); err == nil {
				var evalData struct {
					GroundTruth string `json:"ground_truth"`
				}
				if err := json.Unmarshal(evalContent, &evalData); err == nil {
					groundTruth = evalData.GroundTruth
				}
			}
		}

		// Convert to new format and populate OriginalTranscript
		newResults := make(map[string]llm.EvalResult)
		for provider, v := range results {
			// 1. Try from legacy EvaluatedTranscripts map
			if transcript, ok := evaluatedTranscripts[provider]; ok && transcript != "" {
				v.OriginalTranscript = transcript
			}

			// 2. If still empty, try to find [id].[provider] file
			if v.OriginalTranscript == "" {
				ext := provider
				// Heuristic: if provider is "txt", it's likely [id].txt
				// If provider is "volc", it's [id].volc
				transcriptPath := filepath.Join(*datasetDir, id+"."+ext)
				if content, err := ioutil.ReadFile(transcriptPath); err == nil {
					v.OriginalTranscript = string(content)
				} else if ext == "txt" {
					// Sometimes it might just be [id].txt but provider is something else?
					// Usually "txt" key means [id].txt
				}
			}

			newResults[provider] = v
		}

		newFile := NewAIResultFile{
			EvaluatedGroundTruth: groundTruth,
			Results:              newResults,
		}

		newFilename := fmt.Sprintf("%s.%s.result.json", id, *targetModel)
		newPath := filepath.Join(*datasetDir, newFilename)

		// Save new file
		data, _ := json.MarshalIndent(newFile, "", "  ")
		if err := ioutil.WriteFile(newPath, data, 0644); err != nil {
			log.Printf("Error writing %s: %v", newPath, err)
			return nil
		}

		// Rename old file to .bak
		bakPath := path + ".bak"
		if err := os.Rename(path, bakPath); err != nil {
			log.Printf("Error backing up %s: %v", path, err)
		}

		log.Printf("Migrated %s -> %s", filename, newFilename)
		return nil
	})

	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Printf("Migration completed.")
}
