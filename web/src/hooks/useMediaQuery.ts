import { useState, useEffect } from 'react';

/**
 * Reactive media query hook. Returns true when the query matches.
 *
 * Usage:
 *   const isMobile = useMediaQuery('(max-width: 767px)');
 *   const isTablet = useMediaQuery('(max-width: 1023px)');
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window === 'undefined') return false;
    return window.matchMedia(query).matches;
  });

  useEffect(() => {
    const mql = window.matchMedia(query);
    const handler = (e: MediaQueryListEvent) => setMatches(e.matches);
    mql.addEventListener('change', handler);
    setMatches(mql.matches); // sync on mount / query change
    return () => mql.removeEventListener('change', handler);
  }, [query]);

  return matches;
}
