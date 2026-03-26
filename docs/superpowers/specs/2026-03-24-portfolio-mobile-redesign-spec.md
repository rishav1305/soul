# Portfolio Mobile-Responsive Redesign Spec

**Date:** 2026-03-24
**Author:** Loki (Brand & Growth Lead)
**Collaborators:** Fury (Strategy), Pepper (Product)
**Implementer:** Shuri (Technical PM)
**Status:** DRAFT v1.1 — Fury strategic input integrated, awaiting Pepper product requirements overlay

---

## Problem Statement

The portfolio site (rishavchatterjee.com) looks good on desktop but is **broken on mobile**. CEO has confirmed this directly. Given that 60-70% of recruiter/hiring manager traffic originates from LinkedIn profile clicks on mobile devices, this is a conversion-killing issue.

### Who Visits on Mobile?

| Visitor Type | Context | What They Want | Time Budget |
|---|---|---|---|
| Recruiter screening candidates | Clicked LinkedIn profile link on phone | Quick credibility check: skills, experience, proof | 30-60 seconds |
| Hiring manager evaluating | Forwarded link from recruiter or saw LinkedIn post | Deeper assessment: projects, technical depth, fit | 2-5 minutes |
| Peer/conference attendee | Scanned QR or clicked link at event | What does this person do? Anything impressive? | 15-30 seconds |
| Inbound from content | Read LinkedIn post, clicked profile | Validate the claims made in content | 1-3 minutes |

**Primary mobile action:** View credentials and projects, then contact or save for later.

---

## Audit Findings

### Critical Issues (BROKEN)

#### C1: GOAT Preview / ContainerScroll — Dead Space Monster

**File:** `GoatPreviewSection.tsx` + `container-scroll-animation.tsx`

- Container is `h-[60rem]` (960px) on mobile — creates a massive scroll dead zone
- The scroll-linked 3D rotation effect doesn't work well on touch (no fine scroll control)
- `PlatformMock` has a `w-44` sidebar that doesn't collapse, with `text-[10px]`/`text-[11px]` text — **unreadable on phones**
- Top bar content overflows: "5,047 active users" + model names + status dot all in one row
- Source tokens line at bottom uses horizontal flex with no wrapping

**Impact:** ~960px of broken/unreadable content. Worst single offender.

#### C2: Image Accordion in FlagshipProjects — Touch-Broken Interaction

**File:** `interactive-image-accordion.tsx` + `FlagshipProjects.tsx`

- Accordion uses `onMouseEnter` for interaction — **does not exist on touch devices**
- Mobile users cannot switch between project images/descriptions
- `flex-[4]`/`flex-[0.5]` in `flex-row` creates unreadable slivers on 375px screens
- Height hardcoded at `h-[450px]` regardless of viewport
- The `data-cursor="view"` custom cursor hint is useless on mobile

**Impact:** Flagship projects section (core credibility) is non-functional on mobile.

#### C3: 60% Cost Reduction Claim — Content Inconsistency

**Files:** `HeroSection.tsx` line 176, `GoatPreviewSection.tsx` line 52

- Hero says: "60% avg cost reduction"
- GOAT mock sidebar: `{ label: 'Cost saved', value: '60%' }`
- Content strategy has corrected this to "30-40% estimated" (CEO Decision #4, Mar 22)
- **If LinkedIn posts say 30-40% and portfolio says 60%, reviewers catch the inconsistency**

**Impact:** Credibility damage. Direct contradiction between content channels.

### Layout Issues

#### L1: Hero Background Shapes Overflow

**File:** `HeroSection.tsx` lines 68-108

- `ElegantShape` widths of 600px, 500px, 300px on a 375px viewport
- Parent has `overflow-hidden` but shapes positioned with `left-[-10%]` can still cause layout calculation issues
- On very small screens, shapes may cause horizontal scroll or render artifacts

**Recommendation:** Reduce shape sizes on mobile or hide entirely. Shapes are decorative, not functional.

#### L2: "I build [rotating text]" Clipping

**File:** `HeroSection.tsx` line 157-168

- `flex-wrap` is set, but `RotatingText` component animates with blur mode
- On narrow screens, if the rotating word is long ("LLM evaluation tools"), the animated transition can clip or overflow the container
- The `gap-2` spacing may not be enough when text wraps to two lines

**Recommendation:** On mobile, use a simpler static subtitle or reduce rotating words to shorter variants.

#### L3: NowSection Terminal Overflow

**File:** `NowSection.tsx`

- Terminal body has monospace text at `text-sm` with no `overflow-x` handling
- Lines like `"soul-bench/ — LLM evaluation framework"` in `flex items-center gap-2` may overflow on narrow viewports
- The separator line `──────────────────────` is fixed-width and will overflow on phones

**Recommendation:** Add `overflow-x-auto` to terminal body, truncate separator to viewport width.

#### L4: SkillsOverview Header Cramping

**File:** `SkillsOverview.tsx` line 75

- `flex items-end justify-between` puts heading and CTA side by side
- On small screens, this can cause the heading to be squeezed or text to wrap oddly
- The "Full skills breakdown" link is `hidden md:inline-flex` (correct), but mobile CTA placement at bottom could be more prominent

### Strategic Issues (Fury's Flags)

#### S1: SoulGraph Missing from Portfolio

**File:** `FlagshipProjects.tsx` lines 8-51

The `PROJECTS` array contains only:
1. GOAT: Agentic AI Platform
2. Soul Bench
3. Soul: 9-Agent Autonomous Team

**SoulGraph is NOT listed** despite being Phase 1 live on GitHub (github.com/rishav1305/soulgraph, shipped Mar 23). This is a significant omission — SoulGraph is the public proof anchor for the current content strategy.

**Recommendation:** Add SoulGraph as a 4th project. On mobile, show only top 2-3 projects with a "View all" link.

#### S2: Consultant vs. Employee CTA Tension

**Files:** `HeroSection.tsx` line 188, `EngagementSection.tsx`

- Hero primary CTA: "Book AI Consultation"
- Full `EngagementSection` with 3 consulting models: Consulting Project, Fractional AI Lead, AI Audit & Strategy
- Salary targeting plan targets FT employment at Rs 50-80 LPA
- **Hiring managers seeing consulting CTAs will assume this person doesn't want a FT role**

This is the deepest strategic issue. The desktop site can afford to show both — mobile first impression cannot.

**Recommendation:** See [Mobile CTA Strategy](#mobile-cta-strategy) below.

#### S3: NowSection Content Stale

**File:** `NowSection.tsx`

- Lists: Soul Bench (focus), Soul Mesh v0.3 (shipping)
- Does NOT mention SoulGraph (Phase 1 live, the current public proof anchor)
- "Soul Mesh" is not part of the current content or positioning strategy

**Recommendation:** Update /now to mention SoulGraph. Remove Soul Mesh reference (not in positioning).

### Performance Issues

#### P1: Unnecessary Mobile Overhead

**File:** `layout.tsx` providers stack

```
SmoothScrollProvider (Lenis?) → CursorProvider → ScrollProgress → PageTransition
```

- `CursorProvider` is **completely useless on touch devices** — custom cursor doesn't exist on mobile
- `SmoothScrollProvider` may fight with native mobile scrolling momentum (iOS rubber-band)
- `ScrollProgress` is a thin bar — low value on mobile where scroll depth is less meaningful
- `PageTransition` adds framer-motion AnimatePresence on every route change

**Recommendation:** Conditionally disable CursorProvider and SmoothScrollProvider on mobile (`pointer: coarse` media query or `window.matchMedia`).

#### P2: framer-motion Everywhere

Every section uses framer-motion with `whileInView` animations. On mobile:
- Lower GPU power means jittery animations, especially with `backdrop-blur`
- Multiple simultaneous `whileInView` triggers as user scrolls fast
- `ElegantShape` components have infinite floating animations (`duration: 12, repeat: POSITIVE_INFINITY`)

**Recommendation:** Respect `prefers-reduced-motion`. On mobile, consider reducing to entrance-only animations (no infinite loops).

#### P3: Custom Cursor Hides Native Cursor

**File:** `globals.css` line 64

```css
@media (pointer: fine) { body { cursor: none; } }
```

This correctly targets only precise pointer devices. However, the CursorProvider still mounts DOM elements and runs JS on mobile. Waste of resources.

---

## Mobile Redesign Specification

### Design Principles

1. **Content-first, not effect-first** — Mobile visitors want information, not animations
2. **Credibility in 30 seconds** — Name, title, proof metrics, and projects must be above the fold or within one scroll
3. **Neutral CTAs on mobile** — No consulting-specific language in the mobile first impression. Consulting language appears ONLY after proof of capability (Fury directive)
4. **Touch-native interactions** — No hover-dependent UI, minimum 44x44px touch targets
5. **Performance budget** — Target < 3s LCP on 4G, < 100ms input latency
6. **Verifiable metrics only** — Every number must survive a 10-minute deep dive. "8+ years production AI" and "5,000+ users" are safe. No inflated claims (Fury directive)
7. **Specificity is the differentiator** — "Multi-Agent Orchestration Framework" not "AI Framework". Specificity signals senior, generics signal junior (Fury directive)

### Breakpoint System

| Breakpoint | Name | Target Devices | Columns |
|---|---|---|---|
| 0-639px | `sm` (default) | iPhone SE to iPhone 15 Pro | 1 |
| 640-767px | `sm:` | Large phones, small tablets | 1-2 |
| 768-1023px | `md:` | iPad Mini, iPad | 2 |
| 1024-1279px | `lg:` | iPad Pro, small laptops | 2-3 |
| 1280px+ | `xl:` | Desktops | 3 |

**Critical test devices:** iPhone SE (375px), iPhone 15 (393px), Samsung Galaxy S24 (360px)

### Section-by-Section Mobile Spec

#### 1. Navbar (OK with tweaks)

**Current state:** Hamburger menu works. Full-screen overlay on mobile is good.

**Changes:**
- Add `touch-action: manipulation` to all nav links (kill 300ms tap delay)
- Ensure close button touch target is at least 44x44px (currently `w-8 h-8` = 32px)
- Add the name "Rishav Chatterjee" as visible text in mobile menu header (currently hidden until scroll)

#### 2. Hero Section

**Current state:** Mostly works but has issues with shape overflow and CTA messaging.

**Changes:**

| Element | Current | Mobile Spec |
|---|---|---|
| Background shapes | 5 shapes, 600px max width, infinite float | Hide on mobile (`hidden md:block`) or reduce to 2 small shapes |
| Overline | "Senior AI Architect . Agentic Systems" | Keep as-is, good |
| Name (h1) | `text-5xl sm:text-6xl md:text-8xl` | Change to `text-4xl sm:text-5xl md:text-8xl` for iPhone SE |
| TextScramble | Runs on load | Disable on mobile (use static GoldDottedName) — saves JS + avoids flicker |
| Rotating text | "I build [rotating words]" with blur animation | Keep but ensure container has `min-h-[2em]` to prevent layout shift |
| Value prop | "60% avg cost reduction" | Change to "30-40% estimated cost reduction" per CEO Decision #4 |
| Primary CTA | "Book AI Consultation" | **Change to "View My Work"** (neutral, works for both FT and consulting) |
| Secondary CTA | "See My Work" | **Change to "Get In Touch"** (neutral contact, no consulting framing) |
| Availability text | Long shimmer text | Shorten on mobile: "Available Q2 2026 . India + Remote" |

<a name="mobile-cta-strategy"></a>
**Mobile CTA Strategy (addressing S2):**

```
Desktop Hero:                    Mobile Hero:
[Book AI Consultation]           [View My Work]     ← primary, scrolls to projects
[See My Work]                    [Get In Touch]     ← secondary, goes to /contact
```

The consulting-specific CTAs remain on desktop and on the /contact page's engagement section. Mobile first impression stays neutral for hiring managers.

#### 3. GOAT Preview Section

**Current state:** BROKEN on mobile. 960px dead space, unreadable mock.

**Mobile spec — replace ContainerScroll with static showcase:**

```
Mobile (< 768px):
┌─────────────────────────┐
│ GOAT Platform           │
│ ────────────────────    │
│ Agentic AI research     │
│ platform for Fortune    │
│ 500 analytics company   │
│                         │
│ ┌─────┐ ┌─────┐ ┌─────┐│
│ │5,000+│ │ 88% │ │30-40%│
│ │users │ │resol│ │cost ↓│
│ └─────┘ └─────┘ └─────┘│
│                         │
│ [View Case Study →]     │
└─────────────────────────┘

Desktop (768px+):
ContainerScroll 3D animation (current behavior, fix cost claim)
```

**Implementation:**
- Wrap `ContainerScroll` in a `hidden md:block` container
- Create a new `GoatMobileCard` component for `md:hidden`
- The mobile card shows: title, 1-line description, 3 metric pills, case study link
- **Fix cost metric:** Change `{ label: 'Cost saved', value: '60%' }` to `{ label: 'Cost saved', value: '30-40%' }`
- Height on mobile: auto (content-driven), NOT fixed `h-[60rem]`

#### 4. Proof Bar

**Current state:** Works reasonably well. Grid is `grid-cols-2 md:grid-cols-4`.

**Changes:**
- Stats: Keep `grid-cols-2` on mobile — this is fine
- Marquee: Reduce brand logo heights slightly on mobile for better fit
- Add `touch-action: manipulation` to prevent accidental zoom on double-tap

#### 5. Services Section

**Current state:** Cards stack correctly (`grid-cols-1 md:grid-cols-3`). 3D tilt effect on hover is desktop-only (good).

**Changes:**
- Reduce padding on mobile: `p-7` to `p-5` on `< md`
- The spotlight gradient effect runs on `mousemove` — verify it doesn't fire on touch (it shouldn't, but test)
- Section padding: reduce `py-24 md:py-32` to `py-16 md:py-32` on mobile (less dead space)

#### 6. Engagement Section

**Current state:** Cards stack correctly. But content is strategically problematic on mobile.

**Mobile spec — Restructure for dual-track positioning:**

```
Desktop:                              Mobile:
3 consulting cards + CTA              2 cards (simplified) + neutral CTA

Consulting Project                    How I Work
Fractional AI Lead           →        [Full-time or project-based]
AI Audit & Strategy                   [Short description]

[Discuss Your Project]                [Explore Opportunities →]
```

**Implementation:**
- On mobile (`md:hidden`), show a simplified 2-card layout:
  - Card 1: "Full-Time / Embedded" — "Senior AI architect embedded in your team. Architecture direction, code review, shipping."
  - Card 2: "Project-Based" — "Scoped AI system delivery. Fixed timeline, clear deliverable, full handover."
- On desktop (`hidden md:block`), keep current 3-card consulting layout
- CTA changes: "Discuss Your Project" becomes "Let's Talk" (neutral)
- This addresses the consultant vs. employee tension without losing the consulting pipeline on desktop

#### 7. Skills Overview

**Current state:** Grid is `grid-cols-1 sm:grid-cols-2 lg:grid-cols-3`. Works well.

**Changes:**
- Minor: reduce section padding `py-20 md:py-28` to `py-14 md:py-28`
- The mobile CTA ("Full skills breakdown") at bottom is correct — keep it

#### 8. Flagship Projects (Selected Work)

**Current state:** BROKEN on mobile due to image accordion.

**Mobile spec — replace accordion with swipeable cards:**

```
Mobile (< 1024px):
┌─────────────────────────┐
│ [GOAT image]            │  ← Full-width image
│ ─────────               │
│ Featured — Enterprise   │
│ GOAT: Agentic AI        │
│ [description]           │
│                         │
│ 5,000+   88%   30-40%   │
│ users    resol  cost ↓  │
│                         │
│ [Case study →]          │
└─────────────────────────┘
  ● ○ ○                     ← Dot indicators, swipe or tap

Desktop (1024px+):
Current 2-column layout with image accordion (keep as-is)
```

**Implementation:**
- Below `lg`, replace `ImageAccordion` with a touch-friendly carousel/swiper
- Each card: full-width project image (16:9 ratio) + info below
- Dot indicators for navigation (tap to switch)
- Optional: swipe gesture to navigate between projects
- Add SoulGraph as 4th project (see S1):

```typescript
{
  title: 'SoulGraph: Multi-Agent Orchestration Framework',
  label: 'Open Source — Multi-Agent Orchestration',
  description:
    'Production-grade multi-agent orchestration framework built on LangGraph. Supervisor pattern with RAG-augmented agents and built-in evaluation pipeline. Phase 1 live on GitHub.',
  techStack: ['Python', 'LangGraph', 'RAG', 'ChromaDB', 'Evaluation'],
  metrics: [
    { value: '21/21', label: 'Tests Passing' },
    { value: 'Phase 1', label: 'Live' },
    { value: 'OSS', label: 'Open Source' },
  ],
  caseStudyLink: null,
  githubLink: 'https://github.com/rishav1305/soulgraph',
}
```

- **Fix cost metric for GOAT:** `{ value: '60%', label: 'Cost Reduction' }` changes to `{ value: '30-40%', label: 'Est. Cost Reduction' }`

#### 9. Testimonials

**Current state:** Works correctly. Single column on mobile, 2-3 on desktop. Mask gradient fade is good.

**Changes:**
- Reduce `max-h-[740px]` to `max-h-[500px]` on mobile (less scroll to see next section)
- Add a "Read more" link below the masked container on mobile

#### 10. NowSection

**Current state:** Terminal aesthetic works but has overflow risks.

**Changes:**
- ASCII art: already `hidden md:block` (correct)
- Add `overflow-x-auto` to the terminal body `div`
- Truncate the separator line: use `w-full` with CSS border instead of fixed characters
- **Update content** (S3 + Fury directive):
  - "Focus:" → "SoulGraph — multi-agent orchestration framework (LangGraph)"
  - "Shipping:" → "CARS Benchmark — LLM evaluation across 52 models, 7 providers"
  - Remove ALL "Soul Mesh" references — was a working title that never shipped (Fury confirmed)
  - "building:" line: `soul-bench/` stays, add `soulgraph/` line
  - "shipping:" line: replace `soul-mesh/v0.3` with `soulgraph/phase-1` (live on GitHub)
- On mobile, reduce terminal width padding from `p-5` to `p-4`
- Wrap long lines in `flex-wrap` layout

#### 11. Latest Writing

**Current state:** Cards stack correctly (`grid-cols-1 md:grid-cols-3`).

**Changes:**
- Show only 2 cards on mobile (currently shows 3 placeholder cards — takes too much space)
- Section padding: reduce from `py-20 md:py-28` to `py-14 md:py-28`

#### 12. CTAFooter

**Current state:** Works well. `flex-col sm:flex-row` for buttons is correct.

**Changes:**
- **CTA text on mobile:**
  - Primary: "Get In Touch" (not "Book AI Consultation")
  - Secondary: Keep "General Inquiry"
- Reduce heading size on mobile: `text-3xl` to `text-2xl` (5 words wrapping on 375px causes awkward line breaks)
- Social links: increase touch target to 44x44px (currently `w-5 h-5` = 20px, need `p-2.5` padding)

### Mobile Content Priority Order (Fury-validated)

**Fury's directive:** (a) What I build, (b) Proof it works, (c) How to engage. Consulting language ONLY after proof.

Above the fold (no scroll):
1. Name + Title ("Senior AI Architect")
2. "I build [rotating text]" — what I build
3. Key metric (5,000+ users, 8+ years)
4. Primary CTA: "View My Work" (neutral)

First scroll — WHAT I BUILD:
5. GOAT showcase (mobile card, not ContainerScroll)
6. Projects with SoulGraph (mobile carousel) — immediately follows GOAT

Second scroll — PROOF IT WORKS:
7. Proof bar (stats + brand logos)
8. Skills overview
9. Testimonials

Third scroll — HOW TO ENGAGE:
10. Services (what I do)
11. Engagement models (neutral framing)
12. /now section
13. Blog
14. CTA footer (neutral CTAs)

**Key change from current:** Projects section moves UP, engagement section moves DOWN. Mobile visitor sees what Rishav builds before being asked how to hire him.

### Provider Optimization

```tsx
// layout.tsx — conditional providers
const isTouchDevice = typeof window !== 'undefined' && window.matchMedia('(pointer: coarse)').matches;

// Option A: Client component wrapper
<ConditionalProviders isMobile={isTouchDevice}>
  {children}
</ConditionalProviders>

// Where ConditionalProviders skips CursorProvider and SmoothScrollProvider on touch devices
```

**Or simpler:** Have `CursorProvider` and `SmoothScrollProvider` internally check `pointer: coarse` and return children unmodified when true. No provider nesting change needed.

### Animation Strategy

| Animation | Desktop | Mobile |
|---|---|---|
| Background shapes (floating) | Keep (infinite) | Hide or reduce to 1 static shape |
| TextScramble (name) | Keep | Disable (use static text) |
| whileInView entrance | Keep | Keep but reduce duration (200ms) |
| Spotlight gradient (services) | Keep (mousemove) | Skip (no mousemove on touch) |
| ContainerScroll 3D | Keep | Replace with static card |
| Image accordion | Keep | Replace with swipe carousel |
| Brand marquee | Keep | Keep (CSS-only, lightweight) |
| Page transitions | Keep | Reduce to simple fade (100ms) |
| ScrollProgress bar | Keep | Hide on mobile |

### Accessibility Checklist

- [ ] All touch targets >= 44x44px
- [ ] Minimum 8px gap between adjacent touch targets
- [ ] `touch-action: manipulation` on all interactive elements
- [ ] No hover-dependent interactions on mobile
- [ ] `prefers-reduced-motion` respected (already partial via `useReducedMotion`)
- [ ] Focus visible styles on all interactive elements
- [ ] Alt text on all images (accordion images currently use title only)
- [ ] Heading hierarchy: h1 (name) > h2 (section titles) > h3 (card titles)
- [ ] Minimum 16px body text (currently using `text-sm` = 14px in some places)

---

## Content Corrections (Ship Immediately)

These changes are factual corrections, not redesign work. Should ship ASAP regardless of mobile redesign timeline:

| Location | Current | Corrected |
|---|---|---|
| `HeroSection.tsx` line 176 | "60% avg cost reduction" | "30-40% estimated cost reduction" |
| `GoatPreviewSection.tsx` line 52 | `{ label: 'Cost saved', value: '60%' }` | `{ label: 'Cost saved', value: '30-40%' }` |
| `FlagshipProjects.tsx` line 18 | `{ value: '60%', label: 'Cost Reduction' }` | `{ value: '30-40%', label: 'Est. Cost Reduction' }` |
| `NowSection.tsx` line 102 | "Soul Bench — LLM eval framework" | Keep (correct) |
| `NowSection.tsx` line 112 | "Soul Mesh v0.3 — distributed inference" | "SoulGraph Phase 1 — multi-agent orchestration (LangGraph)" |
| `NowSection.tsx` line 183 | `soul-bench/` | Keep (correct) |
| `NowSection.tsx` line 196 | `soul-mesh/v0.3 — distributed inference` | `soulgraph/phase-1 — multi-agent orchestration` |
| `FlagshipProjects.tsx` PROJECTS array | 3 projects, no SoulGraph | Add SoulGraph as 4th project |

---

## Implementation Phases

### Phase 0: Content Corrections (< 1 hour)
- Fix all 60% claims to 30-40%
- Add SoulGraph to PROJECTS array
- Update NowSection content
- **No mobile layout changes needed — these are text/data fixes**
- Ship immediately via Vercel auto-deploy

### Phase 1: Critical Mobile Fixes (4-6 hours)
- Replace ContainerScroll with mobile card on < md
- Replace ImageAccordion with mobile carousel on < lg
- Fix hero CTAs for mobile (neutral language)
- Fix touch targets (nav close button, social links, CTA buttons)
- Add overflow handling to NowSection terminal
- Disable TextScramble on mobile
- Conditional skip for CursorProvider on touch devices

### Phase 2: Layout Polish (2-3 hours)
- Reduce section padding on mobile (py-24 → py-16 per section)
- Mobile engagement section (dual-track positioning)
- Testimonials height cap on mobile
- Latest Writing show 2 cards on mobile
- Background shape reduction on mobile
- Animation duration reduction on mobile

### Phase 3: Performance (1-2 hours)
- Conditional SmoothScrollProvider disable on touch
- ScrollProgress hide on mobile
- Reduce framer-motion whileInView triggers
- Test LCP < 3s on 4G throttle

---

## Testing Requirements

| Test | Tool | Pass Criteria |
|---|---|---|
| Visual — iPhone SE (375px) | Chrome DevTools / BrowserStack | No horizontal scroll, all text readable, CTAs tappable |
| Visual — iPhone 15 (393px) | Chrome DevTools / BrowserStack | All sections render correctly |
| Visual — Samsung Galaxy S24 (360px) | Chrome DevTools / BrowserStack | No layout breaks |
| Visual — iPad Mini (768px) | Chrome DevTools | Tablet layout transitions correctly |
| Performance — LCP | Lighthouse mobile | < 3.0s on simulated 4G |
| Performance — CLS | Lighthouse mobile | < 0.1 |
| Touch targets | Manual audit | All interactive elements >= 44x44px |
| Accessibility | axe-core / Lighthouse | Score >= 90 |
| Content accuracy | Manual review | No "60%" claims anywhere on site |
| Interaction | Manual on real device | Image carousel swipeable, all CTAs functional |

---

## Open Questions (Awaiting Input)

### For Pepper (Product):
1. Should we add analytics tracking for mobile vs desktop conversion (e.g., contact form submissions by device)?
2. Is there a preferred mobile conversion funnel order? (current spec: name → proof → services → projects → contact)
3. Should the blog section be deprioritized on mobile? (currently 3 placeholder posts taking significant scroll space)

### For CEO:
1. Approve the dual-track CTA strategy (neutral on mobile, consulting on desktop)?
2. Approve adding SoulGraph to the portfolio?
3. GSC verification token — still needed for SEO tracking.

---

## Appendix: File Reference

| File | Changes Needed |
|---|---|
| `src/components/sections/HeroSection.tsx` | CTA text, cost claim, TextScramble conditional, shape reduction |
| `src/components/sections/GoatPreviewSection.tsx` | Mobile card component, cost claim fix |
| `src/components/ui/container-scroll-animation.tsx` | No changes (hidden on mobile instead) |
| `src/components/sections/FlagshipProjects.tsx` | Mobile carousel, SoulGraph addition, cost claim fix |
| `src/components/ui/interactive-image-accordion.tsx` | No changes (hidden on mobile instead) |
| `src/components/sections/EngagementSection.tsx` | Mobile dual-track layout |
| `src/components/sections/NowSection.tsx` | Content update, overflow fix |
| `src/components/sections/TestimonialsSection.tsx` | Mobile height cap |
| `src/components/sections/LatestWriting.tsx` | Show 2 cards on mobile |
| `src/components/sections/CTAFooter.tsx` | CTA text, heading size, touch targets |
| `src/components/navbar/Navbar.tsx` | Close button touch target, touch-action |
| `src/app/layout.tsx` | Conditional provider optimization |
| `src/app/globals.css` | Touch-action global rule |

---

*This spec will be updated when Pepper provides product requirements. The implementation priority should be Phase 0 (immediate content corrections) followed by Phase 1 (critical mobile fixes).*
