# Testing & Release Checklist

Owner: YOU
Last updated: YYYY-MM-DD

## Pre-Build
- [ ] Review outstanding issues/CVEs; update target versions in `spec/VERSIONS.md`.
- [ ] Confirm documentation updates (UI changes, privacy policy, etc.).
- [ ] Verify OTA server manifest path for new release (`spec/OTA_SERVER_DOCKER.md`).

## Build Stage
- [ ] Run `./docker_build.sh sg2002_recamera_emmc` (via GitLab pipeline or local).
- [ ] Ensure build logs show no errors; capture artifact path.
- [ ] Record firmware version string (from CHANGELOG).

## Automated Tests
- [ ] GitLab pipeline `version_audit` job passed (check `/tmp/version_audit.txt`).
- [ ] Static analysis / linting (if available) returns clean.
- [ ] Optional: run unit/integration tests for Node-RED flows (future automation).

## Manual Validation
- [ ] Flash to reference device; allow boot.
- [ ] OOBE wizard functions: privacy step, network setup, admin password.
- [ ] New status dashboard loads; preview works; services show correct status.
- [ ] AI detection flow triggers expected events.
- [ ] OTA update simulation: `latest` + `download` + `start` using new manifest.
- [ ] Remote upload feature respects consent (off by default, prompts when enabled).
- [ ] Node-RED editor disabled unless intentionally enabled.
- [ ] Police Record integration: OAuth2 login/refresh (Auth0/Keycloak), profile update, officer submission form, admin download (if applicable).

## Security & Privacy
- [ ] Run `spec/version_audit.sh` on device; compare to baseline to ensure components updated.
- [ ] Confirm firewall/iptables rules applied (per hardening guide).
- [ ] Verify logs rotate correctly; no PII leaked.
- [ ] Check privacy toggles persisted and default to opt-out.

## Documentation & Packaging
- [ ] Update README/release notes with summary + upgrade instructions.
- [ ] Copy OTA zip and manifest to OTA server staging directory.
- [ ] Update `spec/CHANGELOG.md` or main `CHANGELOG.md` entry.
- [ ] Ensure new documents (if any) committed.

## Release Approval
- [ ] Product owner sign-off (name/date).
- [ ] Security sign-off (name/date).
- [ ] QA sign-off (name/date).

## Post-Release
- [ ] Publish OTA artifacts (`publish` stage or manual upload).
- [ ] Notify stakeholders/customers; include release notes & security updates summary.
- [ ] Monitor support channels for issues; be prepared to rollback.
