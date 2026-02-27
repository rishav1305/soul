import type { TaskStage } from '../../lib/types.ts';

export const ValidTransitions: Record<TaskStage, TaskStage[]> = {
  backlog: ['brainstorm', 'active'],
  brainstorm: ['active'],
  active: ['blocked', 'validation'],
  blocked: ['active'],
  validation: ['done', 'active'],
  done: [],
};
