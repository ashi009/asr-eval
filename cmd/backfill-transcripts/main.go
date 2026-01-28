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
)

type EvalResult struct {
	Score              float64  `json:"score"`
	RevisedTranscript  string   `json:"revised_transcript"`
	Summary            []string `json:"summary"`
	OriginalTranscript string   `json:"original_transcript"`
}

type AIResultFile struct {
	EvaluatedGroundTruth string                `json:"evaluated_ground_truth"`
	Results              map[string]EvalResult `json:"results"`
}

func main() {
	datasetDir := flag.String("dataset-dir", "transcripts_and_audios", "Directory containing transcripts and result files")
	flag.Parse()

	log.Printf("Starting backfill migration in %s", *datasetDir)

	err := filepath.Walk(*datasetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".result.json") {
			return nil
		}

		// Parse ID from filename: [id].[model].result.json or [id].result.json
		// We can get ID by splitting by first dot
		parts := strings.Split(info.Name(), ".")
		if len(parts) < 2 {
			log.Printf("Skipping malformed filename: %s", info.Name())
			return nil
		}
		id := parts[0]

		// Read result file
		content, err := ioutil.ReadFile(path)
		if err != nil {
			log.Printf("Error reading %s: %v", path, err)
			return nil
		}

		var data AIResultFile
		if err := json.Unmarshal(content, &data); err != nil {
			// Might be legacy format (map directly), but purely map format usually doesn't have "results" key wrapper?
			// The current server uses AIResultFile struct wrapper.
			// Let's try map-only for safety
			var legacyResults map[string]EvalResult
			if err2 := json.Unmarshal(content, &legacyResults); err2 == nil {
				data.Results = legacyResults
			} else {
				log.Printf("Error unmarshaling %s: %v", path, err)
				return nil
			}
		}

		if data.Results == nil {
			return nil
		}

		modified := false
		for provider, result := range data.Results {
			if strings.TrimSpace(result.OriginalTranscript) == "" {
				// Search for transcript file: [id].[provider]
				// e.g. [id].txt, [id].iflybatch
				transcriptPath := filepath.Join(*datasetDir, fmt.Sprintf("%s.%s", id, provider))

				// Check for existence
				if _, err := os.Stat(transcriptPath); err == nil {
					// Read transcript
					tsContent, err := ioutil.ReadFile(transcriptPath)
					if err == nil {
						log.Printf("Backfilling %s (provider: %s) from %s", info.Name(), provider, transcriptPath)
						result.OriginalTranscript = strings.TrimSpace(string(tsContent))
						data.Results[provider] = result
						modified = true
					} else {
						log.Printf("Error reading transcript file %s: %v", transcriptPath, err)
					}
				} else {
					// Try with .txt extension if provider doesn't match extension directly?
					// Actually user instructions imply "corresponding source transcripts".
					// existing code in main.go implies extension matches provider.
					// e.g. if provider is "volc", file is "id.volc".
					// Just logging if not found for now.
					// log.Printf("Transcript file not found: %s", transcriptPath)
				}
			}
		}

		if modified {
			// Write back
			newContent, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				log.Printf("Error marshaling updated data for %s: %v", path, err)
				return nil
			}
			if err := ioutil.WriteFile(path, newContent, 0644); err != nil {
				log.Printf("Error writing updated file %s: %v", path, err)
			} else {
				log.Printf("Successfully updated %s", info.Name())
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
	log.Println("Backfill migration completed.")
}
