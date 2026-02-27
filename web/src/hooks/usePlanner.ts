import { useState, useEffect, useCallback, useMemo } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { PlannerTask, TaskStage, WSMessage } from '../lib/types.ts';

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

  // Fetch all tasks on mount
  useEffect(() => {
    let cancelled = false;

    async function fetchTasks() {
      try {
        const res = await fetch('/api/tasks');
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
        case 'task.updated': {
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
      const res = await fetch('/api/tasks', {
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
      const res = await fetch(`/api/tasks/${id}`, {
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
    const res = await fetch(`/api/tasks/${id}`, {
      method: 'DELETE',
    });
    if (!res.ok) throw new Error(`Failed to delete task: ${res.status}`);
  }, []);

  const moveTask = useCallback(
    async (id: number, stage: TaskStage, comment: string) => {
      const res = await fetch(`/api/tasks/${id}/move`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ stage, comment }),
      });
      if (!res.ok) throw new Error(`Failed to move task: ${res.status}`);
      return (await res.json()) as PlannerTask;
    },
    [],
  );

  return { tasks, tasksByStage, loading, createTask, updateTask, deleteTask, moveTask };
}
