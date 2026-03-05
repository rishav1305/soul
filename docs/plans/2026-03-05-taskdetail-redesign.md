# TaskDetail Redesign — Tabbed Layout

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the cluttered single-scroll TaskDetail modal with a clean tabbed layout (Task, Plan, Implementation, Comments).

**Architecture:** Single-file refactor of `web/src/components/planner/TaskDetail.tsx`. The component keeps the same props interface — only the internal JSX structure changes. A `activeTab` state drives which tab content renders. Processing banner is always visible. Auto-switch to Implementation tab when `streamContent` is non-empty.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4 (Soul design system tokens)

---

### Task 1: Scaffold tab state and tab bar

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx:58-68` (state declarations)
- Modify: `web/src/components/planner/TaskDetail.tsx:166-308` (header JSX)

**Step 1: Add tab state and type**

Add after line 4 (imports):
```tsx
type DetailTab = 'task' | 'plan' | 'implementation' | 'comments';
```

Add to component state (after line 67):
```tsx
const [activeTab, setActiveTab] = useState<DetailTab>('task');
```

**Step 2: Add auto-switch effect**

Add after the existing `useEffect` blocks (~line 113):
```tsx
// Auto-switch to Implementation tab when agent starts processing.
useEffect(() => {
  if (streamContent && streamContent.length > 0) {
    setActiveTab('implementation');
  }
}, [streamContent]);
```

**Step 3: Replace header JSX (lines 169-308)**

Replace the entire header div with a slim header + tab bar:
```tsx
{/* Header — slim */}
<div className="px-6 pt-4 pb-0 shrink-0">
  <div className="flex items-center justify-between gap-4">
    <div className="flex items-center gap-3 min-w-0">
      <span className="text-fg-muted font-mono text-xs shrink-0">#{task.id}</span>
      {editingTitle ? (
        <input
          ref={titleInputRef}
          value={titleDraft}
          onChange={(e) => setTitleDraft(e.target.value)}
          onBlur={commitTitle}
          onKeyDown={(e) => {
            if (e.key === 'Enter') commitTitle();
            if (e.key === 'Escape') { setEditingTitle(false); setTitleDraft(task.title); }
          }}
          className="flex-1 font-display text-lg font-semibold text-fg bg-transparent border-b border-soul/60 outline-none pb-0.5"
        />
      ) : (
        <h3
          className="font-display text-lg font-semibold text-fg truncate cursor-text hover:text-fg/80 transition-colors"
          onClick={() => setEditingTitle(true)}
          title="Click to edit title"
        >
          {task.title}
        </h3>
      )}
      <span className={`px-2 py-0.5 rounded text-xs font-medium shrink-0 ${STAGE_COLORS[task.stage]}`}>
        {STAGE_LABELS[task.stage]}
      </span>
    </div>
    <div className="flex items-center gap-2 shrink-0">
      <button
        type="button"
        onClick={() => onDelete(task.id)}
        className="flex items-center justify-center w-8 h-8 rounded bg-red-600/80 hover:bg-red-600 text-white transition-colors cursor-pointer text-sm"
        title="Delete task"
      >
        ␡
      </button>
      <button
        type="button"
        onClick={onClose}
        className="flex items-center justify-center w-8 h-8 rounded bg-elevated hover:bg-overlay text-fg-muted hover:text-fg transition-colors cursor-pointer text-lg leading-none"
      >
        &times;
      </button>
    </div>
  </div>

  {/* Processing banner */}
  {isProcessing && (
    <div className="flex items-center gap-2 mt-3 px-3 py-1.5 rounded-lg bg-soul/10 border border-soul/20">
      <span className="inline-block w-2 h-2 rounded-full bg-soul animate-pulse" />
      <span className="text-xs text-soul font-medium">Soul is working...</span>
    </div>
  )}

  {/* Tab bar */}
  <div className="flex gap-0 mt-3 border-b border-border-subtle">
    {([
      { key: 'task' as DetailTab, label: 'Task' },
      { key: 'plan' as DetailTab, label: 'Plan' },
      { key: 'implementation' as DetailTab, label: 'Implementation' },
      { key: 'comments' as DetailTab, label: `Comments${comments.length > 0 ? ` (${comments.length})` : ''}` },
    ]).map((tab) => (
      <button
        key={tab.key}
        type="button"
        onClick={() => setActiveTab(tab.key)}
        className={`px-4 py-2 text-xs font-medium transition-colors cursor-pointer ${
          activeTab === tab.key
            ? 'text-soul border-b-2 border-soul -mb-px'
            : 'text-fg-muted hover:text-fg-secondary'
        }`}
      >
        {tab.label}
      </button>
    ))}
  </div>
</div>
```

**Step 4: Verify tabs render and switch**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: Build succeeds (no type errors)

---

### Task 2: Implement Task tab content

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx` (body section)

**Step 1: Replace the body div (lines 311-485)**

Replace the entire `{/* Body */}` div with tab-switched content:
```tsx
{/* Tab content */}
<div className="flex-1 overflow-y-auto px-6 py-4">

  {/* ── Task tab ── */}
  {activeTab === 'task' && (
    <div className="space-y-5">
      {/* Description */}
      <div>
        <h4 className="text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-1.5">Description</h4>
        {task.description ? (
          <p className="text-fg-secondary text-sm whitespace-pre-wrap">{task.description}</p>
        ) : (
          <p className="text-fg-muted text-sm italic">No description</p>
        )}
      </div>

      {/* Acceptance Criteria */}
      {task.acceptance && (
        <div>
          <h4 className="text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-1.5">Acceptance Criteria</h4>
          <p className="text-fg-secondary text-sm whitespace-pre-wrap">{task.acceptance}</p>
        </div>
      )}

      {/* Properties grid */}
      <div>
        <h4 className="text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-2">Properties</h4>
        <div className="grid grid-cols-2 gap-3">
          {/* Stage */}
          <div className="space-y-1">
            <label className="text-[10px] text-fg-muted">Stage</label>
            <select
              value={task.stage}
              onChange={handleStageChange}
              className="soul-select w-full bg-elevated border border-border-default rounded-lg px-3 py-1.5 text-sm text-fg cursor-pointer"
            >
              {ALL_STAGES.map((s) => (
                <option key={s} value={s} className="bg-surface text-fg">{STAGE_LABELS[s]}</option>
              ))}
            </select>
          </div>

          {/* Priority */}
          <div className="space-y-1">
            <label className="text-[10px] text-fg-muted">Priority</label>
            <select
              value={task.priority}
              onChange={handlePriorityChange}
              className="soul-select w-full bg-elevated border border-border-default rounded-lg px-3 py-1.5 text-sm text-fg cursor-pointer"
            >
              {PRIORITY_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value} className="bg-surface text-fg">{opt.label}</option>
              ))}
            </select>
          </div>

          {/* Product */}
          <div className="space-y-1">
            <label className="text-[10px] text-fg-muted">Product</label>
            <select
              value={task.product ?? ''}
              onChange={handleProductChange}
              className="soul-select w-full bg-elevated border border-border-default rounded-lg px-3 py-1.5 text-sm text-fg cursor-pointer"
            >
              <option value="" className="bg-surface text-fg">None</option>
              {productOptions.map((p) => (
                <option key={p} value={p} className="bg-surface text-fg">{p.charAt(0).toUpperCase() + p.slice(1)}</option>
              ))}
            </select>
          </div>

          {/* Autonomous */}
          <div className="space-y-1">
            <label className="text-[10px] text-fg-muted">Autonomous</label>
            <div className="flex items-center gap-2 h-[34px]">
              <button
                type="button"
                onClick={toggleAutonomous}
                className={`relative w-10 h-5 rounded-full transition-colors cursor-pointer shrink-0 ${autonomous ? 'bg-soul' : 'bg-elevated border border-border-default'}`}
              >
                <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full transition-all duration-200 ${autonomous ? 'translate-x-5 bg-deep' : 'translate-x-0 bg-fg-muted'}`} />
              </button>
              {autonomous && (
                <div className="flex gap-1">
                  {(['quick', 'full'] as const).map((mode) => (
                    <button
                      key={mode}
                      type="button"
                      onClick={async () => {
                        const newMeta = { ...meta, workflow: mode };
                        await onUpdate(task.id, { metadata: JSON.stringify(newMeta) });
                      }}
                      className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors cursor-pointer ${
                        (meta.workflow || 'quick') === mode
                          ? 'bg-soul/20 text-soul'
                          : 'bg-elevated text-fg-muted hover:text-fg-secondary'
                      }`}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Error callout */}
      {task.error && (
        <div className="rounded-lg p-3 bg-red-500/10 border border-red-500/20">
          <h4 className="text-[10px] font-semibold uppercase tracking-widest text-stage-blocked mb-1">Error</h4>
          <p className="text-sm text-stage-blocked font-mono whitespace-pre-wrap">{task.error}</p>
        </div>
      )}

      {/* Blocker callout */}
      {task.blocker && (
        <div className="rounded-lg p-3 bg-amber-500/10 border border-amber-500/20">
          <h4 className="text-[10px] font-semibold uppercase tracking-widest text-amber-400 mb-1">Blocker</h4>
          <p className="text-sm text-amber-300 whitespace-pre-wrap">{task.blocker}</p>
        </div>
      )}
    </div>
  )}
```

**Step 2: Verify no syntax errors**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: Build succeeds

---

### Task 3: Implement Plan tab content

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx` (add after Task tab block)

**Step 1: Add Plan tab JSX**

```tsx
  {/* ── Plan tab ── */}
  {activeTab === 'plan' && (
    <div>
      {task.plan ? (
        <div className="prose prose-invert prose-sm prose-soul max-w-none">
          <Markdown remarkPlugins={[remarkGfm]}>{task.plan}</Markdown>
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-16 text-fg-muted">
          <span className="text-2xl mb-2">◇</span>
          <p className="text-sm">No plan generated yet</p>
        </div>
      )}
    </div>
  )}
```

---

### Task 4: Implement Implementation tab content

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx` (add after Plan tab block)

**Step 1: Add Implementation tab JSX**

```tsx
  {/* ── Implementation tab ── */}
  {activeTab === 'implementation' && (
    <div className="space-y-4">
      {/* Validation review link */}
      {task.stage === 'validation' && task.agent_id?.startsWith('auto-') && (
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-soul/10 border border-soul/20 text-sm">
          <span className="text-fg-secondary">Changes live on dev server:</span>
          <a
            href="http://localhost:3001"
            target="_blank"
            rel="noopener noreferrer"
            className="text-soul hover:underline font-mono text-xs"
          >
            localhost:3001
          </a>
          <span className="text-[10px] text-fg-muted ml-auto">Move to Done to merge to production</span>
        </div>
      )}

      {/* Live stream or final output */}
      {streamContent ? (
        <div className="bg-deep/60 rounded-lg p-4 border border-soul/20">
          <div className="prose prose-invert prose-sm prose-soul max-w-none">
            <Markdown remarkPlugins={[remarkGfm]}>{streamContent}</Markdown>
          </div>
          <div ref={streamEndRef} />
        </div>
      ) : task.output ? (
        <div className="prose prose-invert prose-sm prose-soul max-w-none">
          <Markdown remarkPlugins={[remarkGfm]}>{task.output}</Markdown>
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-16 text-fg-muted">
          <span className="text-2xl mb-2">⚙</span>
          <p className="text-sm">Not started yet</p>
        </div>
      )}

      {/* Activity log */}
      {hasActivities && (
        <div>
          <h4 className="text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-2">Activity</h4>
          <div className="space-y-1">
            {activities.map((a, i) => (
              <ActivityEntry key={i} activity={a} />
            ))}
          </div>
        </div>
      )}
    </div>
  )}
```

---

### Task 5: Implement Comments tab content

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx` (add after Implementation tab block)

**Step 1: Add Comments tab JSX**

```tsx
  {/* ── Comments tab ── */}
  {activeTab === 'comments' && (
    <div className="flex flex-col h-full">
      {/* Scrollable thread */}
      <div className="flex-1 overflow-y-auto space-y-3 mb-3">
        {comments.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-fg-muted">
            <span className="text-2xl mb-2">💬</span>
            <p className="text-sm">No comments yet</p>
          </div>
        ) : (
          comments.map((c) => (
            <div
              key={c.id}
              className={`rounded-lg p-3 text-sm ${
                c.author === 'user'
                  ? 'bg-blue-500/10 border border-blue-500/20'
                  : c.type === 'error'
                    ? 'bg-red-500/10 border border-red-500/20'
                    : c.type === 'verification'
                      ? 'bg-emerald-500/10 border border-emerald-500/20'
                      : 'bg-overlay border border-border-subtle'
              }`}
            >
              <div className="flex items-center gap-2 mb-1">
                <span className={`text-xs font-medium ${
                  c.author === 'user' ? 'text-blue-400' : 'text-fg-muted'
                }`}>
                  {c.author === 'user' ? 'You' : 'Soul'}
                </span>
                <span className="text-xs text-fg-muted">
                  {new Date(c.created_at).toLocaleTimeString()}
                </span>
                {c.type !== 'feedback' && (
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-elevated text-fg-muted">
                    {c.type}
                  </span>
                )}
              </div>
              <p className="text-fg-secondary whitespace-pre-wrap">{c.body}</p>
              {c.attachments && c.attachments.length > 0 && (
                <div className="mt-2 flex flex-col gap-2">
                  {c.attachments.map((filename, idx) => (
                    <a
                      key={idx}
                      href={`/api/screenshots/${filename}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="block"
                    >
                      <img
                        src={`/api/screenshots/${filename}`}
                        alt={`Screenshot: ${filename}`}
                        className="rounded-lg border border-border-default max-w-full max-h-64 object-contain hover:border-soul/50 transition-colors cursor-pointer"
                        loading="lazy"
                      />
                      <span className="text-[10px] text-fg-muted mt-0.5 block">{filename}</span>
                    </a>
                  ))}
                </div>
              )}
            </div>
          ))
        )}
        <div ref={commentsEndRef} />
      </div>

      {/* Comment input — pinned at bottom */}
      <div className="flex gap-2 pt-3 border-t border-border-subtle">
        <input
          type="text"
          value={commentText}
          onChange={(e) => setCommentText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              handleSubmitComment();
            }
          }}
          placeholder="Post feedback..."
          className="flex-1 bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder-fg-muted focus:outline-none focus:border-soul/50"
          disabled={submitting}
        />
        <button
          onClick={handleSubmitComment}
          disabled={submitting || !commentText.trim()}
          className="px-3 py-2 rounded-lg bg-soul hover:bg-soul/80 text-deep text-sm font-medium disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
        >
          {submitting ? '...' : 'Send'}
        </button>
      </div>
    </div>
  )}

</div> {/* end tab content wrapper */}
```

---

### Task 6: Remove old Section component, clean up

**Files:**
- Modify: `web/src/components/planner/TaskDetail.tsx` (bottom of file)

**Step 1: Remove the Section component**

Delete the `Section` function (lines 553-559 in original) since tabs replace the section pattern.

**Step 2: Build and verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: Build succeeds with no errors

---

### Task 7: Build frontend and visual verification

**Step 1: Build**
```bash
cd /home/rishav/soul/web && npx vite build
```

**Step 2: Verify in browser**
- Open a task in Soul UI
- Confirm 4 tabs appear below the slim header
- Confirm Task tab shows description, properties grid
- Confirm Plan tab shows markdown or empty state
- Confirm Implementation tab shows output or empty state
- Confirm Comments tab shows thread + input
- Confirm processing banner appears when agent is active
- Confirm auto-switch to Implementation when streaming
