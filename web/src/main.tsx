import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './app.css';

function App() {
  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-2xl font-bold tracking-tight">Soul v2</h1>
        <p className="text-zinc-500 mt-2 text-sm">Spec-driven. Agent-maintained.</p>
      </div>
    </div>
  );
}

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
