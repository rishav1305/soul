import type { ProjectSummary } from '../lib/types';

const statusColor: Record<string, string> = {
  backlog: 'bg-overlay text-fg-secondary',
  active: 'bg-blue-500/20 text-blue-400',
  measuring: 'bg-amber-500/20 text-amber-400',
  documenting: 'bg-purple-500/20 text-purple-400',
  shipped: 'bg-emerald-500/20 text-emerald-400',
};

interface ProjectCardProps {
  project: ProjectSummary;
  onClick: () => void;
}

export function ProjectCard({ project, onClick }: ProjectCardProps) {
  const progressPercent = project.milestones_total > 0
    ? Math.round((project.milestones_done / project.milestones_total) * 100)
    : 0;

  return (
    <button
      onClick={onClick}
      className="w-full bg-surface rounded-lg p-4 hover:bg-elevated transition-colors text-left space-y-3"
      data-testid={`project-card-${project.id}`}
    >
      {/* Name + status */}
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium text-fg truncate">{project.name}</span>
        <span className={`px-2 py-0.5 text-xs rounded-full shrink-0 ${statusColor[project.status] ?? 'bg-overlay text-fg-secondary'}`}>
          {project.status}
        </span>
      </div>

      {/* Phase */}
      <div className="text-xs text-fg-muted">Phase {project.phase}</div>

      {/* Description */}
      <p className="text-xs text-fg-muted line-clamp-2">{project.description}</p>

      {/* Milestone progress */}
      <div className="space-y-1">
        <div className="flex items-center justify-between text-xs text-fg-muted">
          <span>Milestones</span>
          <span>{project.milestones_done}/{project.milestones_total}</span>
        </div>
        <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
          <div
            className="h-full bg-emerald-500 rounded-full transition-all"
            style={{ width: `${progressPercent}%` }}
          />
        </div>
      </div>

      {/* Keywords + hours */}
      <div className="flex items-center justify-between text-xs text-fg-muted">
        <span>{project.keyword_count} keywords</span>
        <span>{project.hours_actual}/{project.hours_estimated}h</span>
      </div>
    </button>
  );
}
