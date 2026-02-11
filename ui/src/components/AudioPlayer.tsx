import { useState, useRef, forwardRef, useImperativeHandle } from 'react';
import { Play, Pause } from 'lucide-react';
import { formatTime } from '../utils/formatUtils';

interface AudioPlayerProps {
  caseId: string;
  className?: string;
}

export interface AudioPlayerHandle {
  seek: (time: number) => void;
  pause: () => void;
}

export const AudioPlayer = forwardRef<AudioPlayerHandle, AudioPlayerProps>(({ caseId, className = '' }, ref) => {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  useImperativeHandle(ref, () => ({
    seek: (time: number) => {
      seek(time);
    },
    pause: () => {
      togglePlay(false);
    }
  }));

  const seek = (time: number) => {
    if (audioRef.current) {
      audioRef.current.currentTime = time;
      togglePlay(true);
    }
  };

  const togglePlay = (shouldPlay?: boolean) => {
    if (!audioRef.current) return;

    const targetState = shouldPlay !== undefined ? shouldPlay : audioRef.current.paused;

    if (targetState) {
      audioRef.current.play();
      setIsPlaying(true);
    } else {
      audioRef.current.pause();
      setIsPlaying(false);
    }
  };

  return (
    <div className={`flex items-center gap-3 ${className}`}>
      <button onClick={() => togglePlay()} className="w-7 h-7 rounded-full bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 flex items-center justify-center hover:opacity-90 transition-opacity shrink-0">
        {isPlaying ? <Pause size={12} fill="currentColor" /> : <Play size={12} fill="currentColor" />}
      </button>
      <div className="flex-1">
        <div
          className="relative h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full cursor-pointer"
          onClick={e => {
            const rect = e.currentTarget.getBoundingClientRect();
            const pct = (e.clientX - rect.left) / rect.width;
            if (audioRef.current) {
              seek(pct * duration);
            }
          }}
        >
          <div className="absolute top-0 left-0 h-full bg-blue-500 dark:bg-blue-400 rounded-full transition-all" style={{ width: `${duration > 0 ? (currentTime / duration) * 100 : 0}%` }} />
        </div>
      </div>
      <span className="text-[10px] font-mono text-slate-400 dark:text-slate-500 shrink-0">{formatTime(currentTime)} / {formatTime(duration)}</span>
      <audio
        ref={audioRef}
        src={`/audio/${caseId}.flac`}
        onTimeUpdate={e => setCurrentTime(e.currentTarget.currentTime)}
        onLoadedMetadata={e => setDuration(e.currentTarget.duration)}
        onEnded={() => setIsPlaying(false)}
      />
    </div>
  );
});
