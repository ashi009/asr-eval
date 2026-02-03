package main

import (
	"asr-eval/pkg/evalv2"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

func main() {
	var datasetDir string
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	flag.Parse()

	// Provider stats
	type providerStats struct {
		WeightedSum float64 // Q Score sum
		WeightedS   float64 // S Score sum
		WeightedP   float64 // P Score sum
		TotalTokens int
		Count       int
	}
	stats := make(map[string]*providerStats)

	// Walk through the dataset directory
	err := filepath.Walk(datasetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Look for .report.v2.json files
		if strings.HasSuffix(info.Name(), ".report.v2.json") {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				log.Printf("Error reading file %s: %v", path, err)
				return nil
			}

			var report evalv2.EvalReport
			if err := json.Unmarshal(content, &report); err != nil {
				log.Printf("Error parsing JSON in %s: %v", path, err)
				return nil
			}

			// Get token count estimate
			tokenCount := report.ContextSnapshot.Meta.TotalTokenCountEstimate
			if tokenCount <= 0 {
				// Fallback: Try to read from [id].gt.v2.json
				gtFilename := strings.Replace(info.Name(), ".report.v2.json", ".gt.v2.json", 1)
				gtPath := filepath.Join(filepath.Dir(path), gtFilename)
				if gtContent, err := ioutil.ReadFile(gtPath); err == nil {
					var ctx evalv2.EvalContext
					if err := json.Unmarshal(gtContent, &ctx); err == nil {
						tokenCount = ctx.Meta.TotalTokenCountEstimate
					}
				}
			}

			if tokenCount <= 0 {
				// fmt.Printf("DEBUG: Skipping %s (TokenCount=%d)\n", info.Name(), tokenCount)
				return nil
			}

			for provider, result := range report.Results {
				// Calculate QScore if not present (CompositeScore calculates it)
				qScore := result.Metrics.CompositeScore()

				if _, ok := stats[provider]; !ok {
					stats[provider] = &providerStats{}
				}
				s := stats[provider]
				// Accumulate Q (0-100)
				s.WeightedSum += float64(qScore) * float64(tokenCount)
				// Accumulate S and P (convert 0.0-1.0 to 0-100 for consistency)
				s.WeightedS += result.Metrics.SScore * 100 * float64(tokenCount)
				s.WeightedP += result.Metrics.PScore * 100 * float64(tokenCount)

				s.TotalTokens += tokenCount
				s.Count++
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}

	// Output results
	fmt.Printf("Weighted Q Scores (Dataset: %s)\n", datasetDir)
	fmt.Println("--------------------------------------------------")

	// Sort providers for consistent output
	var providers []string
	for p := range stats {
		providers = append(providers, p)
	}
	sort.Slice(providers, func(i, j int) bool {
		// Sort by weighted average descending? or name?
		// Let's sort by score descending for better utility
		scoreI := stats[providers[i]].WeightedSum / float64(stats[providers[i]].TotalTokens)
		scoreJ := stats[providers[j]].WeightedSum / float64(stats[providers[j]].TotalTokens)
		return scoreI > scoreJ
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Provider\tWeighted Q\tWeighted S\tWeighted P\tTotal Tokens\tCases")

	for _, p := range providers {
		s := stats[p]
		weightedQ := 0.0
		weightedS := 0.0
		weightedP := 0.0
		if s.TotalTokens > 0 {
			weightedQ = s.WeightedSum / float64(s.TotalTokens)
			weightedS = s.WeightedS / float64(s.TotalTokens)
			weightedP = s.WeightedP / float64(s.TotalTokens)
		}
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%d\t%d\n", p, weightedQ, weightedS, weightedP, s.TotalTokens, s.Count)
	}
	w.Flush()
}
