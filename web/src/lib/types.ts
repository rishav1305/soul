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
  model?: string;
  pmNotification?: {
    severity: 'info' | 'warning' | 'error';
    taskIds: number[];
    check: string;
  };
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

export type TaskView = 'list' | 'kanban' | 'grid' | 'table';
export type GridSubView = 'grid' | 'table' | 'grouped';
export type HorizontalRailPosition = 'bottom' | 'top';
export type PanelPosition = 'bottom' | 'top' | 'right';
export type HorizontalRailTab = 'chat' | 'tasks';
export type DrawerLayout = 'split' | 'independent';

export interface TaskFilters {
  stage: TaskStage | 'all';
  priority: number | 'all';
  product: string | 'all';
}

export interface LayoutState {
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
  activeProduct: string | null;         // which product is open in main view
  railPosition: HorizontalRailPosition; // bottom or top (legacy, used by HorizontalRail)
  chatPosition: PanelPosition;           // independent chat panel position
  tasksPosition: PanelPosition;          // independent tasks panel position
  railExpanded: boolean;                // 48px bar vs expanded panel
  railHeightVh: number;                 // expanded height 25–60vh
  railTab: HorizontalRailTab;           // which tab is focused when expanded
  chatSplitPct: number;                 // chat % of expanded rail (default 60)
  drawerLayout: DrawerLayout;           // split (side-by-side) or independent (stacked)
  panelExpanded: boolean;               // left rail expanded (220px vs 56px)
  sessionsOpen: boolean;                // sessions drawer overlay on left rail
  settingsOpen: boolean;                // settings panel overlay
  autoInjectContext: boolean;           // auto-inject product context on new chat
  showContextChip: boolean;             // show "inject?" chip on product switch
  toastsEnabled: boolean;               // stage-change toast notifications
  inlineBadgesEnabled: boolean;         // pulsing dot badge on task cards
  syncProductFilter: boolean;           // sync task list filter with active product
  chatRailExpanded: boolean;            // per-panel expanded (used in mixed position mode)
  chatRailHeightVh: number;
  tasksRailExpanded: boolean;
  tasksRailHeightVh: number;
  rightPanelWidth: number;            // width in px — both panels open
  rightChatWidth: number;             // width in px — chat only
  rightTasksWidth: number;            // width in px — tasks only
  rightChatExpanded: boolean;         // right chat panel drawer vs rail
  rightTasksExpanded: boolean;        // right tasks panel drawer vs rail
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
  summary: string;
  model: string;
  message_count: number;
  status: 'running' | 'idle' | 'completed';
  created_at: string;
  updated_at: string;
}

export interface TokenUsage {
  inputTokens: number;
  outputTokens: number;
  contextPct: number;
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

/** Product metadata from /api/products */
export interface ProductInfo {
  name: string;
  version: string;
  label: string;
  color: string;
  tools: number;
  running: boolean;
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
