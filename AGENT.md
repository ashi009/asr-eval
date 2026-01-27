# ASR Evaluation Tool - Agent Context

This document provides context for AI agents working on this codebase.

## Architecture

The project consists of a Go backend and a React (Vite) frontend.

-   **Backend**: specific HTTP handlers in `cmd/server/main.go` serve the API and static files.
    -   `/api/cases`: Lists available cases (audio/transcript pairs).
    -   `/api/case`: Retrieves details for a specific case.
    -   `/api/evaluate`: Saves human evaluation results.
    -   `/api/evaluate-llm`: Triggers LLM-based evaluation.
-   **Frontend**: A Single Page Application (SPA) in `ui/`.
    -   `App.jsx`: Main logic.
    -   `config.js`: Service configuration (names, colors).

## Key Workflows

1.  **Listing Cases**: Scans the dataset directory (default: `transcripts_and_audios`) for `.flac` files and corresponding results.
2.  **Evaluation**:
    -   **Human**: User inputs ground truth and comments. Saved to `[ID].eval.json`.
    -   **LLM**: User triggers LLM eval. Ground truth is saved, then `llm.Evaluator` processes results. Saved to `[ID].result.json`.

## Configuration

-   The dataset directory is configurable via the `--dataset-dir` flag in the server.
-   Secrets are loaded from `.env` using `godotenv`.

## Development Guidelines

-   **Backend**: When modifying API handlers, ensure `cmd/server/main.go` logic aligns with `pkg/` definitions.
-   **Frontend**: `ui/` contains source code. `npm run build` updates `static/`.
-   **Dataset**: The dataset is NOT checked into git. Always respect the `.gitignore`.
