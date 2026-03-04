import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { StageNotification, TaskActivity, TaskStage, WSMessage } from '../lib/types.ts';
import type { PlannerTask } from '../lib/types.ts';

function uuid(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/** Parse "backlog → active" style content from stage activity events */
function parseStageChange(content: string): { from: TaskStage; to: TaskStage } | null {
  const match = content.match(/(\w+)\s*[→\->]+\s*(\w+)/);
  if (!match) return null;
  const validStages: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];
  const from = match[1] as TaskStage;
  const to = match[2] as TaskStage;
  if (!validStages.includes(from) || !validStages.includes(to)) return null;
  return { from, to };
}

const AUTO_DISMISS_MS = 4500;
const MAX_VISIBLE = 5;

export function useNotifications(tasks: PlannerTask[]) {
  const { onMessage } = useWebSocket();
  const [notifications, setNotifications] = useState<StageNotification[]>([]);

  useEffect(() => {
    const unsubscribe = onMessage((msg: WSMessage) => {
      if (msg.type !== 'task.activity') return;
      const activity = msg.data as TaskActivity;
      if (!activity?.task_id || activity.type !== 'stage') return;

      const parsed = parseStageChange(activity.content);
      if (!parsed) return;

      const task = tasks.find((t) => t.id === activity.task_id);
      const taskTitle = task?.title ?? `Task #${activity.task_id}`;

      const notification: StageNotification = {
        id: uuid(),
        taskId: activity.task_id,
        taskTitle,
        fromStage: parsed.from,
        toStage: parsed.to,
        time: new Date(),
      };

      setNotifications((prev) => [notification, ...prev].slice(0, MAX_VISIBLE));

      // Auto-dismiss after 4.5s
      setTimeout(() => {
        setNotifications((prev) => prev.filter((n) => n.id !== notification.id));
      }, AUTO_DISMISS_MS);
    });

    return unsubscribe;
  }, [onMessage, tasks]);

  const dismiss = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  }, []);

  return { notifications, dismiss };
}
