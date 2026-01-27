package main

import (
	"context"
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

	"github.com/joho/godotenv"

	"asr-eval/pkg/qwen"
)

func main() {
	// Define flags
	ctxFlag := flag.String("context", "", "Path to context JSON file or raw JSON string (Context/Corpus)")
	extFlag := flag.String("ext", ".qwen", "Output file extension")
	concurrencyFlag := flag.Int("concurrency", 10, "Number of concurrent workers (max 50)")
	modelFlag := flag.String("model", "qwen3-asr-flash-realtime", "Model name (e.g. qwen-realtime-v1)")
	limitFlag := flag.Int("limit", 0, "Limit number of files to process (0 = no limit)")
	batchFlag := flag.String("batch", "", "Directory to scan for unprocessed files (batch mode)")
	flag.Parse()

	_ = godotenv.Load() // Load .env file if it exists

	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set QWEN_API_KEY environment variables.")
	}

	log.Printf("Using model: %s", *modelFlag)

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

	// Limit concurrency
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

			// New Client per worker or per file?
			// Since client holds connection, we create one per file usually, or reuse if client supports relogin.
			// My implementation of Client.ProcessFile does connect/disconnect.
			// So we can just create a new Client helper or reuse a factory.
			// Actually Client struct just holds config (model, key), and ProcessFile creates Conn.
			// So we can reuse Client struct.
			c := qwen.NewClient(*modelFlag, apiKey)

			for file := range fileChan {
				processFile(c, file, ctxString, *extFlag)
			}
		}(i)
	}

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

	sort.Strings(files)
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

func processFile(c *qwen.Client, filePath string, corpusText string, ext string) {
	fmt.Printf("Processing %s...\n", filePath)

	resChan := make(chan qwen.Result)
	var wg sync.WaitGroup
	wg.Add(1)

	var fullTranscript strings.Builder
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		for res := range resChan {
			if res.Error != nil {
				fmt.Printf("Error processing %s: %v\n", filePath, res.Error)
				return
			}

			// We only append Final results to the final transcript
			// But maybe we want to log partials?
			// cmd/processor logs partials.
			if res.IsFinal {
				mu.Lock()
				if fullTranscript.Len() > 0 {
					fullTranscript.WriteString(" ")
				}
				fullTranscript.WriteString(res.Text)
				mu.Unlock()
				log.Printf("[%s] Segment: %s", filepath.Base(filePath), res.Text)
			}
		}
	}()

	err := c.ProcessFile(context.Background(), filePath, corpusText, resChan)
	if err != nil {
		fmt.Printf("Failed to process %s: %v\n", filePath, err)
	}

	wg.Wait()

	finalStr := fullTranscript.String()
	if finalStr != "" {
		outPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ext
		err := ioutil.WriteFile(outPath, []byte(finalStr), 0644)
		if err != nil {
			fmt.Printf("Failed to write result to %s: %v\n", outPath, err)
		} else {
			fmt.Printf("Saved result to %s\n", outPath)
		}
	} else {
		// fmt.Printf("No transcript received for %s\n", filePath)
	}
}
