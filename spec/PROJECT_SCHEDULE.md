# Rough Implementation Schedule

Owner: YOU
Assignee: Mid-level engineer (8am–5pm, 1 hour lunch)
Last updated: YYYY-MM-DD

Assumptions:
- Single engineer working ~6 productive hours/day (rest consumed by meetings/context switching).
- Existing documentation/specs complete.
- External dependencies (API access, Auth0/Keycloak credentials, OTA server hardware) are available.
- Buffer of ~15% built in for interruptions.

## Week 1 – Foundations & Prep
- Day 1: Environment setup (Docker build, GitLab runner smoke test) and repo orientation.
- Day 2: Review specs, create task board, confirm asset access (Police Record sandbox, Auth0/Keycloak tenant).
- Day 3–4: Stub status API service (health checks, service registry).
- Day 5: Design UI components (wireframes) for status screen and OOBE; confirm palette assets.

## Week 2 – Backend Services
- Day 6–7: Implement `/api/status` service (system health, camera, network, OTA states).
- Day 8: Build config service for privacy & OTA toggles (reads/writes `/userdata/config`).
- Day 9–10: Prototype MediaMTX ingestion pipeline (parallel to Live555) and document findings.

## Week 3 – Front-End Shell & Status Dashboard
- Day 11–13: Scaffold SPA, integrate theme/background, hook to status API.
- Day 14: Implement system tiles, service indicators, action bar.
- Day 15: Integrate live preview component (initially MJPEG snapshot fallback).

## Week 4 – OOBE & Privacy Flow
- Day 16–17: Build OOBE screens (welcome, privacy consent, network setup, security).
- Day 18: Wire OOBE backend endpoints and config storage.
- Day 19: Add optional OAuth2 (Auth0/Keycloak) step; handle tokens securely.
- Day 20: End-to-end test OOBE → dashboard transition.

## Week 5 – Police Record Integration
- Day 21: Implement OAuth2 client (PKCE) for Auth0/Keycloak.
- Day 22–23: Node-RED/Backend flows for profile update & submissions (multipart upload).
- Day 24: Admin features (user list, face download) with role checks.
- Day 25: UI forms + error handling; ensure opt-in controls respected.

## Week 6 – Streaming Migration, Remote Access Prototype & Node-RED Hardening
- Day 26–27: Replace Live555 with MediaMTX (build/package, init script, config).
- Day 28: Update preview component to use MediaMTX streams (HLS/WebRTC).
- Day 29: Prototype self-hosted secure remote connectivity (WireGuard/headscale) per `REMOTE_ACCESS_STRATEGY.md`.
- Day 30: Audit Node-RED flows, disable legacy dashboard, lock palette manager; document remote access findings.

## Week 7 – Security, Backups, Logging
- Day 31–32: Automate hardening steps (scripts/tooling per `SECURITY_HARDENING.md`).
- Day 33: Implement logging rotation/export utilities.
- Day 34: Build backup/restore UI/CLI helpers.
- Day 35: Run threat-model validation, update docs with any new mitigations.

## Week 8 – QA, Release Prep
- Day 36–37: Execute release checklist (functional tests, OTA simulation, Police Record flows, OAuth refresh, streaming regression).
- Day 38: Fix defects, re-run targeted tests.
- Day 39: Finalize documentation (screenshots, README updates) and prepare release notes.
- Day 40: RC build, OTA server staging, stakeholder demo, sign-offs.

## Post-Week 8 (Buffer & Support)
- Allocate additional week if integration testing with agencies uncovers issues or if MediaMTX tuning requires more time.
