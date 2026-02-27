import { useState, useCallback, type ReactNode } from 'react';

interface PanelContainerProps {
  children: ReactNode;
  defaultWidth?: number;
}

export default function PanelContainer({
  children,
  defaultWidth = 420,
}: PanelContainerProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [width, setWidth] = useState(defaultWidth);
  const [isResizing, setIsResizing] = useState(false);

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      setIsResizing(true);

      const startX = e.clientX;
      const startWidth = width;

      const handleMouseMove = (moveEvent: MouseEvent) => {
        const delta = startX - moveEvent.clientX;
        const newWidth = Math.max(280, Math.min(800, startWidth + delta));
        setWidth(newWidth);
      };

      const handleMouseUp = () => {
        setIsResizing(false);
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
      };

      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
    },
    [width],
  );

  return (
    <div
      className="relative shrink-0 border-l border-zinc-800 bg-zinc-950 flex"
      style={{ width: collapsed ? 0 : width }}
    >
      {/* Resize handle */}
      {!collapsed && (
        <div
          onMouseDown={handleMouseDown}
          className={`absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-sky-500/50 transition-colors z-10 ${
            isResizing ? 'bg-sky-500/50' : ''
          }`}
        />
      )}

      {/* Collapse toggle */}
      <button
        onClick={() => setCollapsed(!collapsed)}
        className="absolute -left-6 top-3 w-6 h-8 bg-zinc-900 border border-zinc-800 border-r-0 rounded-l flex items-center justify-center text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors z-20"
      >
        <span className="text-xs">{collapsed ? '\u2039' : '\u203A'}</span>
      </button>

      {/* Panel content */}
      {!collapsed && (
        <div className="flex-1 overflow-y-auto overflow-x-hidden">
          {children}
        </div>
      )}
    </div>
  );
}
