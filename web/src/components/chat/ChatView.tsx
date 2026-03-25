// Stub — full implementation in Phase 2 Step 3 (chat component port)

interface ChatViewProps {
  activeSessionId: number | null;
  onSessionCreated?: (id: number) => void;
  activeProduct: string | null;
  buildContextString?: () => string;
  autoInjectContext?: boolean;
  showContextChip?: boolean;
}

export default function ChatView(_props: ChatViewProps) {
  return null;
}
