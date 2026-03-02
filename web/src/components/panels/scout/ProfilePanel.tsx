import { useState } from 'react';
import type {
  ProfileData,
  ProfileSiteConfig,
  ProfileExperience,
  ProfileSkillCategory,
  ProfileProject,
  ProfileEducation,
} from '../../../lib/types.ts';

interface ProfilePanelProps {
  profile: ProfileData | null;
  loading: boolean;
  pulling: boolean;
  error: string | null;
  onPull: () => void;
}

function CollapsibleSection({
  title,
  count,
  defaultOpen = false,
  children,
}: {
  title: string;
  count: number;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="space-y-1.5">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center justify-between w-full text-left cursor-pointer"
      >
        <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
          {title}
          <span className="ml-1.5 text-[10px] font-normal text-fg-muted/60">({count})</span>
        </h3>
        <span className="text-[10px] text-fg-muted">{open ? '\u25B4' : '\u25BE'}</span>
      </button>
      {open && <div className="space-y-1.5">{children}</div>}
    </div>
  );
}

function IdentitySection({ config }: { config: ProfileSiteConfig }) {
  const yearsExp = new Date().getFullYear() - config.years_experience_start_year;
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
        Identity
      </h3>
      <div className="rounded-lg bg-elevated/50 border border-border-subtle p-3 space-y-2">
        <div>
          <div className="text-sm font-semibold text-fg">{config.name}</div>
          <div className="text-[11px] text-soul">{config.title}</div>
        </div>
        <p className="text-[10px] text-fg-secondary leading-relaxed">{config.short_bio}</p>
        <div className="flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-fg-muted">
          <span>{config.location}</span>
          <span>{config.email}</span>
          <span>{yearsExp}+ years</span>
        </div>
        {config.social_media && (
          <div className="flex flex-wrap gap-1.5">
            {Object.entries(config.social_media).map(([platform, url]) => (
              <a
                key={platform}
                href={url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-medium bg-accent/10 text-accent hover:bg-accent/20 capitalize"
              >
                {platform}
              </a>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function ExperienceCard({ exp }: { exp: ProfileExperience }) {
  const [open, setOpen] = useState(false);
  const isCurrent = exp.end_date === null;
  return (
    <div className="rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2 space-y-1">
      <div
        className="flex items-start justify-between gap-2 cursor-pointer"
        onClick={() => setOpen(!open)}
        onKeyDown={(e) => e.key === 'Enter' && setOpen(!open)}
        role="button"
        tabIndex={0}
      >
        <div className="min-w-0">
          <div className="text-xs font-medium text-fg">{exp.role}</div>
          <div className="text-[10px] text-fg-secondary">{exp.company}</div>
        </div>
        <div className="shrink-0 text-right">
          <div className="text-[10px] text-fg-muted">{exp.period}</div>
          {isCurrent && (
            <span className="text-[9px] font-medium text-emerald-400">current</span>
          )}
        </div>
      </div>
      {open && (
        <div className="pt-1 space-y-1.5 border-t border-white/5">
          {exp.location && (
            <div className="text-[10px] text-fg-muted">{exp.location}</div>
          )}
          {exp.achievements && exp.achievements.length > 0 && (
            <ul className="space-y-0.5">
              {exp.achievements.slice(0, 5).map((a, i) => (
                <li key={i} className="text-[10px] text-fg-secondary leading-snug pl-2 border-l border-soul/30">
                  {a.replace(/\*\*/g, '')}
                </li>
              ))}
            </ul>
          )}
          {exp.tech_stack && exp.tech_stack.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {exp.tech_stack.map((t) => (
                <span key={t} className="px-1 py-0.5 rounded text-[9px] bg-zinc-700/50 text-fg-muted">
                  {t}
                </span>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function SkillCategoryCard({ cat }: { cat: ProfileSkillCategory }) {
  return (
    <div className="rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2">
      <div className="text-[11px] font-medium text-fg mb-1">{cat.category_name}</div>
      <div className="flex flex-wrap gap-1">
        {cat.skills.map((s) => (
          <span
            key={s.name}
            className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] bg-soul/10 text-soul"
          >
            {s.name}
            <span className="opacity-50">{s.level}/5</span>
          </span>
        ))}
      </div>
    </div>
  );
}

function ProjectCard({ project }: { project: ProfileProject }) {
  return (
    <div className="rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2 space-y-1">
      <div className="flex items-start justify-between gap-2">
        <div className="text-xs font-medium text-fg">{project.title}</div>
        {project.category && (
          <span className="shrink-0 text-[9px] px-1.5 py-0.5 rounded bg-zinc-700/50 text-fg-muted">
            {project.category}
          </span>
        )}
      </div>
      <div className="text-[10px] text-fg-secondary">{project.company}</div>
      <p className="text-[10px] text-fg-muted leading-snug line-clamp-2">
        {project.short_description}
      </p>
      {project.tech_stack && project.tech_stack.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {project.tech_stack.slice(0, 6).map((t) => (
            <span key={t} className="px-1 py-0.5 rounded text-[9px] bg-zinc-700/50 text-fg-muted">
              {t}
            </span>
          ))}
          {project.tech_stack.length > 6 && (
            <span className="text-[9px] text-fg-muted">+{project.tech_stack.length - 6}</span>
          )}
        </div>
      )}
    </div>
  );
}

function EducationCard({ edu }: { edu: ProfileEducation }) {
  return (
    <div className="rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2">
      <div className="text-xs font-medium text-fg">{edu.institution}</div>
      <div className="text-[10px] text-fg-secondary">
        {edu.degree} &mdash; {edu.focus_area}
      </div>
      <div className="text-[10px] text-fg-muted">{edu.period}</div>
    </div>
  );
}

export default function ProfilePanel({ profile, loading, pulling, error, onPull }: ProfilePanelProps) {
  if (loading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="flex items-center gap-2 text-xs text-fg-muted">
          <span className="w-3 h-3 border-2 border-soul/40 border-t-soul rounded-full animate-spin" />
          Loading profile...
        </div>
      </div>
    );
  }

  if (error && !profile) {
    return (
      <div className="p-4 space-y-3">
        <div className="rounded-lg bg-red-500/10 border border-red-500/20 px-3 py-2 text-xs text-red-400">
          {error}
        </div>
        <button
          type="button"
          onClick={onPull}
          disabled={pulling}
          className="text-[10px] px-2 py-1 rounded bg-soul/15 text-soul hover:bg-soul/25 disabled:opacity-50 cursor-pointer"
        >
          {pulling ? 'Pulling...' : 'Pull from Supabase'}
        </button>
      </div>
    );
  }

  if (!profile) {
    return (
      <div className="flex flex-col items-center justify-center h-32 gap-2 text-center px-6">
        <p className="text-xs text-fg-muted">
          No profile data available. Pull from Supabase to get started.
        </p>
        <button
          type="button"
          onClick={onPull}
          disabled={pulling}
          className="text-[10px] px-2.5 py-1 rounded bg-soul/15 text-soul hover:bg-soul/25 disabled:opacity-50 cursor-pointer"
        >
          {pulling ? 'Pulling...' : 'Pull from Supabase'}
        </button>
      </div>
    );
  }

  const config = profile.site_config?.[0];

  return (
    <div className="px-4 py-3 space-y-4">
      {/* Pull button + status */}
      <div className="flex items-center justify-between">
        <div className="text-[10px] text-fg-muted">
          {profile.experience?.length ?? 0} roles &middot; {profile.projects?.length ?? 0} projects &middot; {profile.skill_categories?.length ?? 0} skill groups
        </div>
        <button
          type="button"
          onClick={onPull}
          disabled={pulling}
          className="text-[10px] px-2 py-1 rounded bg-soul/15 text-soul hover:bg-soul/25 disabled:opacity-50 cursor-pointer"
        >
          {pulling ? 'Pulling...' : 'Sync from Supabase'}
        </button>
      </div>

      {error && (
        <div className="rounded-lg bg-red-500/10 border border-red-500/20 px-3 py-1.5 text-[10px] text-red-400">
          {error}
        </div>
      )}

      {/* Identity */}
      {config && <IdentitySection config={config} />}

      {/* Experience */}
      {profile.experience && profile.experience.length > 0 && (
        <CollapsibleSection title="Experience" count={profile.experience.length} defaultOpen>
          {profile.experience.map((exp) => (
            <ExperienceCard key={exp.id} exp={exp} />
          ))}
        </CollapsibleSection>
      )}

      {/* Skills */}
      {profile.skill_categories && profile.skill_categories.length > 0 && (
        <CollapsibleSection title="Skills" count={profile.skill_categories.length} defaultOpen>
          {profile.skill_categories.map((cat) => (
            <SkillCategoryCard key={cat.id} cat={cat} />
          ))}
        </CollapsibleSection>
      )}

      {/* Projects */}
      {profile.projects && profile.projects.length > 0 && (
        <CollapsibleSection title="Projects" count={profile.projects.length}>
          {profile.projects.map((proj) => (
            <ProjectCard key={proj.id} project={proj} />
          ))}
        </CollapsibleSection>
      )}

      {/* Education */}
      {profile.education && profile.education.length > 0 && (
        <CollapsibleSection title="Education" count={profile.education.length}>
          {profile.education.map((edu) => (
            <EducationCard key={edu.id} edu={edu} />
          ))}
        </CollapsibleSection>
      )}
    </div>
  );
}
