package main

import (
	"asr-eval/pkg/workspace"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

var (
	cfg               = workspace.DefaultServiceConfig()
	concurrency       = 10
	defaultGTProvider = "txt"
)

func main() {
	flag.StringVar(&cfg.DatasetDir, "dataset-dir", cfg.DatasetDir, "Directory containing transcripts and audio files")
	flag.StringVar(&cfg.GenModel, "gen-model", cfg.GenModel, "LLM model to use for context generation")
	flag.StringVar(&cfg.EvalModel, "eval-model", cfg.EvalModel, "LLM model to use for evaluation")
	flag.IntVar(&concurrency, "concurrency", concurrency, "Number of concurrent workers (applied to both pools)")
	flag.StringVar(&defaultGTProvider, "default-gt-provider", defaultGTProvider, "Provider ID to use as initial Ground Truth")
	flag.Parse()

	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Fatalf("Failed to init LLM client: %v", err)
	}

	svc := workspace.NewService(cfg, client)
	ctx := context.Background()

	cases, err := svc.ListCases(ctx)
	if err != nil {
		log.Fatalf("Failed to list cases: %v", err)
	}

	fmt.Printf("Found %d cases. Starting pipeline with concurrency %d for both Gen and Eval...\n", len(cases), concurrency)

	// Channels for the pipeline
	// buffer size = len(cases) to avoid blocking the scanner
	genQueue := make(chan *workspace.Case, len(cases))
	evalQueue := make(chan *workspace.Case, len(cases))

	var wgGen sync.WaitGroup
	var wgEval sync.WaitGroup

	// Start Evaluation Workers
	for i := 0; i < concurrency; i++ {
		wgEval.Add(1)
		go func() {
			defer wgEval.Done()
			for c := range evalQueue {
				processEvaluation(ctx, svc, c)
			}
		}()
	}

	// Start Generation Workers
	for i := 0; i < concurrency; i++ {
		wgGen.Add(1)
		go func() {
			defer wgGen.Done()
			for c := range genQueue {
				// Process Generation checks/actions
				// If successful (or no gen needed), pass to Eval Queue
				if updatedC, ok := processGeneration(ctx, svc, c); ok {
					evalQueue <- updatedC
				}
			}
		}()
	}

	// Feed the pipeline
	for _, c := range cases {
		genQueue <- c
	}
	close(genQueue)

	// Wait for Generation to finish
	wgGen.Wait()
	// Close Eval Queue as no more items will be produced
	close(evalQueue)
	// Wait for Evaluation to finish
	wgEval.Wait()

	fmt.Println("Batch execution complete.")
}

// processGeneration returns the (potentially updated) case and true if the case is ready for evaluation
func processGeneration(ctx context.Context, svc *workspace.Service, c *workspace.Case) (*workspace.Case, bool) {
	// Optimization: Only fetch full case if we actually need to generate context.
	// We rely on ListCases providing a popualted EvalContext (if it exists).

	needsContextGen := false
	var groundTruth string
	var source string

	if c.EvalContext == nil {
		// Case A: Missing Context
		// We need to check if default provider exists.
		// Since 'c' is lightweight, it might not have transcripts.
		// We MUST fetch full case here to check transcripts.
		fullCase, err := svc.GetCase(ctx, c.ID)
		if err != nil {
			log.Printf("[%s] Failed to get full case details: %v", c.ID, err)
			return nil, false
		}

		if gt, ok := fullCase.Transcripts[defaultGTProvider]; ok {
			groundTruth = gt
			needsContextGen = true
			source = "default_provider (" + defaultGTProvider + ")"
		} else {
			log.Printf("[%s] Skipping context gen: default GT provider '%s' not found", c.ID, defaultGTProvider)
			// Cannot evaluate if no context
			return nil, false
		}
	} else if c.EvalContext.Meta.QuestionableGT {
		// Case B: Questionable GT
		// We have EvalContext, so we can check fields directly.
		if c.EvalContext.Meta.AudioRealityInference != "" {
			groundTruth = c.EvalContext.Meta.AudioRealityInference
			needsContextGen = true
			source = "audio_reality_inference"
		} else {
			log.Printf("[%s] Skipping context regen: Questionable GT but no Audio Reality Inference", c.ID)
			// We can still proceed to evaluate with the existing (questionable) context.
		}
	}

	if needsContextGen {
		fmt.Printf("[%s] Generating Context (Source: %s)...\n", c.ID, source)
		req := workspace.GenerateContextRequest{
			ID:          c.ID,
			GroundTruth: groundTruth,
		}
		newCtx, err := svc.GenerateContext(ctx, req)
		if err != nil {
			log.Printf("[%s] Failed to generate context: %v", c.ID, err)
			return nil, false
		}

		// Save Context
		updateReq := workspace.UpdateContextRequest{
			ID:          c.ID,
			EvalContext: newCtx,
		}
		updatedCase, err := svc.UpdateContext(ctx, updateReq)
		if err != nil {
			log.Printf("[%s] Failed to save context: %v", c.ID, err)
			return nil, false
		}
		fmt.Printf("[%s] Context saved.\n", c.ID)
		return updatedCase, true
	}

	return c, true
}

func processEvaluation(ctx context.Context, svc *workspace.Service, c *workspace.Case) {
	// We use 'c' directly. It should have EvalContext.
	// Note: svc.Evaluate will still fetch Transcripts internally (GetCase).
	// But we avoid fetching here in main.

	if c.EvalContext == nil {
		// Should not happen given logic in main/processGen, but safe guard
		log.Printf("[%s] Skipping evaluation: No EvalContext", c.ID)
		return
	}

	enabledProviders := make([]string, 0)
	for p, enabled := range svc.Config.EnabledProviders {
		if enabled {
			enabledProviders = append(enabledProviders, p)
		}
	}

	if len(enabledProviders) > 0 {
		fmt.Printf("[%s] Evaluating providers: %v...\n", c.ID, enabledProviders)
		evalReq := workspace.EvaluateRequest{
			ID:          c.ID,
			EvalContext: c.EvalContext,
			ProviderIDs: enabledProviders,
		}
		_, err := svc.Evaluate(ctx, evalReq)
		if err != nil {
			log.Printf("[%s] Failed to evaluate: %v", c.ID, err)
		} else {
			fmt.Printf("[%s] Evaluation complete.\n", c.ID)
		}
	}
}
