import { useState, useEffect, type ReactNode } from 'react';

const TOKEN_KEY = 'soul-v2-token';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
  window.location.reload();
}

export function AuthGate({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(getToken());
  const [input, setInput] = useState('');

  useEffect(() => {
    const handler = () => setToken(null);
    window.addEventListener('soul-v2-auth-failed', handler);
    return () => window.removeEventListener('soul-v2-auth-failed', handler);
  }, []);

  if (token) return <>{children}</>;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (input.trim()) {
      localStorage.setItem(TOKEN_KEY, input.trim());
      setToken(input.trim());
    }
  };

  return (
    <div data-testid="auth-gate" className="h-screen flex items-center justify-center bg-base">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4 p-8 bg-elevated rounded-xl border border-border-default max-w-sm w-full">
        <h2 className="text-lg font-semibold text-fg text-center">Soul v2</h2>
        <p className="text-sm text-fg-muted text-center">Enter access token to continue</p>
        <input
          data-testid="auth-token-input"
          type="password"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder="Access token"
          className="px-3 py-2 bg-surface border border-border-default rounded-lg text-fg text-sm focus:outline-none focus:border-soul"
          autoFocus
        />
        <button
          data-testid="auth-submit"
          type="submit"
          className="px-4 py-2 bg-soul text-white rounded-lg text-sm font-medium hover:opacity-90"
        >
          Continue
        </button>
      </form>
    </div>
  );
}
