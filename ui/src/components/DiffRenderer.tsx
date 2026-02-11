import { smartDiff } from '../diffUtils';

export const renderDiff = (original: string, revised?: string) => {
  if (revised === undefined || revised === null) return original;

  const diffs = smartDiff(original, revised, true);

  return (
    <span>
      {diffs.map((part, index) => {
        if (part.added) {
          return (
            <span key={index} className="bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 font-medium px-0.5 rounded mx-0.5 animate-in fade-in duration-300 select-none">
              {part.value}
            </span>
          );
        } else if (part.removed) {
          return (
            <span key={index} className="bg-red-50 dark:bg-red-900/30 text-red-500 dark:text-red-400 line-through decoration-red-400/50 px-0.5 rounded mx-0.5 opacity-60">
              {part.value}
            </span>
          );
        }
        return <span key={index}>{part.value}</span>;
      })}
    </span>
  );
};
