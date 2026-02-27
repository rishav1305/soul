import ChatView from './components/chat/ChatView.tsx';
import KanbanBoard from './components/planner/KanbanBoard.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';

function AppContent() {
  return (
    <div className="h-screen bg-zinc-950 text-zinc-100 flex flex-col">
      <header className="h-12 border-b border-zinc-800 flex items-center px-4 shrink-0">
        <span className="text-lg font-bold">&#9670; Soul</span>
      </header>
      <main className="flex-1 flex overflow-hidden">
        <div className="w-1/2 min-w-[400px] border-r border-zinc-800">
          <ChatView />
        </div>
        <div className="w-1/2 min-w-[400px]">
          <KanbanBoard />
        </div>
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
