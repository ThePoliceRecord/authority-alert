# UI & Node-RED Redesign Plan

Owner: Authority Alert Team
Last updated: 2025-10-12

## Objectives
- Retain Node-RED as an automation engine but remove the forced Seed Studio UI integration and login flow.
- Deliver a cleaner landing experience tailored for Authority Alert.
- Preserve the existing startup/loading state so the page stays in sync while services come online.
- Provide an Out-Of-Box Experience (OOBE) for Wi-Fi AP onboarding when no network configuration exists.

## Current Pain Points
- Web UI lives directly inside the Node-RED public folder; dashboard requires Node-RED admin credentials and feels bolted-on.
- Forced login/branding from Seed Studio conflicts with Authority Alert UX goals.
- Node-RED editor shares the same domain and port, increasing attack surface.
- No guided first-boot workflow; camera powers up as AP but UI does not lead users through configuration.

## Proposed Architecture
1. **Front-End Shell**
   - Static SPA served from `/home/recamera/.node-red/public/authority-alert/` (or move to `/mnt/system/webroot`) via custom Nginx/Express service.
   - Landing page handles status polling, OOBE wizard, dashboards, and links to advanced tools.
   - Styling aligned with `spec/UI_THEME.md` palette and background.
   - Provide optional "OTP Export" page/action (when enabled) for manual, small, anonymized exports using removable pad media (see `spec/ONE_TIME_PAD.md`).
    - Integrate OAuth2 login (Auth0 or Keycloak) for Police Record authentication using Authorization Code + PKCE; configuration determines provider endpoints.

2. **Startup / Status API**
   - Lightweight HTTP endpoint (could be part of `sscma-supervisor` or custom Go/Node microservice) exposing:
     - `/api/status` → `{ services: { nodeRed: 'ready'|'starting', camera: 'ready', network: 'up' }, bootState }`
     - `/api/config/network` (GET/POST) for Wi-Fi credentials.
   - Existing startup loading page polls this endpoint; once `nodeRed==='ready'` etc., transitions to main UI.

3. **Node-RED Runtime**
   - Runs headless; Node-RED Dashboard is optional and not part of default landing experience.
   - Expose Node-RED editor only on authenticated route (`/nodered-admin`, optional VPN-only access) or disable by default.
   - Provide curated flows for Authority Alert features; export as default `flows.json` delivered in overlay.
    - Integrate Police Record API flows (see `spec/POLICE_RECORD_INTEGRATION.md`) for submissions and profile sync.

4. **OOBE Flow**
   - Detect first boot by checking for `/userdata/etc/network.conf` (or similar). If missing:
     1. Boot AP (`wlan1` already available) with captive portal or direct redirect.
     2. OOBE UI collects Wi-Fi SSID/password, optional admin password.
     3. POST credentials to `/api/config/network`; service writes config and triggers `wpa_supplicant` reconfigure/reboot.
   - If config exists, skip wizard and render dashboard immediately.
    - Include optional step to connect to Police Record system (authenticate, opt-in) after privacy consent.

5. **Authentication**
   - Landing page uses Authority Alert auth (JWT or session) stored server-side; Node-RED flows interact via REST/MQTT with tokens.
   - Node-RED admin remains disabled unless explicitly enabled; stored credentials set via provisioning pipeline.
   - OTP Export feature is gated by `privacy.otp_export_enabled`; UI exposes controls only when pad media is mounted and validated.

## Implementation Steps
1. **Refactor Static Assets**
   - Move existing Node-RED public assets to new directory structure.
   - Update `settings.js` `httpStatic` to point to new location or serve via separate service.

2. **Status Service**
   - Implement new `/api/status` using supervisor or new daemon.
   - Integrate with startup scripts: when Node-RED fully ready, write readiness file or trigger DBus event consumed by service.

3. **OOBE Module**
   - Build React/Vue (or vanilla) wizard following OOBE flow.
   - Hook to backend network manager (likely `dhcpcd` or NetworkManager replacement). Document CLI fallback.

4. **Node-RED Hardening**
   - Disable default dashboard flows.
   - Provide curated flows under `/home/recamera/.node-red/flows.json` tailored to Authority Alert.
   - Configure `adminAuth` and `httpNodeAuth` as optional components (document enabling process).

5. **Branding & Theme**
   - Apply palette from `spec/UI_THEME.md`.
   - Replace `custom.css` with a new theme matching background imagery.
   - Include new background asset under public static path.

6. **Testing**
   - Boot with no configs → confirm AP + OOBE wizard appears, networks saved, device reconnects.
   - Boot with configs → loading screen transitions to dashboard once services ready.
   - Node-RED accessible only via protected route.
   - OTA upgrade preserves user flows/static assets (ensure overlay path or migration script).

## Deliverables
- Updated `settings.js` and system scripts.
- New front-end bundle + assets (source repo TBD).
- API service for status/config.
- Documentation updates (UI instructions, OOBE manual, admin enablement).

## Open Questions
- Choose framework for landing SPA (React/Next.js, Svelte, or simple vanilla).
- Define exact readiness signals (systemd notify? log watcher?).
- Determine whether Node-RED dashboard is still needed or entirely replaced.
- Decide on captive portal vs simple instructions for AP onboarding.
