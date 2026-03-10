import { useCallback, useRef, useState } from 'react';

interface UseSwipeDrawerOptions {
  /** Width of the left edge zone that triggers swipe-to-open (px). */
  edgeWidth?: number;
  /** Minimum horizontal distance to trigger open/close (px). */
  threshold?: number;
}

interface UseSwipeDrawerReturn {
  isOpen: boolean;
  open: () => void;
  close: () => void;
  toggle: () => void;
  /** Spread onto the container element to enable swipe gestures. */
  handlers: {
    onTouchStart: (e: React.TouchEvent) => void;
    onTouchMoveCapture: (e: React.TouchEvent) => void;
    onTouchEnd: () => void;
  };
}

export function useSwipeDrawer(
  options: UseSwipeDrawerOptions = {},
): UseSwipeDrawerReturn {
  const { edgeWidth = 30, threshold = 50 } = options;

  const [isOpen, setIsOpen] = useState(false);
  const startX = useRef(0);
  const startY = useRef(0);
  const tracking = useRef(false);

  const open = useCallback(() => setIsOpen(true), []);
  const close = useCallback(() => setIsOpen(false), []);
  const toggle = useCallback(() => setIsOpen(v => !v), []);

  const onTouchStart = useCallback(
    (e: React.TouchEvent) => {
      const touch = e.touches[0];
      if (!touch) return;
      startX.current = touch.clientX;
      startY.current = touch.clientY;

      // Only track if drawer is open (swipe to close) or touch starts in edge zone.
      tracking.current = isOpen || touch.clientX <= edgeWidth;
    },
    [isOpen, edgeWidth],
  );

  // Track position during move for use in onTouchEnd (which has no position data).

  const lastTouch = useRef<{ x: number; y: number } | null>(null);

  const onTouchMoveCapture = useCallback((e: React.TouchEvent) => {
    const touch = e.touches[0];
    if (touch) {
      lastTouch.current = { x: touch.clientX, y: touch.clientY };
    }
  }, []);

  const onTouchEnd = useCallback(() => {
    if (!tracking.current) return;
    tracking.current = false;

    const end = lastTouch.current;
    if (!end) return;

    const dx = end.x - startX.current;
    const dy = Math.abs(end.y - startY.current);

    // Ignore mostly-vertical swipes.
    if (dy > Math.abs(dx)) return;

    if (!isOpen && dx > threshold) {
      setIsOpen(true);
    } else if (isOpen && dx < -threshold) {
      setIsOpen(false);
    }
  }, [isOpen, threshold]);

  return {
    isOpen,
    open,
    close,
    toggle,
    handlers: { onTouchStart, onTouchMoveCapture, onTouchEnd },
  };
}
