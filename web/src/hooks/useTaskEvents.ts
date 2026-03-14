import { useEffect, useRef } from 'react';
import type { Task } from '../lib/types';

type TaskEventHandler = (eventType: string, task: Task) => void;

export function useTaskEvents(onEvent: TaskEventHandler): void {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    const handler = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type && msg.type.startsWith('task.') && msg.data) {
          const task = typeof msg.data === 'string' ? JSON.parse(msg.data) : msg.data;
          handlerRef.current(msg.type, task);
        }
      } catch {
        // Not a task event or invalid JSON — ignore.
      }
    };

    // The WebSocket connection is managed by useWebSocket in useChat.
    // We listen on the global message event bus instead.
    window.addEventListener('ws:task-event', handler as EventListener);
    return () => window.removeEventListener('ws:task-event', handler as EventListener);
  }, []);
}
