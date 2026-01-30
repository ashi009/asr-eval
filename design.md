# ASR Eval Pro - Design Document

## 1. Overview
ASR Eval Pro is a full-stack web application designed to evaluate the performance of various Automatic Speech Recognition (ASR) providers against a Ground Truth (GT). It utilizes Large Language Models (LLMs) (specifically Google Gemini) to analyze transcripts, identifying errors and generating quality scores and qualitative feedback.

## 2. Architecture

### 2.1 Tech Stack
-   **Frontend**: React (Vite), TypeScript, TailwindCSS, Lucide React (Icons), React Router.
-   **Backend**: Go (Golang), standard library `net/http` for API, `google.golang.org/genai` for AI integration.
-   **Data Storage**: Local file system (JSON for data, FLAC for audio). No external database.
-   **AI Integration**: Google GenAI SDK (Gemini models) for performing evaluations.

### 2.2 System Components
1.  **UI Client (Single Page App)**:
    -   Communicates with the backend via REST API.
    -   Handles file playback, text editing (Ground Truth), and result visualization.
2.  **Backend Server**:
    -   Serves the static frontend assets.
    -   Exposes JSON endpoints for case management (`/api/cases`, `/api/case`).
    -   Orchestrates the evaluation process (`/api/evaluate-llm`, `/api/evaluate-v2`).
    -   Manages file operations (saving GT, reading transcripts).

## 3. Data Model & File Structure

The application relies on a strict naming convention within a flat directory (default: `transcripts_and_audios`).

### 3.1 File Types
For a given Case ID (e.g., `21dbbe8a`):

| File Type | Pattern | Description |
| :--- | :--- | :--- |
| **Audio** | `[id].flac` | The source audio file. |
| **Transcript** | `[id].[provider].txt` | Raw transcript text from a provider (e.g., `volcengine`). |
| **Ground Truth** | `[id].gt.json` | JSON file containing the user-verified ground truth text. |
| **V1 Report** | `[id].[model].report.json` | Result of V1 evaluation (Scores, Assessment, Revised Transcript). |
| **V2 Context** | `[id].gt.v2.json` | (V2) Generated context/checkpoints derived from GT and Audio. |
| **V2 Report** | `[id].report.v2.json` | (V2) Result of V2 evaluation against the context. |

### 3.2 Data Schemas (JSON)

**EvalReport (V1)**:
```json
{
  "ground_truth": "Expected text...",
  "eval_results": {
    "provider_name": {
      "score": 0.95,
      "transcript": "Original transcript snapshot...",
      "revised_transcript": "Corrected transcript by AI...",
      "summary": ["Analysis point 1", "Analysis point 2"]
    }
  }
}
```

**ContextResponse (V2)**:
```json
{
  "meta": { "business_goal": "...", "ground_truth": "..." },
  "checkpoints": [
    { "id": "1", "text_segment": "...", "tier": 1, "weight": 5, "rationale": "..." }
  ]
}
```

## 4. Workflows

### 4.1 Case Loading
1.  Frontend requests `/api/cases`.
2.  Backend scans the directory for `.flac` files and corresponding `.report.json` files to determine "Done" status and "Best Performers".
3.  Frontend separates cases into "Pending" (no AI eval) and "Done".

### 4.2 Evaluation Process (V1)
1.  User selects one or more providers in the UI.
2.  User clicks "Run AI Eval".
3.  Frontend sends Ground Truth + Selected Transcripts to `/api/evaluate-llm`.
4.  Backend calls LLM to compare Transcripts vs GT.
5.  Results are merged into `[id].[model].report.json` (partial updates supported).
6.  Frontend reloads to display new scores.

### 4.3 Ground Truth Management
-   **Editing**: Users can edit GT in the `CaseDetail` view (`textarea`). Auto-saves to `[id].gt.json`.
-   **Drift Detection**: If GT is edited *after* an evaluation, the valid Ground Truth for the report (`eval_report.ground_truth`) differs from the current Ground Truth.
-   **Resolution**: A "Review" modal appears, showing a diff. Users can either:
    -   **Revert**: Restore GT to the version used in the evaluation.
    -   **Reset Eval**: Discard the old evaluation to start fresh with new GT.

## 5. UI Design

### 5.1 Design System
-   **Typography**: `font-display` (Sans-serif) for UI, `font-mono` for Code/Transcripts/IDs.
-   **Color Palette**:
    -   **Neutral**: Slate (50-900) for structural elements.
    -   **Primary**: Indigo/Blue for actions and accents.
    -   **Status**:
        -   Green (90-100 score, Ground Truth).
        -   Amber/Orange (70-89 score, Warnings, Stale states).
        -   Red (<70 score).

### 5.2 Layout
**Sidebar**:
-   Search bar at top.
-   Two lists: "Pending" and "Done".
-   "Done" items display "badges" for the winning providers (highest score).
-   Responsive: Collapses to a hamburger menu on small screens.

**Main View (`CaseDetail`)**:
-   **Playback Bar**: Sticky at bottom (or top section), waveform control, Play/Pause.
-   **Ground Truth Section**: Collapsible text area. Shows "Review" warning if stale.
-   **Report View**: A tabular grid.
    -   **Columns**: Selection Checkbox | Provider/Score | Transcript (Diff) | Analysis.
    -   **Visuals**:
        -   Score visualization (large numbers + color coding).
        -   **Diff Rendering**: Highlights insertions (green/underline) and deletions (red/strikethrough).
        -   **Diff Modes**:
            -   *Origin vs Revised*: What the AI corrected.
            -   *Origin vs New*: How the file on disk differs from the evaluation snapshot.
            -   *New vs Revised*: Gap between current file and "ideal" AI output.

## 6. Current Feature Set
-   **Multi-Provider Support**: Compare unlimited providers side-by-side.
-   **Staleness Checks**: Automatically detects if the transcript file on disk has changed since the last evaluation (flagged as "Stale").
-   **Search**: Client-side filtering by Case ID.
-   **Playback**: HTML5 Audio with seekable progress bar.
-   **Configurable LLM**: Backend flag `-llm-model` sets the model used (e.g., `gemini-1.5-pro`).
-   **Ranking**: Automatically highlights the "Best Performer" in the sidebar list.

## 7. Future Considerations (For Stitch)
This document serves as a baseline. Future improvements may include:
-   **Batch Processing**: Run eval for all pending cases in one click.
-   **V2 UI Integration**: Fully expose the "Checkpoint" and "Context" (V2) logic in the UI (currently backend-only support).
-   **Upload Interface**: Allow uploading new audio/transcripts via UI (currently manual file placement).
-   **Waveform Visualization**: Replace simple progress bar with true waveform header.
-   **User Auth**: Multi-user support if deployed centrally.
