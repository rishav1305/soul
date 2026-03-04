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
  thinking?: string;
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
export type TaskView = 'list' | 'kanban' | 'grid' | 'table';
export type GridSubView = 'grid' | 'table' | 'grouped';
export type HorizontalRailPosition = 'bottom' | 'top';
export type HorizontalRailTab = 'chat' | 'tasks';
export type ChatPosition = 'bottom' | 'top';

export interface TaskFilters {
  stage: TaskStage | 'all';
  priority: number | 'all';
  product: string | 'all';
}

export interface LayoutState {
  // Legacy — kept for compatibility but no longer drives main layout
  soulState: PanelState;
  chatState: PanelState;
  taskState: PanelState;
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
  // New layout
  activeProduct: string | null;         // which product is open in main view
  railPosition: HorizontalRailPosition; // bottom or top
  railExpanded: boolean;                // 48px bar vs expanded panel
  railHeightVh: number;                 // expanded height 25–60vh
  railTab: HorizontalRailTab;           // which tab is focused when expanded
  chatSplitPct: number;                 // chat % of expanded rail (default 60)
  sessionsOpen: boolean;                // sessions drawer overlay on left rail
  settingsOpen: boolean;                // settings panel overlay
  // Notification + context preferences
  autoInjectContext: boolean;           // auto-inject product context on new chat
  showContextChip: boolean;             // show "inject?" chip on product switch
  toastsEnabled: boolean;               // stage-change toast notifications
  inlineBadgesEnabled: boolean;         // pulsing dot badge on task cards
}

export interface StageNotification {
  id: string;
  taskId: number;
  taskTitle: string;
  fromStage: TaskStage;
  toStage: TaskStage;
  time: string;
}

export interface ChatSession {
  id: number;
  title: string;
  status: 'running' | 'idle' | 'completed';
  created_at: string;
  updated_at: string;
}

export interface SendOptions {
  model?: string;
  chatType?: string;
  disabledTools?: string[];
  thinking?: boolean;
  context?: string; // injected product context
}

export interface TaskActivity {
  task_id: number;
  type: 'status' | 'stage' | 'token' | 'tool_call' | 'tool_complete' | 'tool_progress' | 'tool_error' | 'done';
  content: string;
  time: string;
}

export interface TaskComment {
  id: number;
  task_id: number;
  author: 'user' | 'soul';
  type: 'feedback' | 'status' | 'verification' | 'error';
  body: string;
  attachments: string[];
  created_at: string;
}

/* ── Scout types ────────────────────────────────────── */

export interface ScoutReport {
  sync: ScoutSyncData;
  sweep: ScoutSweepData;
  applications: ScoutApplicationData;
  metrics: Record<string, ScoutMetric>;
  follow_ups: ScoutFollowUp[];
}

export interface ScoutSyncData {
  last_run: string;
  platforms_checked: number;
  in_sync: number;
  drift: number;
  details: ScoutPlatformSync[];
}

export interface ScoutPlatformSync {
  platform: string;
  in_sync: boolean;
  drift_fields: string[];
}

export interface ScoutSweepData {
  last_run: string;
  total_jobs: number;
  new_jobs: number;
  jobs: ScoutJob[];
}

export interface ScoutJob {
  id: string;
  title: string;
  company: string;
  location: string;
  url: string;
  platform: string;
  posted_at: string;
  match_score: number;
}

export interface ScoutApplicationData {
  total: number;
  by_status: Record<string, number>;
  recent: ScoutApplication[];
}

export interface ScoutApplication {
  id: string;
  company: string;
  role: string;
  status: string;
  url: string;
  notes: string;
  created_at: string;
  updated_at: string;
}

export interface ScoutMetric {
  label: string;
  value: string | number;
  trend?: 'up' | 'down' | 'flat';
}

export interface ScoutFollowUp {
  application_id: string;
  company: string;
  role: string;
  due_date: string;
  action: string;
}
