import { useState } from 'react';
import { api } from '../../lib/api';

interface ProfileGateProps {
  onAction?: (action: string) => void;
}

interface AuditResult {
  score: number;
  strengths: string[];
  gaps: string[];
  recommendations: string[];
  keyword_suggestions: string[];
}

interface LinkedInUpdateResult {
  updated_text: string;
}

interface GitHubReadmeResult {
  markdown: string;
}

interface TestimonialResult {
  message: string;
}

interface PinResult {
  recommended_pins: string[];
}

type SectionKey = 'audit' | 'linkedin' | 'github' | 'testimonial' | 'pin';
type Platform = 'linkedin' | 'github';
type LinkedInSection = 'headline' | 'about' | 'experience';

export function ProfileGate({ onAction }: ProfileGateProps) {
  // Section expansion
  const [expanded, setExpanded] = useState<Set<SectionKey>>(new Set(['audit']));

  // Audit state
  const [auditPlatform, setAuditPlatform] = useState<Platform>('linkedin');
  const [profileText, setProfileText] = useState('');
  const [auditResult, setAuditResult] = useState<AuditResult | null>(null);

  // LinkedIn state
  const [linkedinSection, setLinkedinSection] = useState<LinkedInSection>('headline');
  const [linkedinContent, setLinkedinContent] = useState('');
  const [linkedinResult, setLinkedinResult] = useState<LinkedInUpdateResult | null>(null);

  // GitHub README state
  const [repoName, setRepoName] = useState('');
  const [repoDescription, setRepoDescription] = useState('');
  const [readmeResult, setReadmeResult] = useState<GitHubReadmeResult | null>(null);

  // Testimonial state
  const [testimonialLeadId, setTestimonialLeadId] = useState('');
  const [testimonialResult, setTestimonialResult] = useState<TestimonialResult | null>(null);

  // Pin state
  const [pinPlatform, setPinPlatform] = useState<Platform>('linkedin');
  const [pinResult, setPinResult] = useState<PinResult | null>(null);

  // Shared state
  const [loading, setLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const toggle = (section: SectionKey) => {
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

  const handleAudit = async () => {
    if (!profileText.trim()) return;
    setLoading('audit');
    setError(null);
    try {
      const result = await api.post<AuditResult>('/api/ai/profile-audit', {
        platform: auditPlatform,
        current_profile: profileText,
      });
      setAuditResult(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const handleLinkedIn = async () => {
    if (!linkedinContent.trim()) return;
    setLoading('linkedin');
    setError(null);
    try {
      const result = await api.post<LinkedInUpdateResult>('/api/ai/linkedin-update', {
        section: linkedinSection,
        current_content: linkedinContent,
      });
      setLinkedinResult(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const handleGitHub = async () => {
    if (!repoName.trim() || !repoDescription.trim()) return;
    setLoading('github');
    setError(null);
    try {
      const result = await api.post<GitHubReadmeResult>('/api/ai/github-readme', {
        repo_name: repoName,
        description: repoDescription,
      });
      setReadmeResult(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const handleTestimonial = async () => {
    const leadId = parseInt(testimonialLeadId, 10);
    if (isNaN(leadId)) return;
    setLoading('testimonial');
    setError(null);
    try {
      const result = await api.post<TestimonialResult>('/api/ai/testimonial-request', {
        lead_id: leadId,
      });
      setTestimonialResult(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const handlePin = async () => {
    setLoading('pin');
    setError(null);
    try {
      const result = await api.post<PinResult>('/api/ai/pin-recommendation', {
        platform: pinPlatform,
      });
      setPinResult(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const scoreColor = (score: number) => {
    if (score >= 80) return 'text-emerald-400';
    if (score >= 50) return 'text-amber-400';
    return 'text-red-400';
  };

  // Suppress onAction unused warning — it's wired by parent
  void onAction;

  return (
    <div className="space-y-4" data-testid="profile-gate">
      {error && (
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="profile-gate-error">
          {error}
        </div>
      )}

      {/* Profile Audit */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('audit')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-audit"
        >
          <span>Profile Audit</span>
          <span className="text-fg-muted text-xs">{expanded.has('audit') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('audit') && (
          <div className="px-4 pb-4 space-y-3">
            <div>
              <label className="text-xs text-fg-muted block mb-1">Platform</label>
              <select
                value={auditPlatform}
                onChange={e => setAuditPlatform(e.target.value as Platform)}
                className="soul-select"
                data-testid="profile-audit-platform"
              >
                <option value="linkedin">LinkedIn</option>
                <option value="github">GitHub</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-fg-muted block mb-1">Current Profile Text</label>
              <textarea
                value={profileText}
                onChange={e => setProfileText(e.target.value)}
                placeholder="Paste your current profile text here..."
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50 resize-none"
                rows={4}
                data-testid="profile-audit-text"
              />
            </div>
            <button
              onClick={handleAudit}
              disabled={loading === 'audit' || !profileText.trim()}
              className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="profile-audit-btn"
            >
              {loading === 'audit' ? 'Generating...' : 'Run Audit'}
            </button>

            {auditResult && (
              <div className="space-y-3 mt-3" data-testid="profile-audit-result">
                {/* Score gauge */}
                <div className="flex items-center gap-3">
                  <div className="relative w-16 h-16">
                    <svg viewBox="0 0 36 36" className="w-16 h-16 -rotate-90">
                      <circle cx="18" cy="18" r="15.5" fill="none" stroke="currentColor" strokeWidth="2" className="text-elevated" />
                      <circle
                        cx="18" cy="18" r="15.5" fill="none" strokeWidth="2"
                        strokeDasharray={`${auditResult.score * 0.975} 97.5`}
                        strokeLinecap="round"
                        className={scoreColor(auditResult.score)}
                        stroke="currentColor"
                      />
                    </svg>
                    <span className={`absolute inset-0 flex items-center justify-center text-sm font-bold ${scoreColor(auditResult.score)}`}>
                      {auditResult.score}
                    </span>
                  </div>
                  <div>
                    <div className="text-sm font-medium text-fg">Profile Score</div>
                    <div className="text-xs text-fg-muted">
                      {auditResult.score >= 80 ? 'Strong profile' : auditResult.score >= 50 ? 'Needs improvement' : 'Major gaps'}
                    </div>
                  </div>
                </div>

                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Strengths</h4>
                  <ul className="space-y-1">
                    {auditResult.strengths.map((s, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-emerald-400 text-xs mt-0.5">&#x2022;</span>
                        <span>{s}</span>
                      </li>
                    ))}
                  </ul>
                </div>

                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Gaps</h4>
                  <ul className="space-y-1">
                    {auditResult.gaps.map((g, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-red-400 text-xs mt-0.5">&#x2022;</span>
                        <span>{g}</span>
                      </li>
                    ))}
                  </ul>
                </div>

                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Recommendations</h4>
                  <ul className="space-y-1">
                    {auditResult.recommendations.map((r, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                        <span>{r}</span>
                      </li>
                    ))}
                  </ul>
                </div>

                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Keyword Suggestions</h4>
                  <div className="flex flex-wrap gap-1.5">
                    {auditResult.keyword_suggestions.map(kw => (
                      <span key={kw} className="px-2 py-0.5 text-xs rounded-full bg-soul/10 text-soul">{kw}</span>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* LinkedIn Update */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('linkedin')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-linkedin"
        >
          <span>LinkedIn Update</span>
          <span className="text-fg-muted text-xs">{expanded.has('linkedin') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('linkedin') && (
          <div className="px-4 pb-4 space-y-3">
            <div>
              <label className="text-xs text-fg-muted block mb-1">Section</label>
              <select
                value={linkedinSection}
                onChange={e => setLinkedinSection(e.target.value as LinkedInSection)}
                className="soul-select"
                data-testid="profile-linkedin-section"
              >
                <option value="headline">Headline</option>
                <option value="about">About</option>
                <option value="experience">Experience</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-fg-muted block mb-1">Current Content</label>
              <textarea
                value={linkedinContent}
                onChange={e => setLinkedinContent(e.target.value)}
                placeholder="Paste your current section content..."
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50 resize-none"
                rows={3}
                data-testid="profile-linkedin-content"
              />
            </div>
            <button
              onClick={handleLinkedIn}
              disabled={loading === 'linkedin' || !linkedinContent.trim()}
              className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="profile-linkedin-btn"
            >
              {loading === 'linkedin' ? 'Generating...' : 'Generate Update'}
            </button>

            {linkedinResult && (
              <div className="mt-3" data-testid="profile-linkedin-result">
                <h4 className="text-xs text-fg-muted font-medium mb-1">Updated Text</h4>
                <pre className="text-sm text-fg whitespace-pre-wrap font-sans bg-elevated rounded-lg p-3">{linkedinResult.updated_text}</pre>
              </div>
            )}
          </div>
        )}
      </div>

      {/* GitHub README */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('github')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-github"
        >
          <span>GitHub README</span>
          <span className="text-fg-muted text-xs">{expanded.has('github') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('github') && (
          <div className="px-4 pb-4 space-y-3">
            <div>
              <label className="text-xs text-fg-muted block mb-1">Repository Name</label>
              <input
                type="text"
                value={repoName}
                onChange={e => setRepoName(e.target.value)}
                placeholder="e.g., soul-v2"
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50"
                data-testid="profile-github-repo-name"
              />
            </div>
            <div>
              <label className="text-xs text-fg-muted block mb-1">Description</label>
              <textarea
                value={repoDescription}
                onChange={e => setRepoDescription(e.target.value)}
                placeholder="Brief description of the project..."
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50 resize-none"
                rows={3}
                data-testid="profile-github-description"
              />
            </div>
            <button
              onClick={handleGitHub}
              disabled={loading === 'github' || !repoName.trim() || !repoDescription.trim()}
              className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="profile-github-btn"
            >
              {loading === 'github' ? 'Generating...' : 'Generate README'}
            </button>

            {readmeResult && (
              <div className="mt-3" data-testid="profile-github-result">
                <h4 className="text-xs text-fg-muted font-medium mb-1">Generated README</h4>
                <pre className="text-sm text-fg whitespace-pre-wrap font-sans bg-elevated rounded-lg p-3 max-h-64 overflow-y-auto">{readmeResult.markdown}</pre>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Testimonial Request */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('testimonial')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-testimonial"
        >
          <span>Testimonial Request</span>
          <span className="text-fg-muted text-xs">{expanded.has('testimonial') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('testimonial') && (
          <div className="px-4 pb-4 space-y-3">
            <div>
              <label className="text-xs text-fg-muted block mb-1">Lead ID</label>
              <input
                type="number"
                value={testimonialLeadId}
                onChange={e => setTestimonialLeadId(e.target.value)}
                placeholder="Enter lead ID..."
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50"
                data-testid="profile-testimonial-lead-id"
              />
            </div>
            <button
              onClick={handleTestimonial}
              disabled={loading === 'testimonial' || !testimonialLeadId || isNaN(parseInt(testimonialLeadId, 10))}
              className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="profile-testimonial-btn"
            >
              {loading === 'testimonial' ? 'Generating...' : 'Generate Request'}
            </button>

            {testimonialResult && (
              <div className="mt-3" data-testid="profile-testimonial-result">
                <h4 className="text-xs text-fg-muted font-medium mb-1">Request Message</h4>
                <pre className="text-sm text-fg whitespace-pre-wrap font-sans bg-elevated rounded-lg p-3">{testimonialResult.message}</pre>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Pin Recommendation */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('pin')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="profile-section-pin"
        >
          <span>Pin Recommendation</span>
          <span className="text-fg-muted text-xs">{expanded.has('pin') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('pin') && (
          <div className="px-4 pb-4 space-y-3">
            <div>
              <label className="text-xs text-fg-muted block mb-1">Platform</label>
              <select
                value={pinPlatform}
                onChange={e => setPinPlatform(e.target.value as Platform)}
                className="soul-select"
                data-testid="profile-pin-platform"
              >
                <option value="linkedin">LinkedIn</option>
                <option value="github">GitHub</option>
              </select>
            </div>
            <button
              onClick={handlePin}
              disabled={loading === 'pin'}
              className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="profile-pin-btn"
            >
              {loading === 'pin' ? 'Generating...' : 'Get Recommendations'}
            </button>

            {pinResult && (
              <div className="mt-3 space-y-2" data-testid="profile-pin-result">
                <h4 className="text-xs text-fg-muted font-medium mb-1">Recommended Pins</h4>
                {pinResult.recommended_pins.map((pin, i) => (
                  <div key={i} className="bg-elevated rounded-lg p-3 text-sm text-fg" data-testid={`profile-pin-item-${i}`}>
                    {pin}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
