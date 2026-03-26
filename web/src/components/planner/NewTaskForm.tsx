import { useState, useCallback, useEffect } from 'react';

interface NewTaskFormProps {
  onClose: () => void;
  onCreate: (title: string, description: string, priority: number, product: string) => Promise<void>;
  template?: { priority: number; description: string };
  activeProduct?: string;
  products?: string[];
}

const PRIORITY_OPTIONS = [
  { value: 0, label: 'Low' },
  { value: 1, label: 'Normal' },
  { value: 2, label: 'High' },
  { value: 3, label: 'Critical' },
];

export default function NewTaskForm({ onClose, onCreate, template, activeProduct, products }: NewTaskFormProps) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState(template?.description ?? '');
  const [priority, setPriority] = useState(template?.priority ?? 1);
  const [product, setProduct] = useState(activeProduct ?? '');
  const [submitting, setSubmitting] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [errors, setErrors] = useState<string[]>([]);

  const validate = useCallback(() => {
    const w: string[] = [];
    const e: string[] = [];

    if (description.trim() === '') {
      w.push('Tasks without descriptions are harder to execute. Add details?');
    } else if (description.trim().length < 30) {
      w.push('Consider adding acceptance criteria (use - [ ] checklists)');
    }

    if (priority === 3 && description.trim() === '') {
      e.push('Critical tasks require a description');
    }

    if (title.trim().length >= 1 && title.trim().length <= 9) {
      w.push("Try a more specific title (e.g., 'Add logout button to sidebar')");
    }

    setWarnings(w);
    setErrors(e);
    return e.length === 0;
  }, [title, description, priority]);

  useEffect(() => { validate(); }, [validate]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    if (!title.trim() || !product.trim()) return;

    setSubmitting(true);
    try {
      await onCreate(title.trim(), description.trim(), priority, product.trim());
    } catch (err) {
      console.error('Failed to create task:', err);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-surface border border-border-default rounded-2xl shadow-2xl animate-fade-in-scale w-full max-w-lg mx-4">
        <div className="px-6 py-4 border-b border-border-subtle">
          <h3 className="font-display text-lg font-semibold text-fg">New Task</h3>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-4 space-y-4">
          <div>
            <label htmlFor="task-title" className="block font-display text-xs font-medium text-fg-secondary uppercase tracking-wide mb-1">
              Title <span className="text-stage-blocked">*</span>
            </label>
            <input
              id="task-title"
              data-testid="new-task-title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Task title"
              required
              autoFocus
              className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg placeholder:text-fg-muted font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20"
            />
          </div>

          <div>
            <label htmlFor="task-description" className="block font-display text-xs font-medium text-fg-secondary uppercase tracking-wide mb-1">
              Description
            </label>
            <textarea
              id="task-description"
              data-testid="new-task-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe the task..."
              rows={6}
              className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg placeholder:text-fg-muted font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20 resize-none font-mono"
            />
          </div>

          <div className="flex gap-4">
            <div className="flex-1">
              <label htmlFor="task-priority" className="block font-display text-xs font-medium text-fg-secondary uppercase tracking-wide mb-1">
                Priority
              </label>
              <select
                id="task-priority"
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
                className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20"
              >
                {PRIORITY_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>

            <div className="flex-1">
              <label htmlFor="task-product" className="block font-display text-xs font-medium text-fg-secondary uppercase tracking-wide mb-1">
                Product <span className="text-stage-blocked">*</span>
              </label>
              {products && products.length > 0 ? (
                <select
                  id="task-product"
                  value={product}
                  onChange={(e) => setProduct(e.target.value)}
                  className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20"
                >
                  <option value="">Select product</option>
                  <option value="soul">soul</option>
                  {products.map((p) => (
                    <option key={p} value={p}>{p}</option>
                  ))}
                </select>
              ) : (
                <input
                  id="task-product"
                  data-testid="new-task-product-input"
                  type="text"
                  value={product}
                  onChange={(e) => setProduct(e.target.value)}
                  placeholder="e.g. compliance"
                  required
                  className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg placeholder:text-fg-muted font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20"
                />
              )}
            </div>
          </div>

          {(warnings.length > 0 || errors.length > 0) && (
            <div className="space-y-1">
              {errors.map((e, i) => (
                <p key={`e-${i}`} className="text-xs text-red-400 font-body">{e}</p>
              ))}
              {warnings.map((w, i) => (
                <p key={`w-${i}`} className="text-xs text-amber-400 font-body">{w}</p>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between pt-2">
            <span className="text-[10px] text-fg-muted">
              Tip: Press <kbd className="px-1 py-0.5 bg-overlay rounded text-[10px] font-mono">C</kbd> to open this form
            </span>
            <div className="flex gap-3">
              <button
                type="button"
                data-testid="new-task-cancel"
                onClick={onClose}
                className="px-4 py-2 text-sm font-medium rounded-md bg-elevated hover:bg-overlay text-fg-secondary hover:text-fg transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="submit"
                data-testid="new-task-submit"
                disabled={!title.trim() || !product.trim() || submitting || errors.length > 0}
                className="px-4 py-2 text-sm rounded-md bg-soul hover:bg-soul/80 text-deep font-display font-semibold disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer"
              >
                {submitting ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
