package main

import (
	"asr-eval/pkg/workspace"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

var (
	datasetDir       string
	genModelFlag     string
	evalModelFlag    string
	enabledProviders = map[string]bool{
		"volc":         false,
		"volc_ctx":     false,
		"volc_ctx_rt":  false,
		"volc2_ctx":    false,
		"volc2_ctx_rt": true,
		"qwen_ctx_rt":  true,
		"ifly":         true,
		"ifly_mq":      true,
		"ifly_en":      false,
		"iflybatch":    false,
		"dg":           true,
		"snx":          true,
		"snxrt":        true,
		"snxrt_v4":     true,
		"ist_basic":    true,
		"txt":          false,
	}
)

func main() {
	var port int
	flag.StringVar(&datasetDir, "dataset-dir", "transcripts_and_audios", "Directory containing transcripts and audio files")
	flag.StringVar(&genModelFlag, "gen-model", "gemini-3-pro-preview", "LLM model to use for context generation")
	flag.StringVar(&evalModelFlag, "eval-model", "gemini-3-flash-preview", "LLM model to use for evaluation")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.Parse()

	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Fatalf("Failed to init LLM client: %v", err)
	}

	config := workspace.ServiceConfig{
		GenModel:         genModelFlag,
		EvalModel:        evalModelFlag,
		EnabledProviders: enabledProviders,
	}
	svc := workspace.NewService(datasetDir, config, client)

	// Use Go 1.22+ ServeMux patterns if available, or just standard.
	// RegisterRoutes uses pattern matching like "GET /api/cases/{id}" which requires Go 1.22.
	// If user is on older Go, this might fail or treated as exact path.
	// Assuming Go 1.22 based on recent work.
	mux := http.NewServeMux()

	// Register API Routes
	svc.RegisterRoutes(mux)

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	// Handle root specially to fallback to index.html for SPA routing
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If it's an API call not matched above, it returns 404 naturally?
		// No, RegisterRoutes handles specific paths.
		// We need to serve static files.

		path := "./static" + r.URL.Path
		if _, err := os.Stat(path); err == nil && r.URL.Path != "/" {
			fs.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html
		http.ServeFile(w, r, "./static/index.html")
	})

	// Serve audio files
	audioFs := http.FileServer(http.Dir(datasetDir))
	mux.Handle("/audio/", http.StripPrefix("/audio/", audioFs))

	fmt.Printf("Attempting to listen on 127.0.0.1:%d...\n", port)
	fmt.Printf("Using dataset directory: %s\n", datasetDir)
	if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), mux); err != nil {
		log.Fatalf("Failed to bind to 127.0.0.1:%d: %v\n", port, err)
	}
}
