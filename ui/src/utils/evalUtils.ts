import { EvalResult } from '../types';

export const isResultStale = (currentTranscript: string | undefined, result: EvalResult | undefined) => {
  if (!result?.transcript) return false;
  return currentTranscript !== result.transcript;
};
