# Project Sidebar Refinement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refine the project detail sidebar so the current project, selected module, and return action are clearer while preserving SparkOps' restrained frontend philosophy.

**Architecture:** Keep the existing `ProjectNav` route and active-state logic. Make the rendered markup slightly more semantic for accessibility, then style the project-sidebar elements through existing global classes and design tokens.

**Tech Stack:** Next.js App Router, React client component, CSS in `fronted/apps/web/app/globals.css`, existing SparkOps design tokens.

## Global Constraints

- Only optimize the project sidebar shown in the screenshot.
- Do not change platform navigation, admin navigation, topbar, AppShell width, route structure, permission logic, or active-route matching.
- Reuse existing tokens for color, radius, shadow, and motion.
- Do not introduce a UI library, hardcoded brand colors, gradients, bounce animation, or competing primary visual focus.

---

### Task 1: Project Sidebar Markup And Styling

**Files:**
- Modify: `fronted/apps/web/src/components/layout/ProjectNav.tsx`
- Modify: `fronted/apps/web/app/globals.css`
- Verify: `fronted/apps/web`

**Interfaces:**
- Consumes: `projectId: string`, `projectName: string`, current `usePathname()` value.
- Produces: same navigation links and routes as before; active links additionally expose `aria-current="page"`.

- [ ] **Step 1: Update project nav markup**

In `fronted/apps/web/src/components/layout/ProjectNav.tsx`, keep `PROJECT_NAV_ITEMS`, `PROJECT_NAV_SUFFIXES`, and `isNavActive()` unchanged. Update only the returned JSX so active links expose `aria-current`, project nav items have a text wrapper, and decorative icons are hidden from assistive tech:

```tsx
      {PROJECT_NAV_ITEMS.map((item) => {
        const href = `${base}${item.suffix}`;
        const active = isNavActive(pathname, projectId, item.suffix);
        return (
          <Link
            key={item.label}
            href={href}
            className={`nav-link${active ? " nav-link-active" : ""}`}
            aria-current={active ? "page" : undefined}
          >
            <span className="nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="2" />
              </svg>
            </span>
            <span className="nav-link-label">{item.label}</span>
          </Link>
        );
      })}
```

- [ ] **Step 2: Refine project-sidebar CSS**

In `fronted/apps/web/app/globals.css`, update only the navigation selectors already used by the project sidebar. Use tokens and neutral rgba values, keeping the style compatible with other nav links:

```css
.nav-link {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.625rem 0.875rem;
  border-radius: var(--radius-pill);
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-secondary);
  transition:
    background var(--motion-duration) var(--motion-ease),
    color var(--motion-duration) var(--motion-ease),
    box-shadow var(--motion-duration) var(--motion-ease),
    transform var(--motion-duration) var(--motion-ease);
}

.nav-link:hover {
  background: rgba(255, 255, 255, 0.58);
  color: var(--text);
}

.nav-link:focus-visible {
  outline: 2px solid var(--border-strong);
  outline-offset: 2px;
}

.nav-link-active {
  background: rgba(255, 255, 255, 0.72);
  color: var(--text);
  font-weight: 600;
  box-shadow: var(--shadow-sm);
}

.nav-icon {
  width: 1.125rem;
  height: 1.125rem;
  opacity: 0.58;
  flex-shrink: 0;
  transition: opacity var(--motion-duration) var(--motion-ease);
}

.nav-link-active .nav-icon,
.nav-link:hover .nav-icon {
  opacity: 0.82;
}

.nav-link-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

Then refine the project context block:

```css
.nav-back-link {
  display: inline-flex;
  align-items: center;
  margin-bottom: 0.625rem;
  padding: 0.375rem 0.75rem;
  border-radius: var(--radius-pill);
  font-size: 0.8125rem;
  color: var(--muted);
  transition:
    background var(--motion-duration) var(--motion-ease),
    color var(--motion-duration) var(--motion-ease);
}

.nav-back-link:hover {
  background: rgba(255, 255, 255, 0.5);
  color: var(--text);
}

.nav-project-head {
  margin: 0.125rem 0 0.875rem;
  padding: 0.75rem 0.875rem;
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.56);
  box-shadow: var(--shadow-sm);
}

.nav-project-label {
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--muted);
}

.nav-project-name {
  margin-top: 0.2rem;
  font-size: 0.9375rem;
  font-weight: 650;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [ ] **Step 3: Verify CSS parsing and lint feedback**

Run from repo root:

```powershell
pnpm --dir fronted/apps/web lint
```

Expected: command completes without new errors in `ProjectNav.tsx` or `globals.css`. If the repo lacks this script, run the closest existing frontend verification script from `fronted/apps/web/package.json` and record the result.

- [ ] **Step 4: Review visual acceptance criteria**

Open a project page and confirm:

- Current project name is clear but not presented as a heavy gray block.
- Active nav item is recognizable without adding strong color.
- Return link, project card, and module links form three clear layers.
- Long project names truncate inside the sidebar.
- Keyboard focus is visible on nav links.
