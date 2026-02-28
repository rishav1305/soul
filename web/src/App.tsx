import AppShell from './components/layout/AppShell.tsx';
import SplashScreen from './components/layout/SplashScreen.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';

export default function App() {
  const ws = useWebSocketProvider();
  return (
    <WebSocketContext.Provider value={ws}>
      <SplashScreen ready={ws.connected} />
      <AppShell />
    </WebSocketContext.Provider>
  );
}
