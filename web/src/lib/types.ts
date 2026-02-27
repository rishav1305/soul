export interface WSMessage {
  type: string;
  session_id?: string;
  content?: string;
  data?: unknown;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  toolCalls?: ToolCallMessage[];
  timestamp: Date;
}

export interface ToolCallMessage {
  id: string;
  name: string;
  input: unknown;
  status: 'running' | 'complete' | 'error';
  progress?: number;
  output?: string;
  findings?: FindingMessage[];
}

export interface FindingMessage {
  id: string;
  title: string;
  severity: string;
  file?: string;
  line?: number;
  evidence?: string;
}
