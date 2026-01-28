# ASR Evaluation Tool - Agent Context

This document provides context for AI agents working on this codebase.

## Architecture

The project consists of a Go backend and a React (Vite) frontend.

-   **Backend**: specific HTTP handlers in `cmd/server/main.go` serve the API and static files.
    -   `/api/cases`: Lists available cases (audio/transcript pairs).
    -   `/api/case`: Retrieves details for a specific case.
    -   `/api/evaluate`: Saves human evaluation results.
    -   `/api/evaluate-llm`: Triggers LLM-based evaluation.
    -   `/api/config`: Exposes server configuration (e.g., current LLM model).
    -   `/api/reset-eval`: Deletes evaluation results for the current model.
-   **Frontend**: A Single Page Application (SPA) in `ui/`.
    -   `App.tsx`: Main logic.
    -   `config.ts`: Service configuration (names, colors).

## Key Workflows

1.  **Listing Cases**: Scans the dataset directory for `.flac` files and corresponding results.
    -   **Filtering**: The server STRICTLY filters results to only show those matching the active LLM model.
2.  **Evaluation**:
    -   **Human**: User inputs ground truth and comments. Saved to `[ID].eval.json`.
    -   **LLM**: User triggers LLM eval. Saved to `[ID].[MODEL].result.json` (e.g., `id.gemini-2.5-flash.result.json`).
3.  **Reset**:
    -   Users can reset (delete) results for the *current* model via the UI.

## Configuration

-   The dataset directory is configurable via the `--dataset-dir` flag in the server.
-   **LLM Model**: Must be specified via `--llm-model` (e.g., `./server -llm-model gemini-2.5-flash`).
-   Secrets are loaded from `.env` using `godotenv`.

## Development Guidelines

-   **Backend**: When modifying API handlers, ensure `cmd/server/main.go` logic aligns with `pkg/` definitions.
-   **Frontend**: `ui/` contains source code. `npm run build` updates `static/`.
-   **Dataset**: The dataset is NOT checked into git. Always respect the `.gitignore`.
