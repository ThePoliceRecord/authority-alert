# Implementation Task List

Owner: Authority Alert Team
Last updated: 2025-10-12

This is the human-level, end-to-end task list for delivering privacy-first remote analysis, data-at-rest encryption, and optional OTP export. Use it to create tickets and track work by phase.

## Phase 0 — Align & Decide
- [ ] Product: Approve scope (24‑hour retention, no PII off-device, on-device anonymization defaults). Update `PRIVACY_AND_REMOTE_ACCESS.md` and `REMOTE_ANALYSIS.md` if needed.
- [ ] Security: Finalize threat model and encryption approach. Review `DATA_AT_REST_ENCRYPTION.md` and `THREAT_MODEL.md`.
- [ ] Architecture: Confirm remote analysis hosting (self-hosted baseline; 3rd-party behind DPA). Lock `remote_analysis.yaml`.
- [ ] Legal: Validate broad baseline (GDPR-level consent + CJIS controls) in `REGULATORY.md`.

Acceptance: Written approvals captured in MR comments; docs updated with Owner/date.

## Phase 1 — Data‑at‑Rest Encryption
- [ ] Platform Eng: Create LUKS2-encrypted `/userdata` partition; boot script to unlock + mount (see `DATA_AT_REST_ENCRYPTION.md`).
- [ ] Platform Eng: Implement key management (device‑sealed or passphrase keyslot) and crypto‑erase/wipe utility.
- [ ] DevOps: Enable kernel/userland configs (dm‑crypt, cryptsetup, fscrypt) in build; document in `VERSIONS.md`.
- [ ] QA: Test boot/unlock failure modes, throughput, wipe/rotation.

Acceptance: Clean boot with `/userdata` mounted; wipe removes CEK and data; fio/perf within targets.

## Phase 2 — Remote Analysis Service
- [ ] Backend: Implement `/v1/analyze`, `/v1/analysis/{id}` (GET/DELETE) per `remote_analysis.yaml`; enforce 24h TTL deletion.
- [ ] Backend: Enforce no‑PII schemas, Bearer/mTLS auth, rate limiting, content validation.
- [ ] DevOps: Containerize; deploy staging; logs/metrics/secrets; backups excluded from media content.
- [ ] QA: Conformance tests; TTL/erase tests; load/perf at target QPS.

Acceptance: OpenAPI tests green; GC removes media ≤24h; audit logs show no PII.

## Phase 3 — On‑Device Anonymization & Uploader
- [ ] Edge SW: Build anonymization CLI/daemon (EXIF strip; optional face/plate redaction; audio removal); unit tests.
- [ ] Edge SW: Implement uploader honoring `SYSTEM_CONFIG.md` keys; HTTPS, retries, queue, local audit without PII.
- [ ] Security: Static/dynamic checks that no PII leaves device; config to force redaction by default.
- [ ] QA: Golden-set tests; offline/online transitions; queue durability.

Acceptance: Before/after fixtures verified; uploads blocked when opt‑in disabled; retries bounded.

## Phase 4 — UI & Settings
- [ ] Frontend: Settings toggles for remote analysis (enable, endpoint, token) per `UI_REDESIGN.md` and `SYSTEM_CONFIG.md`.
- [ ] Frontend: Status indicators for remote features (`UI_STATUS_SCREEN.md`).
- [ ] UX: Consent copy, OOBE step referencing privacy posture (`OOBE_FLOW.md`).
- [ ] QA: E2E consent persistence; toggles gate any outbound calls.

Acceptance: Toggling settings updates `system.json`; no network calls when disabled.

## Phase 5 — OTP Export (Optional)
- [ ] Security: Implement `aa-otp` CLI (encrypt/decrypt, pad cursor, one‑time MAC) per `ONE_TIME_PAD.md`.
- [ ] Edge SW: Pad media validation & mount watcher; strict size cap; zeroization.
- [ ] Frontend: “Export (OTP)” action gated by `privacy.otp_export_enabled`; UX for media/pad checks.
- [ ] QA: Pad reuse refused; MAC verifies; no pad/plaintext persists; receipt logged without PII.

Acceptance: Happy-path export/decrypt works; all misuse paths blocked; logs contain no PII.

## Phase 6 — CI/CD, OTA, Packaging
- [ ] DevOps: Extend `.gitlab-ci.yml` to build/package new services; include in OTA artifacts (`GITLAB_PIPELINE.md`).
- [ ] DevOps: OTA publish to staging server (`OTA_SERVER_DOCKER.md`).
- [ ] Platform Eng: Update `version_audit.sh` coverage and `VERSIONS.md` entries for new binaries.
- [ ] QA: Upgrade/rollback tests; A/B slot sanity; artifact integrity.

Acceptance: Pipeline green; OTA install succeeds; rollback safe.

## Phase 7 — Legal/Regulatory Integration
- [ ] Legal: Draft privacy policy/terms reflecting opt‑in, 24h retention, data minimization (`REGULATORY.md`).
- [ ] Security: CJIS alignment checklist; logging/audit scope; access controls.
- [ ] Product: Consent wording and jurisdictional notices in UI; documentation cross‑checks.

Acceptance: Legal sign‑off captured; UI text matches policy.

## Phase 8 — Test & Hardening
- [ ] QA: Full suite—cold/warm boot; power loss during upload; network flaps; lock/unlock cycles.
- [ ] Security: Threat model validation (`THREAT_MODEL.md`); dependency scan; config audit.
- [ ] Perf: Measure CPU/IO with encryption + streaming + anonymization; tune parameters.

Acceptance: All tests pass; documented performance baseline met.

## Phase 9 — Release & Handover
- [ ] Docs: Finalize `README.md` (spec index), changelog, user/admin guides.
- [ ] Release: Complete `RELEASE_CHECKLIST.md` sign‑offs (Product/Security/QA).
- [ ] DevOps: Tag release; push OTA; update secrets rotation schedule.

Acceptance: Release tagged; OTA published; documentation updated with dates/owners.

## Ownership & Labels
- Suggested labels: `platform`, `backend`, `edge`, `frontend`, `security`, `devops`, `legal`, `qa`.
- Assign owners per team capacity; maintain milestones matching phases.

