export interface EvalResult {
  score: number;
  revised_transcript?: string;
  transcript?: string;
  summary?: string[];
}

export interface EvalReport {
  ground_truth: string;
  eval_results: Record<string, EvalResult>;
}

export interface Case {
  id: string;
  eval_report?: EvalReport;
  ground_truth?: string;
  transcripts: Record<string, string>;
  has_ai?: boolean;
  best_performers?: string[];
  eval_context?: ContextResponse;
  report_v2?: EvaluationResponse;
}

export interface LoadingData {
  id: string;
  eval_report?: EvalReport;
  results?: Record<string, EvalResult>;
  transcripts: Record<string, string>;
  isLoading?: boolean;
  error?: string;
  has_ai?: boolean;
  ground_truth?: string;
  evaluated_ground_truth?: string;
  eval_context?: ContextResponse;
  report_v2?: EvaluationResponse;
}

// v2 Types
export interface Checkpoint {
  id: string;
  text_segment: string;
  tier: number;
  weight: number;
  rationale: string;
}

export interface MetaInfo {
  business_goal: string;
  audio_reality_inference: string;
  total_token_count_estimate: number;
  ground_truth: string;
  questionable_gt?: boolean;
  questionable_reason?: string;
}

export interface ContextResponse {
  meta: MetaInfo;
  checkpoints: Checkpoint[];
}

export interface CheckpointResult {
  status: string; // "Pass", "Fail", "Partial"
  detected: string;
  reason?: string;
}

export interface PERDetails {
  sub: number;
  del: number;
  ins: number;
}

export interface Metrics {
  S_score: number;
  P_score: number;
  Q_score: number; // Pre-computed composite score from backend
  PER_details: PERDetails;
}

export interface ModelEvaluation {
  transcript: string;
  revised_transcript: string;
  metrics: Metrics;
  checkpoint_results: Record<string, CheckpointResult>;
  summary: string[];
}

export interface EvaluationResponse {
  evaluations: Record<string, ModelEvaluation>;
  context_snapshot?: ContextResponse;
  context_hash?: string;
}
