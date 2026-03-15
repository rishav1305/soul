import { useState } from 'react';
import type { ScoutProfile } from '../../hooks/useScout';

interface ProfilePanelProps {
  profile: ScoutProfile;
}

type Section = 'experience' | 'projects' | 'skills' | 'education' | 'certifications';

export function ProfilePanel({ profile }: ProfilePanelProps) {
  const [expanded, setExpanded] = useState<Set<Section>>(new Set(['experience']));

  const toggle = (section: Section) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  };

  return (
    <div className="space-y-2" data-testid="profile-panel">
      {/* Experience */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('experience')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-experience"
        >
          <span>Experience ({profile.experience.length})</span>
          <span className="text-fg-muted text-xs">{expanded.has('experience') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('experience') && (
          <div className="px-4 pb-3 space-y-3">
            {profile.experience.map((exp, i) => (
              <div key={i} className="border-l-2 border-soul/30 pl-3">
                <div className="text-sm font-medium text-fg">{exp.title}</div>
                <div className="text-xs text-fg-muted">{exp.company} - {exp.duration}</div>
                <p className="text-xs text-fg-muted mt-1">{exp.description}</p>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Projects */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('projects')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-projects"
        >
          <span>Projects ({profile.projects.length})</span>
          <span className="text-fg-muted text-xs">{expanded.has('projects') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('projects') && (
          <div className="px-4 pb-3 space-y-2">
            {profile.projects.map((proj, i) => (
              <div key={i}>
                <div className="text-sm font-medium text-fg">{proj.name}</div>
                <p className="text-xs text-fg-muted">{proj.description}</p>
                {proj.url && <div className="text-xs text-soul mt-0.5">{proj.url}</div>}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Skills */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('skills')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-skills"
        >
          <span>Skills ({profile.skills.length})</span>
          <span className="text-fg-muted text-xs">{expanded.has('skills') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('skills') && (
          <div className="px-4 pb-3">
            <div className="flex flex-wrap gap-1.5">
              {profile.skills.map(skill => (
                <span key={skill} className="px-2 py-0.5 text-xs rounded-full bg-soul/10 text-soul">
                  {skill}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Education */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('education')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-education"
        >
          <span>Education ({profile.education.length})</span>
          <span className="text-fg-muted text-xs">{expanded.has('education') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('education') && (
          <div className="px-4 pb-3 space-y-2">
            {profile.education.map((edu, i) => (
              <div key={i}>
                <div className="text-sm font-medium text-fg">{edu.degree}</div>
                <div className="text-xs text-fg-muted">{edu.institution} - {edu.year}</div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Certifications */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('certifications')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-certifications"
        >
          <span>Certifications ({profile.certifications.length})</span>
          <span className="text-fg-muted text-xs">{expanded.has('certifications') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('certifications') && (
          <div className="px-4 pb-3 space-y-2">
            {profile.certifications.map((cert, i) => (
              <div key={i}>
                <div className="text-sm font-medium text-fg">{cert.name}</div>
                <div className="text-xs text-fg-muted">{cert.issuer} - {cert.year}</div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
