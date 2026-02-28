import { useCallback, useEffect, useRef } from 'react';

interface ResizeDividerProps {
  onResize: (chatPercent: number) => void;
}

export default function ResizeDivider({ onResize }: ResizeDividerProps) {
  const dragging = useRef(false);

  useEffect(() => {
    return () => {
      if (dragging.current) {
        dragging.current = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
    };
  }, []);

  const onMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragging.current = true;
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';

      const onMouseMove = (ev: MouseEvent) => {
        if (!dragging.current) return;
        const pct = (ev.clientX / window.innerWidth) * 100;
        const clamped = Math.min(85, Math.max(25, pct));
        onResize(clamped);
      };

      const onMouseUp = () => {
        dragging.current = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', onMouseUp);
      };

      document.addEventListener('mousemove', onMouseMove);
      document.addEventListener('mouseup', onMouseUp);
    },
    [onResize],
  );

  return (
    <div
      onMouseDown={onMouseDown}
      className="w-1 hover:w-2 bg-zinc-800 hover:bg-zinc-600 cursor-col-resize shrink-0 transition-all"
    />
  );
}
