import { useState, useEffect, useCallback } from 'react';
import type { Task, TaskStage } from '../lib/types';
import { api } from '../lib/api';

interface UseTasksReturn {
  tasks: Task[];
  loading: boolean;
  error: string | null;
  refresh: () => void;
  createTask: (title: string, description?: string) => Promise<Task>;
  updateTask: (id: number, fields: Partial<Pick<Task, 'title' | 'description' | 'stage'>>) => Promise<Task>;
  deleteTask: (id: number) => Promise<void>;
  startTask: (id: number) => Promise<void>;
  stopTask: (id: number) => Promise<void>;
}

export function useTasks(): UseTasksReturn {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    api.get<Task[]>('/api/tasks')
      .then(data => {
        setTasks(data);
        setError(null);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const createTask = useCallback(async (title: string, description = '') => {
    const task = await api.post<Task>('/api/tasks', { title, description });
    setTasks(prev => [task, ...prev]);
    return task;
  }, []);

  const updateTask = useCallback(async (id: number, fields: Partial<Pick<Task, 'title' | 'description' | 'stage'>>) => {
    const task = await api.patch<Task>(`/api/tasks/${id}`, fields);
    setTasks(prev => prev.map(t => t.id === id ? task : t));
    return task;
  }, []);

  const deleteTask = useCallback(async (id: number) => {
    await api.delete(`/api/tasks/${id}`);
    setTasks(prev => prev.filter(t => t.id !== id));
  }, []);

  const startTask = useCallback(async (id: number) => {
    await api.post<{ status: string }>(`/api/tasks/${id}/start`);
    refresh();
  }, [refresh]);

  const stopTask = useCallback(async (id: number) => {
    await api.post<{ status: string }>(`/api/tasks/${id}/stop`);
    refresh();
  }, [refresh]);

  return { tasks, loading, error, refresh, createTask, updateTask, deleteTask, startTask, stopTask };
}
