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
  --aa-color-primary: #1E88E5;
  --aa-color-secondary: #6C5CE7;
  --aa-bg: #ffffff;
  --aa-bg-inverse: #0f1115;
  --aa-surface: #f4f6f8;
  --aa-text: #1a1d21;
  --aa-text-muted: #5c6672;
  --aa-text-inverse: #e6e9ee;
  --aa-success: #2e7d32;
  --aa-warning: #ef6c00;
  --aa-danger: #c62828;
  --aa-info: #0288d1;
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
  --aa-surface: #161922;
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
      "baseColor": "#1E88E5",
      "pageBg": "#ffffff",
      "groupBg": "#f4f6f8",
      "textColor": "#1a1d21"
    },
    "darkTheme": {
      "baseColor": "#6C5CE7",
      "pageBg": "#0f1115",
      "groupBg": "#161922",
      "textColor": "#e6e9ee"
    }
  }
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

