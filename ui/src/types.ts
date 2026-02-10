

export interface Case {
  id: string;

  ground_truth?: string;
  transcripts: Record<string, string>;
  has_ai?: boolean;
  best_performers?: string[];
  questionable_gt?: boolean;
  eval_context?: EvalContext;
  report_v2?: EvalReport;
}



// v2 Types
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
  context_hash?: string;
}
