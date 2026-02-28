import { useState } from 'react';

interface NewTaskFormProps {
  onClose: () => void;
  onCreate: (title: string, description: string, priority: number, product: string) => Promise<void>;
}

const PRIORITY_OPTIONS = [
  { value: 0, label: 'Low' },
  { value: 1, label: 'Normal' },
  { value: 2, label: 'High' },
  { value: 3, label: 'Critical' },
];

export default function NewTaskForm({ onClose, onCreate }: NewTaskFormProps) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState(1);
  const [product, setProduct] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
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
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Task title"
              required
              className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg placeholder:text-fg-muted font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20"
            />
          </div>

          <div>
            <label htmlFor="task-description" className="block font-display text-xs font-medium text-fg-secondary uppercase tracking-wide mb-1">
              Description
            </label>
            <textarea
              id="task-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe the task..."
              rows={4}
              className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg placeholder:text-fg-muted font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20 resize-none"
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
              <input
                id="task-product"
                type="text"
                value={product}
                onChange={(e) => setProduct(e.target.value)}
                placeholder="e.g. compliance"
                required
                className="w-full px-3 py-2 bg-elevated border border-border-default rounded-lg text-fg placeholder:text-fg-muted font-body text-sm focus:border-soul/50 focus:outline-none focus:ring-1 focus:ring-soul/20"
              />
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium rounded-md bg-elevated hover:bg-overlay text-fg-secondary hover:text-fg transition-colors cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!title.trim() || !product.trim() || submitting}
              className="px-4 py-2 text-sm rounded-md bg-soul hover:bg-soul/80 text-deep font-display font-semibold disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer"
            >
              {submitting ? 'Creating...' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
