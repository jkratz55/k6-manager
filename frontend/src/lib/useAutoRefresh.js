import { useEffect, useRef } from 'react';

// useAutoRefresh calls callback once on mount, then every intervalMs while
// enabled, skipping ticks while the browser tab is hidden.
export function useAutoRefresh(callback, intervalMs, enabled = true) {
  const saved = useRef(callback);

  useEffect(() => {
    saved.current = callback;
  }, [callback]);

  useEffect(() => {
    saved.current();
  }, []);

  useEffect(() => {
    if (!enabled) return undefined;
    const id = setInterval(() => {
      if (!document.hidden) saved.current();
    }, intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, enabled]);
}
