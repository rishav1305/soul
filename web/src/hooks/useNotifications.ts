import { useState, useEffect, useCallback } from 'react';
import { useWebSocketCtx as useWebSocket } from './useWebSocketContext.ts';
import { uuid } from '../lib/api.ts';
import type { StageNotification, PlannerActivity, WSMessage } from '../lib/types.ts';

const MAX_TOASTS = 5;
const AUTO_DISMISS_MS = 4000;

export function useNotifications(tasks: { id: number; title: string }[], enabled: boolean) {
  const { onMessage } = useWebSocket();
  const [toasts, setToasts] = useState<StageNotification[]>([]);

  useEffect(() => {
    if (!enabled) return;

    const unsubscribe = onMessage((msg: WSMessage) => {
      if (msg.type !== 'task.activity') return;
      // Go Activity shape: { id, taskId, eventType, data, createdAt }
      const activity = msg.data as PlannerActivity;
      if (!activity || activity.eventType !== 'task.stage_changed') return;

      // Parse "backlog → active" from data field
      const match = activity.data.match(/(\w+)\s*(?:→|->)\s*(\w+)/);
      if (!match) return;

      const fromStage = match[1] as StageNotification['fromStage'];
      const toStage = match[2] as StageNotification['toStage'];

      const task = tasks.find((t) => t.id === activity.taskId);
      const taskTitle = task?.title ?? `Task #${activity.taskId}`;

      const notification: StageNotification = {
        id: uuid(),
        taskId: activity.taskId,
        taskTitle,
        fromStage,
        toStage,
        time: activity.createdAt || new Date().toISOString(),
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
