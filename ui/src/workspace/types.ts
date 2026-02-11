export interface Case {
  id: string;
  // Computed fields (from backend or client-side logic?)
  // Backend types.go has HasAI, QuestionableGT.
  // Wait, I removed them from backend types.go!
  // Implementation Plan said: "Ensure EvalContext and ReportV2 are sufficient for frontend to derive this state."
  // So I need to derive them in the frontend or add them back if derivation is too complex for list view.
  // In `service.go`, I removed them from `Case` struct.
  // So `fetchCases` returns cases without `has_ai`.
  // I need to add derivation logic in `Case` or a wrapper.
  // Let's add optional fields to interface matching what we want to use, but we need to populate them.
  // OR update Layout.tsx to derive them.

  // Let's update Layout.tsx to derive them from report/context.

  // Data Fields (from backend)
  transcripts?: Record<string, string>;

  // Complex Objects
  eval_context?: EvalContext;
  report_v2?: EvalReport;
}

export interface Config {
  gen_model: string;
  eval_model: string;
  enabled_providers: Record<string, boolean>;
}

export interface UpdateContextRequest {
  id: string;
  eval_context: EvalContext;
}

export interface GenerateContextRequest {
  id: string;
  ground_truth: string;
}

export interface EvaluateRequest {
  id: string;
  eval_context: EvalContext;
  provider_ids: string[];
}

export interface Checkpoint {
  id: string;
  start_ms?: number;
  text_segment: string;
  tier: number;
  weight: number;
  rationale: string;
}

export interface ContextMeta {
  business_goal: string;
  audio_reality_inference: string;
  total_token_count_estimate: number;
  ground_truth: string;
  questionable_gt?: boolean;
  questionable_reason?: string;
}

export interface EvalContext {
  meta: ContextMeta;
  checkpoints: Checkpoint[];
  hash?: string;
}

export interface CheckpointResult {
  status: string; // "Pass", "Fail", "Partial"
  detected: string;
  reason?: string;
}

export interface PhoneticDetails {
  sub: number;
  del: number;
  ins: number;
}

export interface EvalMetrics {
  S_score: number;
  P_score: number;
  Q_score: number; // Pre-computed composite score from backend
  PER_details: PhoneticDetails;
}

export interface EvalResult {
  transcript: string;
  revised_transcript: string;
  metrics: EvalMetrics;
  checkpoint_results: Record<string, CheckpointResult>;
  summary: string[];
}

export interface EvalReport {
  evaluations: Record<string, EvalResult | Partial<EvalResult>>;
  context_snapshot?: EvalContext;
}
