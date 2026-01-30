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
}
