import AppShell from './components/layout/AppShell.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';

export default function App() {
  const ws = useWebSocketProvider();
  return (
    <WebSocketContext.Provider value={ws}>
      <AppShell />
    </WebSocketContext.Provider>
  );
}
