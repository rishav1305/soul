import { useState, useEffect, useCallback, useRef } from 'react';
import type { Task, TaskActivity } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

export interface CreateTaskInput {
  title: string;
  description?: string;
  product?: string;
}

interface Comment {
  id: number;
  taskId: number;
  author: string;
  type: string;
  body: string;
  createdAt: string;
}

interface SyncResponse {
  tasks: Task[];
  deleted: number[];
  cursor: string;
  fullSync: boolean;
}

interface UseTaskSyncOptions {
  taskId?: number;
  mode?: 'kanban' | 'detail';
}

interface UseTaskSyncReturn {
  tasks: Task[];
  task: Task | null;
  activities: TaskActivity[];
  comments: Comment[];
  loading: boolean;
  error: string | null;
  connected: boolean;
  refresh: () => void;
  createTask: (input: CreateTaskInput) => Promise<Task>;
  updateTask: (id: number, fields: Partial<Task>) => Promise<Task>;
  deleteTask: (id: number) => Promise<void>;
  startTask: (id: number) => Promise<void>;
  stopTask: (id: number) => Promise<void>;
  addComment: (id: number, body: string) => Promise<void>;
}

export function useTaskSync(options?: UseTaskSyncOptions): UseTaskSyncReturn {
  const mode = options?.mode ?? 'kanban';
  const taskId = options?.taskId;

  const [taskMap, setTaskMap] = useState<Map<number, Task>>(new Map());
  const [activities, setActivities] = useState<TaskActivity[]>([]);
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const wasConnectedRef = useRef(false);

  const cursorRef = useRef<string>('');
  const lastActivityIdRef = useRef<number>(0);
  const lastCommentIdRef = useRef<number>(0);
  const inflightRef = useRef(false);

  // --- Helpers ---

  const applyTaskUpdate = useCallback((task: Task) => {
    setTaskMap(prev => {
      const existing = prev.get(task.id);
      if (existing && existing.seq >= task.seq) return prev;
      const next = new Map(prev);
      next.set(task.id, task);
      return next;
    });
  }, []);

  const applyTaskDelete = useCallback((id: number) => {
    setTaskMap(prev => {
      if (!prev.has(id)) return prev;
      const next = new Map(prev);
      next.delete(id);
      return next;
    });
  }, []);

  const appendActivity = useCallback((act: TaskActivity) => {
    setActivities(prev => {
      if (prev.some(a => a.id === act.id)) return prev;
      return [...prev, act];
    });
    if (act.id > lastActivityIdRef.current) {
      lastActivityIdRef.current = act.id;
    }
  }, []);

  const appendComment = useCallback((cmt: Comment) => {
    setComments(prev => {
      if (prev.some(c => c.id === cmt.id)) return prev;
      return [...prev, cmt];
    });
    if (cmt.id > lastCommentIdRef.current) {
      lastCommentIdRef.current = cmt.id;
    }
  }, []);

  // --- Initial Fetch (defined before WS listener so it can be referenced) ---

  const doFullSync = useCallback(async () => {
    try {
      const resp = await api.get<SyncResponse>('/api/tasks/sync');
      const map = new Map<number, Task>();
      for (const t of resp.tasks) map.set(t.id, t);
      setTaskMap(map);
      cursorRef.current = resp.cursor;
      setError(null);

      if (mode === 'detail' && taskId) {
        const [acts, cmts] = await Promise.all([
          api.get<TaskActivity[]>(`/api/tasks/${taskId}/activity?after=0`),
          api.get<Comment[]>(`/api/tasks/${taskId}/comments?after=0`),
        ]);
        setActivities(acts);
        setComments(cmts);
        if (acts.length > 0) lastActivityIdRef.current = acts[acts.length - 1]!.id;
        if (cmts.length > 0) lastCommentIdRef.current = cmts[cmts.length - 1]!.id;
      }
    } catch (err) {
      reportError('useTaskSync.fullSync', err);
      setError(err instanceof Error ? err.message : 'Sync failed');
    } finally {
      setLoading(false);
    }
  }, [mode, taskId]);

  // --- WS Event Listener ---

  useEffect(() => {
    const handler = (event: Event) => {
      try {
        const detail = (event as CustomEvent).detail;
        if (!detail?.type) return;
        const { type, data } = detail;

        const parse = (d: unknown) => (typeof d === 'string' ? JSON.parse(d) : d);

        switch (type) {
          case 'task.created':
          case 'task.updated':
          case 'task.stage_changed':
          case 'task.substep_changed': {
            applyTaskUpdate(parse(data));
            break;
          }
          case 'task.deleted': {
            const payload = parse(data);
            applyTaskDelete(payload.id);
            break;
          }
          case 'task.activity': {
            if (mode === 'detail' && taskId) {
              const payload = parse(data);
              if (payload.taskId === taskId) {
                appendActivity(payload.activity);
              }
            }
            break;
          }
          case 'task.comment': {
            if (mode === 'detail' && taskId) {
              const payload = parse(data);
              if (payload.taskId === taskId) {
                appendComment(payload.comment);
              }
            }
            break;
          }
        }
      } catch (err) {
        reportError('useTaskSync.wsEvent', err);
      }
    };

    window.addEventListener('ws:task-event', handler);

    const onConnected = () => {
      const wasDown = !wasConnectedRef.current;
      setConnected(true);
      wasConnectedRef.current = true;
      if (wasDown && cursorRef.current) doFullSync();
    };
    const onDisconnected = () => {
      setConnected(false);
      wasConnectedRef.current = false;
    };
    window.addEventListener('ws:connected', onConnected);
    window.addEventListener('ws:disconnected', onDisconnected);

    return () => {
      window.removeEventListener('ws:task-event', handler);
      window.removeEventListener('ws:connected', onConnected);
      window.removeEventListener('ws:disconnected', onDisconnected);
    };
  }, [taskId, mode, applyTaskUpdate, applyTaskDelete, appendActivity, appendComment, doFullSync]);

  useEffect(() => { doFullSync(); }, [doFullSync]);

  // --- Heartbeat ---

  useEffect(() => {
    const interval = mode === 'detail' ? 5000 : 30000;

    const tick = async () => {
      if (inflightRef.current) return;
      inflightRef.current = true;

      try {
        if (mode === 'kanban') {
          let resp: SyncResponse;
          try {
            resp = await api.get<SyncResponse>(
              `/api/tasks/sync?cursor=${encodeURIComponent(cursorRef.current)}`
            );
          } catch (fetchErr) {
            if (fetchErr instanceof Error && fetchErr.message.includes('400')) {
              cursorRef.current = '';
              resp = await api.get<SyncResponse>('/api/tasks/sync');
            } else {
              throw fetchErr;
            }
          }
          if (resp.fullSync) {
            const map = new Map<number, Task>();
            for (const t of resp.tasks) map.set(t.id, t);
            setTaskMap(map);
          } else {
            for (const t of resp.tasks) applyTaskUpdate(t);
            for (const id of resp.deleted) applyTaskDelete(id);
          }
          cursorRef.current = resp.cursor;
        } else if (mode === 'detail' && taskId) {
          const [taskResp, acts, cmts] = await Promise.all([
            api.get<Task>(`/api/tasks/${taskId}`),
            api.get<TaskActivity[]>(`/api/tasks/${taskId}/activity?after=${lastActivityIdRef.current}`),
            api.get<Comment[]>(`/api/tasks/${taskId}/comments?after=${lastCommentIdRef.current}`),
          ]);
          applyTaskUpdate(taskResp);
          for (const a of acts) appendActivity(a);
          for (const c of cmts) appendComment(c);
        }
        setError(null);
      } catch (err) {
        reportError('useTaskSync.heartbeat', err);
        setError(err instanceof Error ? err.message : 'Heartbeat failed');
      } finally {
        inflightRef.current = false;
      }
    };

    let timerId: ReturnType<typeof setInterval> | null = setInterval(tick, interval);

    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        if (timerId) { clearInterval(timerId); timerId = null; }
      } else {
        tick();
        if (!timerId) { timerId = setInterval(tick, interval); }
      }
    };
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      if (timerId) clearInterval(timerId);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [mode, taskId, applyTaskUpdate, applyTaskDelete, appendActivity, appendComment]);

  // --- Actions ---

  const createTask = useCallback(async (input: CreateTaskInput) => {
    const task = await api.post<Task>('/api/tasks', input);
    applyTaskUpdate(task);
    reportUsage('task.create', { taskId: task.id });
    return task;
  }, [applyTaskUpdate]);

  const updateTask = useCallback(async (id: number, fields: Partial<Task>) => {
    const task = await api.patch<Task>(`/api/tasks/${id}`, fields);
    applyTaskUpdate(task);
    reportUsage('task.update', { taskId: id, fields: Object.keys(fields) });
    return task;
  }, [applyTaskUpdate]);

  const deleteTask = useCallback(async (id: number) => {
    applyTaskDelete(id);
    try {
      await api.delete(`/api/tasks/${id}`);
      reportUsage('task.delete', { taskId: id });
    } catch (err) {
      doFullSync();
      throw err;
    }
  }, [applyTaskDelete, doFullSync]);

  const startTask = useCallback(async (id: number) => {
    await api.post(`/api/tasks/${id}/start`);
    reportUsage('task.start', { taskId: id });
  }, []);

  const stopTask = useCallback(async (id: number) => {
    await api.post(`/api/tasks/${id}/stop`);
    reportUsage('task.stop', { taskId: id });
  }, []);

  const addComment = useCallback(async (id: number, body: string) => {
    await api.post(`/api/tasks/${id}/comments`, { author: 'user', type: 'feedback', body });
    reportUsage('task.addComment', { taskId: id });
  }, []);

  const refresh = useCallback(() => { doFullSync(); }, [doFullSync]);

  // --- Derived state ---

  const tasks = Array.from(taskMap.values());
  const task = taskId ? taskMap.get(taskId) ?? null : null;

  return {
    tasks,
    task,
    activities,
    comments,
    loading,
    error,
    connected,
    refresh,
    createTask,
    updateTask,
    deleteTask,
    startTask,
    stopTask,
    addComment,
  };
}
