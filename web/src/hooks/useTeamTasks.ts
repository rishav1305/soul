import { useState, useEffect, useCallback, useRef } from 'react';
import { authFetch } from '../lib/api.ts';

/** Task stage for the Kanban board. */
export type TeamTaskStage = 'backlog' | 'in-progress' | 'blocked' | 'done';

/** Shape of a team task. */
export interface TeamTask {
  id: string;
  title: string;
  description?: string;
  assignee?: string;
  priority: 'critical' | 'high' | 'medium' | 'low';
  stage: TeamTaskStage;
  track?: string;
  dueDate?: number;
  createdAt: number;
}

export interface TaskFilters {
  assignee: string;
  track: string;
  priority: string;
}

// -- Backend mapping ----------------------------------------------------------

/** Map backend task status to Kanban stage. */
function toStage(status: string): TeamTaskStage {
  switch (status) {
    case 'in_progress': return 'in-progress';
    case 'blocked': return 'blocked';
    case 'done': case 'completed': return 'done';
    default: return 'backlog'; // open, pending, etc.
  }
}

/** Map backend priority (numeric string or keyword) to frontend priority. */
function toPriority(p: string): TeamTask['priority'] {
  const n = parseInt(p, 10);
  if (!isNaN(n)) {
    if (n >= 9) return 'critical';
    if (n >= 7) return 'high';
    if (n >= 4) return 'medium';
    return 'low';
  }
  if (['critical', 'high', 'medium', 'low'].includes(p)) return p as TeamTask['priority'];
  return 'medium';
}

/** Map a backend TaskItem to the frontend TeamTask shape. */
function mapTask(raw: Record<string, unknown>, idx: number): TeamTask {
  const created = raw.created
    ? new Date(String(raw.created)).getTime()
    : Date.now();
  const dueDate = raw.due_date
    ? new Date(String(raw.due_date)).getTime()
    : undefined;

  return {
    id: String(raw.file ?? raw.id ?? `task-${idx}`),
    title: String(raw.title ?? ''),
    description: raw.description ? String(raw.description) : undefined,
    assignee: raw.assign ? String(raw.assign) : undefined,
    priority: toPriority(String(raw.priority ?? '5')),
    stage: toStage(String(raw.status ?? 'open')),
    track: raw.project ? String(raw.project) : undefined,
    dueDate: dueDate && !isNaN(dueDate) ? dueDate : undefined,
    createdAt: isNaN(created) ? Date.now() : created,
  };
}

// -- Stage grouping -----------------------------------------------------------

function groupByStage(tasks: TeamTask[]): Record<TeamTaskStage, TeamTask[]> {
  const result: Record<TeamTaskStage, TeamTask[]> = {
    backlog:       [],
    'in-progress': [],
    blocked:       [],
    done:          [],
  };
  for (const t of tasks) result[t.stage].push(t);
  return result;
}

// -- Hook ---------------------------------------------------------------------

interface UseTeamTasksResult {
  tasks: TeamTask[];
  tasksByStage: Record<TeamTaskStage, TeamTask[]>;
  selectedTask: TeamTask | null;
  createTask: (title: string, assignee: string, priority: TeamTask['priority']) => void;
  updateTask: (id: string, updates: Partial<TeamTask>) => void;
  moveTask: (id: string, stage: TeamTaskStage) => void;
  selectTask: (id: string | null) => void;
  filterBy: (filters: TaskFilters) => void;
  activeFilters: TaskFilters;
}

/** Map frontend stage back to backend status string. */
function toBackendStatus(stage: TeamTaskStage): string {
  switch (stage) {
    case 'in-progress': return 'in_progress';
    case 'backlog': return 'open';
    default: return stage;
  }
}

/**
 * useTeamTasks -- manages the Kanban task board backed by the real API.
 *
 * - List: GET /api/team/tasks
 * - Create: POST /api/team/tasks
 * - Update: PATCH /api/team/tasks/:id
 */
export function useTeamTasks(): UseTeamTasksResult {
  const [tasks, setTasks] = useState<TeamTask[]>([]);
  const [selectedTask, setSelectedTask] = useState<TeamTask | null>(null);
  const [activeFilters, setActiveFilters] = useState<TaskFilters>({ assignee: '', track: '', priority: '' });
  const mountedRef = useRef(true);

  // Fetch tasks on mount and every 30s.
  const fetchTasks = useCallback(async () => {
    try {
      const res = await authFetch('/api/team/tasks');
      if (!res.ok) return;
      const data = await res.json();

      // The dashboard endpoint returns { projects, overdue, due_soon, summary }.
      // We flatten all tasks from projects.
      let allTasks: TeamTask[] = [];
      if (data && Array.isArray(data.projects)) {
        for (const proj of data.projects) {
          if (Array.isArray(proj.top_tasks)) {
            allTasks.push(...proj.top_tasks.map((t: Record<string, unknown>, i: number) => mapTask(t, i)));
          }
        }
      } else if (Array.isArray(data)) {
        // Direct task list response (with filters).
        allTasks = data.map((t: Record<string, unknown>, i: number) => mapTask(t, i));
      }

      if (mountedRef.current) {
        setTasks(allTasks);
      }
    } catch {
      // Non-fatal: keep existing local state.
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    void fetchTasks();
    const interval = setInterval(() => { void fetchTasks(); }, 30000);
    return () => {
      mountedRef.current = false;
      clearInterval(interval);
    };
  }, [fetchTasks]);

  const filtered = tasks.filter((t) => {
    if (activeFilters.assignee && t.assignee !== activeFilters.assignee) return false;
    if (activeFilters.track && t.track !== activeFilters.track) return false;
    if (activeFilters.priority && t.priority !== activeFilters.priority) return false;
    return true;
  });

  const tasksByStage = groupByStage(filtered);

  const createTask = useCallback((title: string, assignee: string, priority: TeamTask['priority']) => {
    // Optimistic local add.
    const newTask: TeamTask = {
      id: `t${Date.now()}`,
      title,
      assignee,
      priority,
      stage: 'backlog',
      createdAt: Date.now(),
    };
    setTasks((prev) => [...prev, newTask]);

    // POST to backend.
    authFetch('/api/team/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title,
        assign: assignee,
        priority: priority === 'critical' ? '10' : priority === 'high' ? '8' : priority === 'medium' ? '5' : '2',
        project: 'soul-team',
      }),
    }).catch(() => {
      // Optimistic update stays; next poll will reconcile.
    });
  }, []);

  const updateTask = useCallback((id: string, updates: Partial<TeamTask>) => {
    setTasks((prev) => prev.map((t) => t.id === id ? { ...t, ...updates } : t));
    setSelectedTask((prev) => prev?.id === id ? { ...prev, ...updates } : prev);

    // PATCH to backend.
    const body: Record<string, string> = {};
    if (updates.stage) body.status = toBackendStatus(updates.stage);
    if (updates.assignee) body.assign = updates.assignee;
    if (updates.priority) {
      body.priority = updates.priority === 'critical' ? '10' : updates.priority === 'high' ? '8' : updates.priority === 'medium' ? '5' : '2';
    }

    if (Object.keys(body).length > 0) {
      authFetch(`/api/team/tasks/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }).catch(() => {
        // Keep optimistic state.
      });
    }
  }, []);

  const moveTask = useCallback((id: string, stage: TeamTaskStage) => {
    setTasks((prev) => prev.map((t) => t.id === id ? { ...t, stage } : t));

    // PATCH status to backend.
    authFetch(`/api/team/tasks/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status: toBackendStatus(stage) }),
    }).catch(() => {
      // Keep optimistic state.
    });
  }, []);

  const selectTask = useCallback((id: string | null) => {
    if (id === null) { setSelectedTask(null); return; }
    setSelectedTask(tasks.find((t) => t.id === id) ?? null);
  }, [tasks]);

  const filterBy = useCallback((filters: TaskFilters) => {
    setActiveFilters(filters);
  }, []);

  return { tasks: filtered, tasksByStage, selectedTask, createTask, updateTask, moveTask, selectTask, filterBy, activeFilters };
}
