// Stub — full implementation in Phase 2 Step 3 (chat component port)
import type { ChatSession } from '../../lib/types.ts';

interface SessionDrawerProps {
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSelect: (id: number) => void;
  onClose: () => void;
}

export default function SessionDrawer(_props: SessionDrawerProps) {
  return null;
}
