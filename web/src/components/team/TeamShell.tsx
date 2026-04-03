import { useState, useCallback } from 'react';
import { NavigationSidebar } from './NavigationSidebar.tsx';
import TeamDashboard from './TeamDashboard.tsx';
import AgentDetail from './AgentDetail.tsx';
import TeamChat from './TeamChat.tsx';
import TaskBoard from './TaskBoard.tsx';
import TeamHealth from './TeamHealth.tsx';

/**
 * TeamShell — top-level shell for the team section.
 * Manages internal navigation state (no react-router needed — integrates into AppShell ProductView).
 * Routes:
 *   /team          → TeamDashboard
 *   /team/:agent   → AgentDetail
 *   /team/chat     → TeamChat
 *   /team/tasks    → TaskBoard
 *   /team/health   → TeamHealth
 */
export default function TeamShell() {
  const [currentPath, setCurrentPath] = useState('/team');

  const navigate = useCallback((to: string) => {
    setCurrentPath(to);
  }, []);

  // Derive current view from path
  const renderPage = () => {
    if (currentPath === '/team') {
      return <TeamDashboard onNavigate={navigate} />;
    }
    if (currentPath === '/team/chat') {
      return <TeamChat />;
    }
    if (currentPath === '/team/tasks') {
      return <TaskBoard />;
    }
    if (currentPath === '/team/health') {
      return <TeamHealth />;
    }
    // /team/:agent — extract agent name from path
    const agentMatch = currentPath.match(/^\/team\/([a-z]+)$/);
    if (agentMatch?.[1]) {
      return <AgentDetail agentName={agentMatch[1]} onNavigate={navigate} />;
    }
    // Fallback
    return <TeamDashboard onNavigate={navigate} />;
  };

  return (
    <div data-testid="team-shell" className="flex h-full overflow-hidden bg-deep">
      {/* Left navigation sidebar */}
      <NavigationSidebar
        currentPath={currentPath}
        onNavigate={navigate}
      />

      {/* Main content area */}
      <main className="flex-1 min-w-0 overflow-hidden" id="team-main-content">
        {renderPage()}
      </main>
    </div>
  );
}
