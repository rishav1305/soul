import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Shell } from './components/Shell';
import './app.css';

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <Shell />
    </StrictMode>,
  );
}

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {
    // Service worker registration failed — app works without it
  });
}
