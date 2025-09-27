# Status & Preview Screen Revamp

Owner: YOU
Last updated: YYYY-MM-DD

## Goals
- Replace the legacy Seed UI status page with a modern Authority Alert dashboard.
- Present clear system state (camera, network, storage, OTA) and live preview.
- Integrate with startup loading flow and Node-RED backend while keeping the experience responsive on limited hardware.

## Status Screen Requirements
1. **Global Header**
   - Authority Alert branding, background from `spec/UI_THEME.md`.
   - Current time, device name, build version (from `sg2002_recamera_emmc_md5sum.txt` / `swupdate` version).
2. **System Tiles** (responsive grid)
   - *Camera*: sensor status, resolution, FPS, recording state.
   - *Network*: AP/STA state, IPs, Wi-Fi signal strength, OTA server connectivity.
   - *Remote Access*: tunnel status (disabled/connecting/connected), relay server info, last handshake (only shown if feature enabled).
   - *Storage*: usage on rootfs A/B, `/userdata`, SD card.
   - *Services*: Node-RED, Mosquitto, Supervisor, OTA agent (running? restart controls?).
   - *Alerts*: last OTA result, CVE status (from version audit), error events.
3. **Live Preview Panel**
   - Upgraded from static image to low-latency stream preview.
   - Primary options:
     - MJPEG stream via existing HTTP endpoint (if available).
     - WebRTC or HLS fallback (depending on camera capabilities).
     - Snapshot refresh if bandwidth-constrained (still image with manual refresh).
   - Overlay camera metadata (FPS, exposure, temperature) if available from Node-RED/sscma API.
4. **Action Bar**
   - Buttons: Start recording, Trigger snapshot, Toggle IR/LED, Run diagnostics, Launch Node-RED flows.
   - Quick link to OTA updates page.
5. **Mobile/Tablet Support**
   - Layout collapses gracefully, preview and crucial status front and center.

## Loading & Startup Behavior
- Reuse existing startup loading screen but attach to the new status endpoint.
- Display phased progress (boot → services → camera ready → network up → Node-RED ready).
- Auto-transition to dashboard once all required services report `ready`.
- Provide fallback “limited mode” message if camera offline but network/service OK.

## Data Sources
- `/api/status` (new service per `spec/UI_REDESIGN.md`)
  - Provide JSON with camera, network, storage, services, OTA.
- Live preview stream endpoint (existing or new pipeline via Supervisor/Node-RED).
- `version_audit` report or Node-RED flow to fetch current component versions.
- OTA info from `swupdate` or custom log aggregator.

## UI Technology
- SPA (React/Vue/Svelte) or lightweight vanilla JS + Web Components (choose based on team preference).
- Use CSS variables derived from `spec/UI_THEME.md` for consistent colors.
- Deploy bundle under `/home/recamera/.node-red/public/authority-alert/` or dedicated web root.

## Implementation Steps
1. **Design**
   - Wireframe desktop/mobile layouts.
   - Define component structure (Header, SummaryGrid, PreviewPanel, AlertsList, ActionBar).
2. **API Contracts**
   - Document shape of `/api/status` responses and control endpoints (e.g., `POST /api/camera/action`).
   - Decide preview stream protocol (MJPEG vs WebRTC).
3. **Frontend Build**
   - Scaffold project with Vite/Next/etc.
   - Implement components + state management (poll `/api/status` every few seconds or use WebSocket push).
   - Integrate preview (video tag / canvas / WebRTC).
4. **Backend Hooks**
   - Extend Supervisor/Node-RED to expose required metrics and control endpoints.
   - Ensure camera pipeline can deliver preview stream accessible via new UI.
5. **Deployment**
   - Bundle assets and copy into overlay or OTA update package.
   - Update `settings.js` `httpStatic` path if necessary.
   - Adjust loading screen to point to new landing page once ready.
6. **QA**
   - Boot scenarios (cold boot, camera disconnected, network failure).
   - Performance check on device CPU (ensure preview doesn’t overwhelm system).
   - Security check (no unauthenticated control endpoints; support auth from `spec/UI_REDESIGN.md`).

## Future Enhancements
- Integration with analytics (historical charts, alerts timeline).
- Multi-camera view with quick switching if hardware supports it.
- Night/day theme toggle.
- User roles (viewer vs admin) with limited controls.
- Remote access diagnostics view (debug tunnel setup, show recommended actions).
