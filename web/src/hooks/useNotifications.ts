import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import { uuid } from '../lib/api.ts';
import type { StageNotification, TaskActivity, WSMessage } from '../lib/types.ts';

const MAX_TOASTS = 5;
const AUTO_DISMISS_MS = 4000;

export function useNotifications(tasks: { id: number; title: string }[], enabled: boolean) {
  const { onMessage } = useWebSocket();
  const [toasts, setToasts] = useState<StageNotification[]>([]);

  useEffect(() => {
    if (!enabled) return;

    const unsubscribe = onMessage((msg: WSMessage) => {
      if (msg.type !== 'task.activity') return;
      const activity = msg.data as TaskActivity;
      if (!activity || activity.type !== 'stage') return;

      // Parse "backlog → active" from content
      const match = activity.content.match(/(\w+)\s*(?:→|->)\s*(\w+)/);
      if (!match) return;

      const fromStage = match[1] as StageNotification['fromStage'];
      const toStage = match[2] as StageNotification['toStage'];

      const task = tasks.find((t) => t.id === activity.task_id);
      const taskTitle = task?.title ?? `Task #${activity.task_id}`;

      const notification: StageNotification = {
        id: uuid(),
        taskId: activity.task_id,
        taskTitle,
        fromStage,
        toStage,
        time: activity.time || new Date().toISOString(),
      };

      setToasts((prev) => [notification, ...prev].slice(0, MAX_TOASTS));

      // Auto-dismiss after 4s
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== notification.id));
      }, AUTO_DISMISS_MS);
    });

    return unsubscribe;
  }, [onMessage, tasks, enabled]);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return { toasts, dismiss };
}
