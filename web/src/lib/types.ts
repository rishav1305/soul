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

export type TaskStage = 'backlog' | 'brainstorm' | 'active' | 'blocked' | 'validation' | 'done';
export type TaskSubstep = 'tdd' | 'implementing' | 'reviewing' | 'qa_test' | 'e2e_test' | 'security_review';

export interface PlannerTask {
  id: number;
  title: string;
  description: string;
  acceptance: string;
  stage: TaskStage;
  substep: TaskSubstep | '';
  priority: number;
  source: string;
  blocker: string;
  plan: string;
  output: string;
  error: string;
  agent_id: string;
  product: string;
  parent_id: number | null;
  metadata: string;
  retry_count: number;
  max_retries: number;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
}

/* ── Layout types ────────────────────────────────────── */

export type PanelState = 'rail' | 'open';
export type TaskView = 'list' | 'kanban' | 'grid';
export type GridSubView = 'grid' | 'table' | 'grouped';

export interface TaskFilters {
  stage: TaskStage | 'all';
  priority: number | 'all';
  product: string | 'all';
}

export interface LayoutState {
  soulState: PanelState;
  chatState: PanelState;
  taskState: PanelState;
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
}

export interface ChatSession {
  id: number;
  title: string;
  status: 'running' | 'idle' | 'completed';
  created_at: string;
  updated_at: string;
}

export interface ChatMessageRecord {
  id: number;
  session_id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  created_at: string;
}

export interface SendOptions {
  model?: string;
  chatType?: string;
  disabledTools?: string[];
  thinking?: boolean;
}

export interface TaskActivity {
  task_id: number;
  type: 'status' | 'stage' | 'token' | 'tool_call' | 'tool_complete' | 'tool_progress' | 'tool_error' | 'done';
  content: string;
  time: string;
}
