package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"asr-eval/pkg/volc/client"
	"asr-eval/pkg/volc/request"
	"asr-eval/pkg/volc/response"
)

func main() {
	// Define flags
	ctxFlag := flag.String("context", "", "Path to context JSON file or raw JSON string")
	extFlag := flag.String("ext", ".volc2", "Output file extension")
	concurrencyFlag := flag.Int("concurrency", 10, "Number of concurrent workers (max 50)")
	modelFlag := flag.String("model", "v2", "Model version: v1 (bigasr) or v2 (seedasr)")
	limitFlag := flag.Int("limit", 0, "Limit number of files to process (0 = no limit)")
	batchFlag := flag.String("batch", "", "Directory to scan for unprocessed files (batch mode)")
	realtimeFlag := flag.Bool("realtime", false, "Use realtime streaming API instead of nostream")
	flag.Parse()

	// Set model version
	request.SetModelVersion(*modelFlag)
	log.Printf("Using model version: %s", *modelFlag)

	// Set API mode (realtime vs nostream)
	var url string
	if *realtimeFlag {
		url = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async"
		request.SetEnableNonstream(true)
		request.SetResultType("single")
		log.Println("Using realtime streaming API (enable_nonstream=true, result_type=single)")
	} else {
		url = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream"
		request.SetEnableNonstream(false)
		request.SetResultType("full")
		log.Println("Using nostream API (enable_nonstream=false, result_type=full)")
	}

	// Optional: Allow overriding URL from env
	if u := os.Getenv("VOLC_URL"); u != "" {
		url = u
	}

	_ = godotenv.Load() // Load .env file if it exists

	// Validate Env vars are set
	if os.Getenv("VOLC_APPID") == "" || os.Getenv("VOLC_TOKEN") == "" {
		log.Fatal("Please set VOLC_APPID and VOLC_TOKEN environment variables in .env file or export them.")
	}

	// Read context string once
	var ctxString string
	if *ctxFlag != "" {
		if _, err := os.Stat(*ctxFlag); err == nil {
			bytes, err := ioutil.ReadFile(*ctxFlag)
			if err != nil {
				log.Fatalf("Failed to read context file: %v", err)
			}
			ctxString = string(bytes)
		} else {
			ctxString = *ctxFlag
		}
		log.Printf("Context payload: %s", ctxString)
	}

	var files []string
	args := flag.Args()

	if *batchFlag != "" {
		// Batch mode: scan directory for unprocessed files
		var err error
		files, err = getUnprocessedFlacFiles(*batchFlag, *extFlag, *limitFlag)
		if err != nil {
			log.Fatalf("Failed to scan directory: %v", err)
		}
		if len(files) == 0 {
			log.Println("No unprocessed files found")
			return
		}
	} else if len(args) > 0 {
		// Explicit file list
		files = args
	} else {
		flag.Usage()
		log.Fatal("Please specify files as arguments or use -batch <directory>")
	}

	// Limit concurrency to max 50
	concurrency := *concurrencyFlag
	if concurrency > 50 {
		concurrency = 50
	}
	if concurrency < 1 {
		concurrency = 1
	}

	log.Printf("Processing %d files with %d concurrent workers", len(files), concurrency)

	// Worker pool
	fileChan := make(chan string, len(files))
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// Each worker has its own client
			segDuration := 200
			c := client.NewAsrWsClient(url, segDuration)
			if ctxString != "" {
				c.SetContext(ctxString)
			}

			for file := range fileChan {
				processFile(c, file, *extFlag, *realtimeFlag)
			}
		}(i)
	}

	// Send files to workers
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	wg.Wait()
	log.Printf("Finished processing %d files", len(files))
}

func getUnprocessedFlacFiles(root string, ext string, limit int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".flac" {
			// Check if output file with specified extension exists
			outPath := strings.TrimSuffix(path, ".flac") + ext
			if _, err := os.Stat(outPath); os.IsNotExist(err) {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort alphabetically
	sort.Strings(files)

	// Apply limit if specified
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	return files, nil
}

type StreamEntry struct {
	Timestamp int64  `json:"t"`
	Final     bool   `json:"f,omitempty"`
	Text      string `json:"s"`
}

func processFile(c *client.AsrWsClient, filePath string, ext string, realtime bool) {
	fmt.Printf("Processing %s...\n", filePath)

	resChan := make(chan *response.AsrResponse)
	var wg sync.WaitGroup
	wg.Add(1)

	var finalTranscript string
	var mu sync.Mutex

	startTime := time.Now()

	go func() {
		defer wg.Done()

		var streamFile *os.File
		var err error
		if realtime {
			streamPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ext + ".stream.json"
			streamFile, err = os.Create(streamPath)
			if err != nil {
				log.Printf("Failed to create stream file %s: %v", streamPath, err)
			} else {
				defer streamFile.Close()
			}
		}

		for res := range resChan {
			if res.Code != 0 {
				fmt.Printf("Error response: Code=%d, Error=%s\n", res.Code, res.PayloadMsg.Error)
				return
			}
			if res.PayloadMsg != nil && res.PayloadMsg.Result.Text != "" {
				mu.Lock()
				if !realtime {
					finalTranscript = res.PayloadMsg.Result.Text
				}
				mu.Unlock()

				msg := res.PayloadMsg
				log.Printf("Updated transcript (len=%d). Utterances: %d", len(finalTranscript), len(msg.Result.Utterances))

				if streamFile != nil || realtime { // Process utterances for both stream file and final transcript accumulation
					currentUtterances := msg.Result.Utterances
					var partialParts []string

					for _, u := range currentUtterances {
						if u.Definite {
							// Finalized segment
							if u.Text != "" {
								if streamFile != nil {
									entry := StreamEntry{
										Timestamp: time.Since(startTime).Milliseconds(),
										Final:     true,
										Text:      u.Text,
									}
									line, _ := json.Marshal(entry)
									_, _ = streamFile.Write(line)
									_, _ = streamFile.WriteString("\n")
								}
								// Accumulate to final transcript in realtime mode
								mu.Lock()
								finalTranscript += u.Text
								mu.Unlock()

								log.Printf("Finalized utterance: %s", u.Text)
							}
						} else {
							// Active indefinite segment
							partialParts = append(partialParts, u.Text)
						}
					}

					// Stream aggregated partial text
					if streamFile != nil {
						partialText := strings.Join(partialParts, "")
						if partialText != "" {
							entry := StreamEntry{
								Timestamp: time.Since(startTime).Milliseconds(),
								Final:     false,
								Text:      partialText,
							}
							line, _ := json.Marshal(entry)
							_, _ = streamFile.Write(line)
							_, _ = streamFile.WriteString("\n")
						}
					}
				}
			}
		}
	}()

	err := c.Excute(context.Background(), filePath, resChan)
	if err != nil {
		fmt.Printf("Failed to process %s: %v\n", filePath, err)
		return
	}

	wg.Wait()

	if finalTranscript != "" {
		volcPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ext
		err := ioutil.WriteFile(volcPath, []byte(finalTranscript), 0644)
		if err != nil {
			fmt.Printf("Failed to write result to %s: %v\n", volcPath, err)
		} else {
			fmt.Printf("Saved result to %s\n", volcPath)
		}
	} else {
		// If empty, maybe it was silence or failed silently?
		// Don't overwrite if empty unless sure.
		fmt.Printf("No transcript received for %s\n", filePath)
	}
}
