package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"asr-eval/pkg/evalv2"
)

func main() {
	datasetDir := flag.String("dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	llmModel := flag.String("llm-model", "gemini-2.0-flash-exp", "LLM model to use")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent workers")
	flag.Parse()

	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}

	// We need a separate client per worker if the client is not thread-safe,
	// or share it if it is. documentation says: "Clients are safe for concurrent use by multiple goroutines."
	// So we can share one client.
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	files, err := ioutil.ReadDir(*datasetDir)
	if err != nil {
		log.Fatalf("Failed to read dir: %v", err)
	}

	var mu sync.Mutex
	var questionable []string
	flacFiles := []string{}
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".flac" {
			flacFiles = append(flacFiles, f.Name())
		}
	}

	log.Printf("Found %d audio files", len(flacFiles))

	// Job channel
	paramsChan := make(chan string, len(flacFiles))
	var wg sync.WaitGroup

	// Worker function
	worker := func(id int) {
		defer wg.Done()
		// Re-create generator per worker if needed, but client is shared.
		// Actually generator just holds client, so it should be fine.
		// But to be safe and clean, let's just make one generator per worker or share it.
		// Generator struct in types.go: type Generator struct { client *genai.Client; model string }
		// Read-only except internal state of client which is thread safe.
		localGenerator := evalv2.NewEvaluator(client, *llmModel, "")

		for flacName := range paramsChan {
			id := strings.TrimSuffix(flacName, ".flac")
			reportPath := filepath.Join(*datasetDir, id+".report.v2.json")
			ctxPath := filepath.Join(*datasetDir, id+".gt.v2.json")

			// Check if already evaled (skip if report exists)
			if _, err := os.Stat(reportPath); err == nil {
				// Skip
				continue
			}

			log.Printf("[%s] Generating Context...", id)

			// 1. Get Ground Truth
			gt := ""
			gtPath := filepath.Join(*datasetDir, id+".gt.json")
			if content, err := ioutil.ReadFile(gtPath); err == nil {
				var gtObj struct {
					GroundTruth string `json:"ground_truth"`
				}
				if err := json.Unmarshal(content, &gtObj); err == nil {
					gt = gtObj.GroundTruth
				}
			}

			// Fallback: use .txt as gt
			if gt == "" {
				txtPath := filepath.Join(*datasetDir, id+".txt")
				if content, err := ioutil.ReadFile(txtPath); err == nil {
					gt = string(content)
					log.Printf("[%s] Used .txt as GT", id)
				}
			}

			// 2. Gather Transcripts
			transcripts := make(map[string]string)
			// Scan directory for this ID's transcripts
			// This scan is inefficient inside the loop if "files" list is huge.
			// But we already have "files" read once. "files" slice is available in closure?
			// Yes, but accessing "files" slice concurrently is read-only so fine.
			for _, f := range files {
				name := f.Name()
				if strings.HasPrefix(name, id+".") && strings.HasSuffix(name, ".txt") {
					rest := strings.TrimPrefix(name, id+".")
					if rest == "txt" {
						continue // handled above
					}
					provider := strings.TrimSuffix(rest, ".txt")
					content, err := ioutil.ReadFile(filepath.Join(*datasetDir, name))
					if err == nil {
						transcripts[provider] = string(content)
					}
				}
			}

			if gt == "" && len(transcripts) > 0 {
				for _, v := range transcripts {
					gt = v
					log.Printf("[%s] No GT found, using first transcript as GT", id)
					break
				}
			}

			if gt == "" {
				log.Printf("[%s] SKIPPING - No GT and no transcripts found", id)
				continue
			}

			// 3. Generate
			resp, usage, err := localGenerator.GenerateContext(context.Background(), filepath.Join(*datasetDir, flacName), gt, transcripts)
			if err != nil {
				log.Printf("[%s] ERROR: %v", id, err)
				continue
			}

			if usage != nil {
				log.Printf("[%s] Usage: %d tokens", id, usage.TotalTokenCount)
			}

			// 4. Save
			bytes, _ := json.MarshalIndent(resp, "", "  ")
			if err := ioutil.WriteFile(ctxPath, bytes, 0644); err != nil {
				log.Printf("[%s] Failed to save context: %v", id, err)
			} else {
				log.Printf("[%s] Saved context", id)
			}

			// 5. Check Questionable
			if resp.Meta.QuestionableGT {
				msg := fmt.Sprintf("[%s] %s", id, resp.Meta.QuestionableReason)
				mu.Lock()
				questionable = append(questionable, msg)
				mu.Unlock()
				log.Println("!!! " + msg)
			}
		}
	}

	// Start workers
	log.Printf("Starting %d workers...", *concurrency)
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	// Feed jobs
	for _, flacName := range flacFiles {
		paramsChan <- flacName
	}
	close(paramsChan)

	// Wait
	wg.Wait()

	fmt.Println("\n=== Targets with Questionable GTs ===")
	// no sort built-in for []string, but simple print is fine
	if len(questionable) == 0 {
		fmt.Println("None found.")
	} else {
		for _, q := range questionable {
			fmt.Println(q)
		}
	}
}
