import { useState, useEffect } from 'react';
import type { ConnectionState } from '../lib/types';

interface ConnectionBannerProps {
  status: ConnectionState;
  reconnectAttempt?: number;
}

export function ConnectionBanner({ status, reconnectAttempt = 0 }: ConnectionBannerProps) {
  const [dismissed, setDismissed] = useState(false);

  // Reset dismissed state when status changes to a non-error state,
  // so the banner reappears on the next disconnect.
  useEffect(() => {
    if (status === 'connected' || status === 'connecting') {
      setDismissed(false);
    }
  }, [status]);

  const show = !dismissed && (status === 'disconnected' || status === 'error');

  if (!show) return null;

  const isError = status === 'error';
  const bgClass = isError
    ? 'bg-red-900/80 border-red-700'
    : 'bg-yellow-900/80 border-yellow-700';
  const textClass = isError ? 'text-red-200' : 'text-yellow-200';

  const base = isError ? 'Connection error.' : 'Connection lost.';
  const suffix = reconnectAttempt > 1
    ? `Retry #${reconnectAttempt}...`
    : 'Reconnecting...';

  return (
    <div
      data-testid="connection-banner"
      className={`flex items-center justify-between px-4 py-2 text-sm border-b ${bgClass} ${textClass} transition-opacity duration-300`}
    >
      <span>{base} {suffix}</span>
      <button
        onClick={() => setDismissed(true)}
        className="ml-4 hover:opacity-70 transition-opacity"
        aria-label="Dismiss"
      >
        &times;
      </button>
    </div>
  );
}
