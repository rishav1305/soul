import { useState, useEffect } from 'react';
import type { ReactNode } from 'react';
import type { ConnectionState } from '../lib/types';

interface ConnectionBannerProps {
  status: ConnectionState;
  reconnectAttempt?: number;
  onReconnect?: () => void;
  authError?: boolean;
  onReauth?: () => Promise<void>;
}

export function ConnectionBanner({
  status,
  reconnectAttempt = 0,
  onReconnect,
  authError = false,
  onReauth,
}: ConnectionBannerProps) {
  const [dismissed, setDismissed] = useState(false);

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
  const btnClass = isError
    ? 'ml-2 px-2 py-0.5 text-xs border border-red-500 rounded hover:bg-red-800 transition-colors'
    : 'ml-2 px-2 py-0.5 text-xs border border-yellow-500 rounded hover:bg-yellow-800 transition-colors';

  let message: string;
  let actionButton: ReactNode = null;

  if (status === 'error' && authError) {
    message = 'Authentication failed.';
    if (onReauth) {
      actionButton = (
        <button
          data-testid="reauth-button"
          className={btnClass}
          onClick={() => { void onReauth(); }}
        >
          Re-authenticate
        </button>
      );
    }
  } else if (status === 'error') {
    message = 'Connection lost.';
    if (onReconnect) {
      actionButton = (
        <button
          data-testid="retry-button"
          className={btnClass}
          onClick={onReconnect}
        >
          Retry
        </button>
      );
    }
  } else {
    // disconnected: auto-reconnecting
    const suffix = reconnectAttempt > 1 ? `Retry #${reconnectAttempt}...` : 'Reconnecting...';
    message = `Connection lost. ${suffix}`;
  }

  return (
    <div
      data-testid="connection-banner"
      className={`flex items-center justify-between px-4 py-2 text-sm border-b ${bgClass} ${textClass} transition-opacity duration-300`}
    >
      <span className="flex items-center">
        {message}
        {actionButton}
      </span>
      <button
        data-testid="dismiss-banner-button"
        onClick={() => setDismissed(true)}
        className="ml-4 hover:opacity-70 transition-opacity"
        aria-label="Dismiss"
      >
        &times;
      </button>
    </div>
  );
}
