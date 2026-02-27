import ChatView from './components/chat/ChatView.tsx';
import PanelContainer from './components/panels/PanelContainer.tsx';
import CompliancePanel from './components/panels/CompliancePanel.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';
import { useScanResult } from './hooks/useScanResult.ts';

function AppContent() {
  const { findings, progress, scanResult, isScanning } = useScanResult();

  return (
    <div className="h-screen bg-zinc-950 text-zinc-100 flex flex-col">
      <header className="h-12 border-b border-zinc-800 flex items-center px-4 shrink-0">
        <span className="text-lg font-bold">&#9670; Soul</span>
      </header>
      <main className="flex-1 flex overflow-hidden">
        <div className="flex-1 min-w-0">
          <ChatView />
        </div>
        <PanelContainer>
          <CompliancePanel
            findings={findings}
            progress={progress}
            scanResult={scanResult}
            isScanning={isScanning}
          />
        </PanelContainer>
      </main>
    </div>
  );
}

export default function App() {
  const ws = useWebSocketProvider();

  return (
    <WebSocketContext.Provider value={ws}>
      <AppContent />
    </WebSocketContext.Provider>
  );
}
