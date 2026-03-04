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
export type TaskView = 'list' | 'kanban' | 'grid' | 'table';
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
  context?: string;
}

/* ── Layout redesign types ────────────────────────────── */

export type ChatPosition = 'bottom' | 'top';

export interface StageNotification {
  id: string;
  taskId: number;
  taskTitle: string;
  fromStage: TaskStage;
  toStage: TaskStage;
  time: string;
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

export interface ScoutSyncDetail {
  field: string;
  expected: string;
  match: boolean;
}

export interface ScoutPlatformSync {
  platform: string;
  status: 'synced' | 'drift';
  issues?: string[];
  details?: ScoutSyncDetail[];
  checkedAt?: string;
}

export interface ScoutSweepData {
  last_run: string;
  new_opportunities: number;
  messages: number;
  opportunities: ScoutOpportunity[];
}

export interface ScoutOpportunity {
  id: string;
  company: string;
  role: string;
  platform: string;
  match?: number;
  url?: string;
  location?: string;
  postedAt?: string;
  foundAt: string;
  dismissed?: boolean;
}

export interface ScoutApplicationData {
  total: number;
  active: number;
  by_status: Record<string, number>;
  recent: ScoutApplication[];
}

export interface ScoutApplication {
  id: string;
  company: string;
  role: string;
  status: string;
  appliedAt: string;
  updatedAt: string;
}

export interface ScoutMetric {
  applied: number;
  responses: number;
  interviews: number;
  offers: number;
}

export interface ScoutFollowUp {
  company: string;
  role: string;
  follow_up: string;
  notes: string;
}

/* ── Profile Hub types ─────────────────────────────── */

export interface ProfileData {
  site_config: ProfileSiteConfig[];
  experience: ProfileExperience[];
  skill_categories: ProfileSkillCategory[];
  projects: ProfileProject[];
  education: ProfileEducation[];
  testimonials: ProfileTestimonial[];
  brands: ProfileBrand[];
  services: ProfileService[];
  case_studies: ProfileCaseStudy[];
}

export interface ProfileSiteConfig {
  id: string;
  name: string;
  title: string;
  email: string;
  short_bio: string;
  long_bio: string[];
  location: string;
  years_experience_start_year: number;
  whatsapp: string;
  social_media: Record<string, string>;
  domain_expertise?: string[];
  contact_info?: Record<string, string>;
}

export interface ProfileExperience {
  id: string;
  role: string;
  company: string;
  period: string;
  start_date: string;
  end_date: string | null;
  location: string;
  achievements: string[];
  tech_stack: string[];
  experience_type?: string;
  description?: string | null;
  details?: string[];
  tags?: string[];
  remote_work?: boolean;
  team_size?: number;
  key_metrics?: { label: string; value: string }[];
}

export interface ProfileSkillCategory {
  id: string;
  category_name: string;
  skills: { name: string; level: number }[];
  display_order: number;
}

export interface ProfileProject {
  id: string;
  title: string;
  description: string;
  short_description: string;
  tech_stack: string[];
  category: string;
  company: string;
  link?: string;
  start_date?: string;
  end_date?: string | null;
}

export interface ProfileEducation {
  id: string;
  institution: string;
  degree: string;
  period: string;
  location: string;
  focus_area: string;
  description: string;
}

export interface ProfileTestimonial {
  id: string;
  name: string;
  position: string;
  company: string;
  text: string;
  location?: string;
}

export interface ProfileBrand {
  id: string;
  name: string;
  logo: string;
  color: string;
}

export interface ProfileService {
  id: string;
  title: string;
  description: string;
  icon_name: string;
  skills: string[];
}

export interface ProfileCaseStudy {
  id: string;
  title: string;
  role: string;
  challenge: string;
  solution: string;
  impact: string;
  metrics: { label: string; value: string }[];
  tech_stack: string[];
}
