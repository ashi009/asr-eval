import React, { useState, useRef, useLayoutEffect } from 'react';
import { createPortal } from 'react-dom';

interface RichTooltipProps {
  trigger: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  tooltipClassName?: string;
  style?: React.CSSProperties;
}

export function RichTooltip({ trigger, children, className = "", tooltipClassName = "", style }: RichTooltipProps) {
  const [isVisible, setIsVisible] = useState(false);
  const [coords, setCoords] = useState<{ top: number; left: number; side: 'top' | 'bottom' } | null>(null);
  const triggerRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (isVisible && triggerRef.current) {
      const triggerRect = triggerRef.current.getBoundingClientRect();
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;

      const tooltipWidth = 288; // matches w-72 (18rem)
      const estimatedHeight = 160;
      const verticalGap = 8;
      const horizontalPadding = 0;

      // Horizontal centering + clamping
      const triggerCenter = triggerRect.left + triggerRect.width / 2;
      const tooltipHalfWidth = tooltipWidth / 2;

      const minLeft = tooltipHalfWidth + horizontalPadding;
      const maxLeft = viewportWidth - tooltipHalfWidth - horizontalPadding;

      const finalCenter = Math.max(minLeft, Math.min(maxLeft, triggerCenter));
      const finalLeft = finalCenter - tooltipHalfWidth;

      // Vertical positioning with flip
      let finalTop = triggerRect.bottom + verticalGap;
      let side: 'top' | 'bottom' = 'bottom';

      // If it overflows bottom, flip to top
      if (finalTop + estimatedHeight > viewportHeight - 10) {
        finalTop = triggerRect.top - verticalGap;
        side = 'top';
      }

      setCoords({ top: finalTop, left: finalLeft, side });
    } else {
      setCoords(null);
    }
  }, [isVisible]);

  const hasPositioning = className.includes('absolute') || className.includes('fixed') || className.includes('top-') || className.includes('left-');
  const baseClasses = hasPositioning ? "" : "relative inline-block";

  return (
    <div
      ref={triggerRef}
      className={`${baseClasses} ${className}`}
      style={style}
      onMouseEnter={() => setIsVisible(true)}
      onMouseLeave={() => setIsVisible(false)}
    >
      {trigger}
      {isVisible && coords && createPortal(
        <div
          style={{
            position: 'fixed',
            top: coords.side === 'bottom' ? `${coords.top}px` : 'auto',
            bottom: coords.side === 'top' ? `${window.innerHeight - coords.top}px` : 'auto',
            left: `${coords.left}px`,
            width: '288px',
          }}
          className={`z-[100] p-0 bg-white shadow-2xl rounded-xl border overflow-hidden animate-in fade-in zoom-in duration-200 pointer-events-none ${coords.side === 'bottom' ? 'origin-top' : 'origin-bottom'
            } ${tooltipClassName || 'border-slate-200'}`}
        >
          {children}
        </div>,
        document.body
      )}
    </div>
  );
}
