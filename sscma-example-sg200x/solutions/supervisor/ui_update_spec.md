# Supervisor Web UI Redesign Spec (UI-only)

## Summary
Redesign the Supervisor web UI to match the look/feel of the primary brand site (The Police Record / TPR) to improve brand continuity across devices (mobile + desktop). This is a **UI-only** effort: **no backend or frontend logic changes** (APIs, routing behavior, data flow) beyond what is strictly required to apply new styling and layout.

This document is the working plan/spec for executing the redesign.

## Goals
- Make the Supervisor UI visually consistent with the main site (colors, typography, spacing, components, voice).
- Improve perceived quality and usability on small screens and desktop screens without changing feature behavior.
- Ensure the UI works fully **offline** (no required external CSS/fonts/images/CDNs).

## Non-goals / Constraints
- Do not change backend (Go) logic or API behavior.
- Do not change frontend business logic (data fetching, store shape, auth flow, route structure, API endpoints).
- `TPR.css` in `solutions/supervisor/TPR.css` is **reference only** and **must not ship** in compiled output.
- Avoid introducing runtime dependencies on third-party CDNs or external fonts.

## Current Implementation (What We’re Styling)
- Frontend is a Vite + React + TypeScript app with TailwindCSS + antd/antd-mobile + Zustand:
  - Source: `sscma-example-sg200x/solutions/supervisor/www/`
  - Entry: `sscma-example-sg200x/solutions/supervisor/www/index.html` → `src/main.tsx` → `src/App.tsx`
  - Routing: hash router (`createHashRouter`) for static hosting compatibility.
  - UI source-of-truth: `sscma-example-sg200x/solutions/supervisor/www/src/` (this existing structure should be respected and used as the basis for UI updates).
- Serving model (in scope):
  - Backend serves static files from `SUPERVISOR_ROOT_DIR` (default `/usr/share/supervisor/www/`).
  - The UI refresh must keep the current supervisor web root/paths intact (no re-homing of files or changing served locations).
- Deployment to end users (out of scope):
  - How artifacts are packaged/installed onto devices is explicitly out of scope for this UI project and should not drive UI decisions in this spec.

## Brand Reference Inputs
### CSS reference file
- `sscma-example-sg200x/solutions/supervisor/TPR.css` is a guide for brand styling patterns.
- It currently references external images via URLs (example patterns):
  - `https://cdn.thepolicerecord.com/...`
  - `https://dev.thepolicerecord.com/...`

### Required offline asset policy
- Any assets referenced by the redesigned UI (images, icons, fonts) must be included in the repository and bundled into the built `dist/` output.
- No runtime fetching of brand assets from the internet should be required.

## Design Direction (High-level)
### Look & Feel
- Adopt TPR-like:
  - Color system (primary, background surfaces, neutral text scale, error/warn states).
  - Typography scale and weight hierarchy.
  - Card/surface styling, shadows, borders, rounding.
  - Button styling and form field treatments.
  - Navigation/header treatments and spacing rhythm.

### Responsive behavior
- Maintain existing “mobile vs PC layout” intent found under `src/layout/`.
- Ensure:
  - Mobile-first layout, touch-friendly sizing.
  - Desktop layout with appropriate use of space and clearer information hierarchy.

## UI Architecture Plan (No logic changes)
### Styling approach
- Keep Tailwind as the primary layout/styling system.
- Use Tailwind theme tokens to encode brand values:
  - Update `www/tailwind.config.js` with brand colors, typography, spacing scales as needed.
  - Prefer CSS variables for theme primitives where helpful (e.g., `--color-bg`, `--color-surface`, `--color-text`), mapped into Tailwind.
- Use minimal global CSS in `www/src/assets/style/index.css` for:
  - Base/background, typography defaults, ant resets, and a small number of global utility overrides.

### Component strategy
- Re-skin existing components and pages; do not rewrite flows.
- Introduce a small set of “brand wrappers” (UI-only components) if needed:
  - `BrandHeader`, `BrandCard`, `BrandButton` (thin wrappers around Tailwind/antd components)
  - Keep props and usage identical where possible to avoid logic changes.

### Ant Design theming
- Align antd token theming with brand palette:
  - Update `ConfigProvider` theme tokens (already present in `src/App.tsx`) to match the brand system.
  - Ensure contrast and accessibility for text on primary backgrounds.

## Offline Asset Plan
### Inventory external references
- Scan for `http(s)://` and `url(` in:
  - `solutions/supervisor/TPR.css`
  - `solutions/supervisor/www/src/**`
- Produce a list of:
  - External images
  - External fonts (if any)
  - Any other CDN references

### Download + store locally
- Add assets to the frontend under one of:
  - `www/public/` (copied as-is by Vite, stable URL paths)
  - `www/src/assets/` (bundled and hashed by Vite when imported)
- Prefer `public/brand/...` for direct CSS `url(...)` usage without import churn.
- Replace external `url(https://...)` with local `url(/brand/...)` (or equivalent relative paths depending on `base`).

### Base path considerations
- Use asset paths that work with the supervisor’s current static file root (`SUPERVISOR_ROOT_DIR`) and do not require changing where files are served from.
- Prefer relative asset references where feasible to avoid assumptions about installation/packaging location.

## Scope of Visual Updates (Pages/Areas)
UI changes apply across all screens without changing behavior:
- Login (`src/views/login`)
- Overview (`src/views/overview`)
- Network (`src/views/network`)
- Files (`src/views/files`)
- Terminal (`src/views/terminal`)
- System / Security / Power (`src/views/system`, `src/views/security`, `src/views/power`)
- Dashboard wrapper (`src/views/dashboard`)
- Shared layout shells (`src/layout/mobile/*`, `src/layout/pc/*`)
- Common components (`src/components/*`)

## Work Plan
### Phase 0 — Research (how it works)
1. Map navigation + layout structure:
   - Identify where header/nav/sidebar are rendered (mobile + PC).
   - Identify global containers (page padding, max widths, background surfaces).
2. Catalog reused components and style entry points:
   - Ant components used commonly (Buttons, Forms, Lists, Tabs, Popup/Modal).
   - Tailwind conventions and existing tokens.
   - Confirm the primary UI organization under `www/src/` (layouts, views, shared components, assets) and plan reskin work around it.
3. Create an “external dependency” list for assets and URLs.

### Phase 1 — Define the Brand System (tokens)
1. Create/update Tailwind theme tokens (colors, radii, shadows, spacing).
2. Define typography rules (sizes/weights/line heights).
3. Define core surfaces:
   - App background
   - Page surface
   - Card surface
   - Elevated surface (modal/popup)

### Phase 2 — Layout + Navigation Reskin
1. Mobile header/sidebar reskin (spacing, icons, backgrounds).
2. Desktop layout reskin (left nav, top bar, content container).
3. Ensure consistent page titles, breadcrumbs (if present), and section spacing.

### Phase 3 — Component Reskin
1. Buttons, inputs, selects, toggles, lists, cards, tabs.
2. Popups/modals to match brand.
3. Loading/empty/error states.

### Phase 4 — Page-by-page Visual Polish
1. Apply consistent hero/intro sections where appropriate.
2. Harmonize spacing and information density per page.
3. Ensure the camera streaming/preview area (if present) looks integrated.

### Phase 5 — Offline Assets + Hardening
1. Download and vendor any brand assets referenced by CSS.
2. Replace external references.
3. Build and verify the resulting `dist/` contains everything required.

## Acceptance Criteria
- UI visually matches the main brand direction (TPR-like) across mobile and desktop.
- No runtime dependency on external assets (fonts/images/CSS) to render correctly.
- No changes to API behavior, auth flow, or feature behavior (UI-only changes).
- Current supervisor static file serving locations/paths remain intact (served from `SUPERVISOR_ROOT_DIR`).

## Deliverables
- Updated UI styling and layout in `sscma-example-sg200x/solutions/supervisor/www/` (future work; not part of this doc).
- A locally vendored asset set inside the frontend project (future work).
- This spec: `sscma-example-sg200x/solutions/supervisor/ui_update_spec.md`
