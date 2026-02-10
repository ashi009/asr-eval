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

func main() {
	var (
		cfg  = workspace.DefaultServiceConfig()
		port = 8080
	)

	flag.StringVar(&cfg.DatasetDir, "dataset-dir", cfg.DatasetDir, "Directory containing transcripts and audio files")
	flag.StringVar(&cfg.GenModel, "gen-model", cfg.GenModel, "LLM model to use for context generation")
	flag.StringVar(&cfg.EvalModel, "eval-model", cfg.EvalModel, "LLM model to use for evaluation")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.Parse()

	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Fatalf("Failed to init LLM client: %v", err)
	}

	svc := workspace.NewService(cfg, client)

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
	audioFs := http.FileServer(http.Dir(cfg.DatasetDir))
	mux.Handle("/audio/", http.StripPrefix("/audio/", audioFs))

	fmt.Printf("Attempting to listen on 127.0.0.1:%d...\n", port)
	fmt.Printf("Using dataset directory: %s\n", cfg.DatasetDir)
	if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), mux); err != nil {
		log.Fatalf("Failed to bind to 127.0.0.1:%d: %v\n", port, err)
	}
}
