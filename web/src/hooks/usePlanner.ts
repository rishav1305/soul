import { useState, useEffect, useCallback, useMemo } from 'react';
import { useWebSocketCtx as useWebSocket } from './useWebSocketContext.ts';
import { authFetch } from '../lib/api.ts';
import type { PlannerTask, TaskStage, PlannerActivity, TaskComment, TaskActivityEvent, TaskCommentEvent, WSMessage } from '../lib/types.ts';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

function emptyByStage(): Record<TaskStage, PlannerTask[]> {
  return {
    backlog: [],
    brainstorm: [],
    active: [],
    blocked: [],
    validation: [],
    done: [],
  };
}

export function usePlanner() {
  const { onMessage } = useWebSocket();
  const [tasks, setTasks] = useState<PlannerTask[]>([]);
  const [loading, setLoading] = useState(true);
  // Track live activity streams per task. Key = taskID.
  const [taskActivities, setTaskActivities] = useState<Record<number, PlannerActivity[]>>({});
  // Track streaming output per task (accumulated tokens).
  const [taskStreams, setTaskStreams] = useState<Record<number, string>>({});
  const [taskComments, setTaskComments] = useState<Record<number, TaskComment[]>>({});

  // Fetch all tasks on mount
  useEffect(() => {
    let cancelled = false;

    async function fetchTasks() {
      try {
        const res = await authFetch('/api/tasks');
        if (!res.ok) throw new Error(`Failed to fetch tasks: ${res.status}`);
        const data: PlannerTask[] = await res.json();
        if (!cancelled) {
          setTasks(data);
        }
      } catch (err) {
        console.error('Failed to fetch tasks:', err);
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    fetchTasks();
    return () => { cancelled = true; };
  }, []);

  // Listen for WebSocket events
  useEffect(() => {
    const unsubscribe = onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case 'task.created': {
          const task = msg.data as PlannerTask;
          setTasks((prev) => [...prev, task]);
          break;
        }
        case 'task.updated':
        case 'task.stage_changed': {
          // Go backend fires task.stage_changed on stage transitions,
          // task.updated on other field changes. Both carry a full Task payload.
          const task = msg.data as PlannerTask;
          setTasks((prev) =>
            prev.map((t) => (t.id === task.id ? task : t)),
          );
          break;
        }
        case 'task.deleted': {
          const data = msg.data as { id: number };
          setTasks((prev) => prev.filter((t) => t.id !== data.id));
          break;
        }
        case 'task.activity': {
          // Go broadcasts TaskActivity wrapper: { taskId, activity: Activity{...} }
          const wrapper = msg.data as TaskActivityEvent;
          const activity = wrapper?.activity;
          if (!activity?.taskId) break;

          // Go executor prefixes event types with "agent." (e.g. agent.tool_call,
          // agent.end_turn, agent.hit_limit). Check with endsWith for future-proofing.
          if (activity.eventType.endsWith('.tool_call')) {
            // Agent tool call — accumulate streaming output.
            setTaskStreams((prev) => ({
              ...prev,
              [activity.taskId]: (prev[activity.taskId] || '') + activity.data,
            }));
          } else if (activity.eventType.endsWith('.end_turn') || activity.eventType.endsWith('.hit_limit')) {
            // Agent finished — clear stream.
            setTaskStreams((prev) => {
              const next = { ...prev };
              delete next[activity.taskId];
              return next;
            });
          }

          // Store non-streaming activities in the log.
          if (!activity.eventType.endsWith('.tool_call')) {
            setTaskActivities((prev) => ({
              ...prev,
              [activity.taskId]: [...(prev[activity.taskId] || []), activity].slice(-50),
            }));
          }
          break;
        }
        case 'task.comment': {
          // Go broadcasts TaskComment wrapper: { taskId, comment: Comment{...} }
          const wrapper = msg.data as TaskCommentEvent;
          const comment = wrapper?.comment;
          if (!comment?.taskId) break;
          setTaskComments((prev) => ({
            ...prev,
            [comment.taskId]: [...(prev[comment.taskId] || []), comment],
          }));
          break;
        }
      }
    });

    return unsubscribe;
  }, [onMessage]);

  const tasksByStage = useMemo(() => {
    const grouped = emptyByStage();
    for (const stage of STAGES) {
      grouped[stage] = tasks.filter((t) => t.stage === stage);
    }
    return grouped;
  }, [tasks]);

  const createTask = useCallback(
    async (title: string, description: string, priority: number, product: string) => {
      const res = await authFetch('/api/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, description, priority, product }),
      });
      if (!res.ok) throw new Error(`Failed to create task: ${res.status}`);
      return (await res.json()) as PlannerTask;
    },
    [],
  );

  const updateTask = useCallback(
    async (id: number, updates: Partial<PlannerTask>) => {
      const res = await authFetch(`/api/tasks/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      });
      if (!res.ok) throw new Error(`Failed to update task: ${res.status}`);
      return (await res.json()) as PlannerTask;
    },
    [],
  );

  const deleteTask = useCallback(async (id: number) => {
    const res = await authFetch(`/api/tasks/${id}`, {
      method: 'DELETE',
    });
    if (!res.ok) throw new Error(`Failed to delete task: ${res.status}`);
  }, []);

  const moveTask = useCallback(
    async (id: number, stage: TaskStage, comment: string) => {
      // v2 backend: PATCH /api/tasks/{id} to change stage (no /move endpoint).
      const res = await authFetch(`/api/tasks/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ stage }),
      });
      if (!res.ok) throw new Error(`Failed to move task: ${res.status}`);
      const task = (await res.json()) as PlannerTask;

      // Post move comment separately if provided.
      if (comment.trim()) {
        await authFetch(`/api/tasks/${id}/comments`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ author: 'user', type: 'stage_change', body: comment }),
        });
      }

      return task;
    },
    [],
  );

  const fetchComments = useCallback(async (taskId: number) => {
    const res = await authFetch(`/api/tasks/${taskId}/comments`);
    if (!res.ok) throw new Error(`Failed to fetch comments: ${res.status}`);
    const data: TaskComment[] = await res.json();
    setTaskComments((prev) => ({ ...prev, [taskId]: data }));
    return data;
  }, []);

  const addComment = useCallback(async (taskId: number, body: string) => {
    const res = await authFetch(`/api/tasks/${taskId}/comments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ author: 'user', type: 'feedback', body }),
    });
    if (!res.ok) throw new Error(`Failed to add comment: ${res.status}`);
    return (await res.json()) as TaskComment;
  }, []);

  return {
    tasks,
    tasksByStage,
    loading,
    taskActivities,
    taskStreams,
    createTask,
    updateTask,
    deleteTask,
    moveTask,
    taskComments,
    fetchComments,
    addComment,
  };
}
