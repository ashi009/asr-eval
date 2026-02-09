import { diffWords, Change } from 'diff';

export type DiffChange = Change;



export const smartDiff = (original: string, revised: string, ignoreCase = false): DiffChange[] => {
  const options: any = { ignoreCase };
  if (typeof Intl !== 'undefined' && Intl.Segmenter) {
    options.intlSegmenter = new Intl.Segmenter('zh-CN', { granularity: 'word' });
  }
  return diffWords(original, revised, options) || [];
};
