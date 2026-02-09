interface ResultWithTranscript {
  transcript?: string;
}

export const isResultStale = (currentTranscript: string | undefined, result: ResultWithTranscript | undefined) => {
  if (!result?.transcript) return false;
  return currentTranscript !== result.transcript;
};
