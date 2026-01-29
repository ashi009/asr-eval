import { diffArrays, ArrayChange } from 'diff';



// Smart segmentation for mixed English/Chinese text
const segmentText = (text: string): string[] => {
  if (!text) return [];
  // Use Intl.Segmenter if available (modern browsers)
  if (typeof Intl !== 'undefined' && Intl.Segmenter) {
    const segmenter = new Intl.Segmenter('zh-CN', { granularity: 'word' });
    return Array.from(segmenter.segment(text)).map(s => s.segment);
  }

  // Fallback: Split by spaces but keep Chinese characters separate (simple regex approach)
  // This is less robust but works for basic cases if Intl is missing
  return text.split(/(\s+|[\u4e00-\u9fa5])/).filter(Boolean);
};

export type DiffChange = ArrayChange<string>;

export const smartDiff = (original: string, revised: string, ignoreCase = false): DiffChange[] => {
  const oldArr = segmentText(original);
  const newArr = segmentText(revised);

  if (ignoreCase) {
    return diffArrays(oldArr, newArr, {
      comparator: (left, right) => left.localeCompare(right, undefined, { sensitivity: 'accent' }) === 0
    });
  }

  return diffArrays(oldArr, newArr);
};
