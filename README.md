# ASR Evaluation Tool

A tool for evaluating and comparing Automatic Speech Recognition (ASR) services.

## Prerequisites

-   Go 1.22+
-   Node.js & npm (for UI development)

## Setup

1.  **Clone the repository**
2.  **Environment Variables**: Copy `.env.sample` to `.env` and fill in your keys.
    ```bash
    cp .env.sample .env
    ```

## Running the Server

Build and run the server:

```bash
go build -o server cmd/server/main.go
./server
```

### Dataset Configuration

By default, the server looks for transcripts and audio files in the `transcripts_and_audios` directory. You can specify a different directory using the `--dataset-dir` flag:

```bash
./server --dataset-dir=/path/to/your/dataset
```

## Running the UI (Development)

The UI is built with React/Vite.

```bash
cd ui
npm install
npm run dev
```

Build for production:

```bash
cd ui
npm run build
```
The build artifacts will be in `ui/dist` (packaged as `static` in the server).

## Project Structure

-   `cmd/`: Entry points for applications.
    -   `server/`: The main backend server.
    -   `processor/`, `qwen-processor/`: Data processing tools.
-   `pkg/`: Library code.
    -   `llm/`: LLM integration for evaluation.
    -   `volc/`, `qwen/`: ASR provider clients.
-   `ui/`: Frontend application.
-   `static/`: Compiled frontend assets.
