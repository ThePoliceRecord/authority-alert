# UI Theme

Owner: Authority Alert Team
Last updated: 2025-10-12

## Purpose
- Define product‑wide visual tokens (color, type, spacing) and how to apply them in the Web UI and Node‑RED dashboard.
- Ensure accessible contrast, consistent branding, and easy light/dark theming.

## Color Palette (Tokens)
- Primary: `--aa-color-primary` (brand accent)
- Secondary: `--aa-color-secondary`
- Background: `--aa-bg` (light), `--aa-bg-inverse` (dark)
- Surface: `--aa-surface`
- Text: `--aa-text`, `--aa-text-muted`, `--aa-text-inverse`
- Status: `--aa-success`, `--aa-warning`, `--aa-danger`, `--aa-info`

Example CSS variables:
```css
:root {
  --aa-color-primary: #0344ff;   /* vivid brand blue */
  --aa-color-secondary: #0065a3; /* deep cyan-blue */
  --aa-accent: #f1d302;          /* warm accent yellow */
  --aa-bg: #ffffff;
  --aa-bg-inverse: #0b0b0b;      /* near-black */
  --aa-surface: #f4f6f8;
  --aa-surface-inverse: #2a2a27; /* jet gray */
  --aa-text: #1a1d21;
  --aa-text-muted: #5c6672;
  --aa-text-inverse: #e0e0e0;    /* platinum */
  --aa-success: #9be564;         /* CTA green */
  --aa-warning: #f3b61f;         /* highlight */
  --aa-danger: #730001;          /* deep red */
  --aa-info: #1fa9ff;            /* cobalt blue */
}
```

## Typography
- Base font: system UI stack or `Inter`, 14–16px body.
- Headings use consistent scale (e.g., 1.25x, 1.5x, 2x).
- Code/mono: `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`.

## Spacing & Radius
- Spacing scale: 4px unit (`4, 8, 12, 16, 24, 32`).
- Radius: small `4px`, medium `8px`, large `12px`.

## Light/Dark Theme
- Light (default) uses `--aa-bg` and dark text.
- Dark toggles background to `--aa-bg-inverse`, surface to a darker shade, and inverts text tokens.

CSS example:
```css
html[data-theme="dark"] {
  --aa-bg: var(--aa-bg-inverse);
  --aa-surface: var(--aa-surface-inverse);
  --aa-text: var(--aa-text-inverse);
  --aa-text-muted: #a2aab6;
}
```

## Node‑RED Dashboard
- Configure theme in `ui_base` or Dashboard 2.0 settings.
- Map tokens to Dashboard variables (example):
```json
{
  "theme": {
    "name": "authority-alert",
    "lightTheme": {
      "baseColor": "#0344ff",
      "pageBg": "#ffffff",
      "groupBg": "#f4f6f8",
      "textColor": "#1a1d21"
    },
    "darkTheme": {
      "baseColor": "#0065a3",
      "pageBg": "#0b0b0b",
      "groupBg": "#2a2a27",
      "textColor": "#e0e0e0"
    }
  }
}
```

## TAA Theme (Source: thepolicerecord.com)
- Primary: `#0344ff` (rgba(3,68,255,1))
- Secondary: `#0065a3`
- Accent/Highlight: `#f1d302` / `#f3b61f`
- Success/CTA: `#9be564`
- Danger: `#730001`
- Background (dark): `#0b0b0b`, Surface (dark): `#2a2a27`, Text inverse: `#e0e0e0`

Node‑RED editor overrides (drop into `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/usr/lib/node_modules/node-red/custom.css`):
```css
:root {
  --nr-theme-primary: #0344ff;
  --nr-theme-accent: #f1d302;
}

/* Header branding */
#header {
  background: linear-gradient(rgba(6,68,250,0.2), rgba(0,0,0,0.9));
}

/* Sidebar and workspace tints */
body.red-ui-editor {
  --aa-bg-inverse: #0b0b0b;
  --aa-surface-inverse: #2a2a27;
}
```

## Accessibility
- Target WCAG 2.1 AA contrast: text ≥ 4.5:1, large text ≥ 3:1.
- Ensure interactive states (focus/hover/active) are visible in both themes.

## Assets
- Keep logo, favicon, and wordmark sources under `spec/assets/` (add when available). Provide light/dark variants if needed.

## Maintenance
- Update tokens if branding changes; record the change in `spec/VERSIONS.md` (UI section).
- Validate new components in both themes; add screenshots to `UI_STATUS_SCREEN.md` when available.
