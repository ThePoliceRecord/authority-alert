# OOBE (Out-of-Box Experience) — 40‑Hour Implementation Plan (1 Week)

Owner: TBD  
Last updated: 2026-01-02

## Scope
This plan targets a v1 OOBE that meets `docs/oobe_spec.md` and `docs/oobe_programming_spec.md`:
- OOBE UI (Welcome → Consent → Network → Security → Review → Complete)
- Persistence to `/userdata/config/system.json`
- Wi‑Fi scan + configure via existing device services (preferred: Supervisor)
- Minimal hardening (gating + rate limit + no secrets in logs)
- Package build + deploy instructions

Out of scope for the 40 hours:
- Captive portal, localization, SSO/OAuth, remote relay onboarding, full redesign of main dashboard.

## Deliverables (end of week)
- OOBE frontend served at `/oobe/`
- OOBE API implemented (`/api/oobe/*`, `/api/wifi/*` as needed)
- Config persistence + completion gating
- Build/deploy steps validated on a real camera
- Short “how to test” checklist

## Hour-by-hour Breakdown (40h)
### Day 1 (8h) — Requirements + skeleton
- (2h) Align on exact flow + copy; confirm acceptance criteria.
- (2h) Define JSON schema + atomic write approach; confirm locations (`/userdata/config/system.json`).
- (2h) Create minimal UI skeleton with routes/steps + state machine.
- (2h) Stub backend endpoints with mock responses; wire UI to stubs.

### Day 2 (8h) — Consent + persistence
- (2h) Implement config read/write (atomic) + status endpoint.
- (2h) Implement consent endpoint + UI; add local log events.
- (2h) Implement gating rules (`oobe_complete=false`).
- (2h) Add basic error handling + loading states.

### Day 3 (8h) — Wi‑Fi onboarding
- (3h) Implement Wi‑Fi scan endpoint (or integrate existing Supervisor Wi‑Fi scan).
- (3h) Implement Wi‑Fi configure endpoint + apply behavior (restart/reboot decision).
- (2h) UI: scan list, hidden SSID, retry flows, “stay in AP mode”.

### Day 4 (8h) — Security + finish flow
- (3h) Implement admin password set (hash + persist); ensure no plaintext leakage.
- (2h) Implement review/confirm + finish endpoint; mark `oobe_complete=true`.
- (2h) Smoke test end-to-end locally (dev environment) and fix defects.
- (1h) Add rate limiting / basic protections on sensitive routes.

### Day 5 (8h) — On-device validation + docs
- (4h) Deploy to camera, validate cold boot flows, failure handling, and post-OOBE behavior.
- (2h) Fix camera-only issues (permissions, paths, service restarts).
- (2h) Write/update docs: install, runbook, testing checklist, troubleshooting.

## Risks / Unknowns
- Wi‑Fi management ownership (Supervisor vs separate service) may affect Day 3 estimates.
- Device-specific networking differences (wpa_supplicant, interface names) may require extra camera time.
- If a captive portal is required, it will exceed 40 hours.

