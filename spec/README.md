# Authority Alert – Specs Index

Owner: Authority Alert Team
Last updated: 2025-10-12

This folder is the single source of truth for the platform’s product, build/OTA, UX, security, and deployment documentation. Start here to find the right guide quickly.

## Quick Links
- Build basics (vendor repo): `reCamera-OS/README.md`
- CI pipeline file: `../.gitlab-ci.yml` and guide: [GITLAB_PIPELINE.md](./GITLAB_PIPELINE.md)
- OTA server (local hosting): [OTA_SERVER_DOCKER.md](./OTA_SERVER_DOCKER.md)
- API/OpenAPI specs: `spec/api_spec.yaml`, `spec/user.yaml`, `spec/subject.yaml`
 - System configuration keys: [SYSTEM_CONFIG.md](./SYSTEM_CONFIG.md)
 - Implementation tasks: [IMPLEMENTATION_TASKS.md](./IMPLEMENTATION_TASKS.md)

## Platform Overview
- [CUSTOMIZATION_AND_OTA.md](./CUSTOMIZATION_AND_OTA.md) – firmware layout and OTA flows.
- [BOOT_AND_STARTUP.md](./BOOT_AND_STARTUP.md) – boot sequence, init scripts, service order.
- [PACKAGES_ON_DEVICE.md](./PACKAGES_ON_DEVICE.md) – major packages/services on-device.
- [PRODUCT_FEATURES.md](./PRODUCT_FEATURES.md) – feature summary and accessory matrix.
- Planned: `VOLUMES.md` – storage/partition notes (to be added if needed).

## Build, OTA & Automation
- [DEV_DOCKER.md](./DEV_DOCKER.md) – Docker build environment usage.
- [OTA_SERVER_DOCKER.md](./OTA_SERVER_DOCKER.md) – lightweight OTA web server setup.
- [STREAMING_UPGRADE.md](./STREAMING_UPGRADE.md) – migrate Live555 to MediaMTX.
- [.gitlab-ci.yml](../.gitlab-ci.yml) and [GITLAB_PIPELINE.md](./GITLAB_PIPELINE.md) – CI/CD stages and artifacts.
- [RELEASE_CHECKLIST.md](./RELEASE_CHECKLIST.md) – validations before releasing firmware.

## UI, Node-RED & Experience
- [UI_REDESIGN.md](./UI_REDESIGN.md) – overall UX plan, Node‑RED integration.
- [UI_STATUS_SCREEN.md](./UI_STATUS_SCREEN.md) – status/preview dashboard redesign.
- [UI_THEME.md](./UI_THEME.md) – color palette, tokens, light/dark theming.
- [OOBE_FLOW.md](./OOBE_FLOW.md) – first‑boot wizard.
- [NODERED_UI.md](./NODERED_UI.md) – Node‑RED runtime config and customizations.
- [NODERED_NODES.md](./NODERED_NODES.md) – inventory of nodes and integration additions.
- [POLICE_RECORD_INTEGRATION.md](./POLICE_RECORD_INTEGRATION.md) – integration workflow; covers Auth0/Keycloak, submissions, admin tasks.
- [REMOTE_ACCESS_STRATEGY.md](./REMOTE_ACCESS_STRATEGY.md) – secure remote connectivity plan; includes self‑hosted hole punching.

## Security, Privacy & Compliance
- [PRIVACY_AND_REMOTE_ACCESS.md](./PRIVACY_AND_REMOTE_ACCESS.md) – opt‑in rules for remote features.
- [SECURITY_HARDENING.md](./SECURITY_HARDENING.md) – post‑install locking steps.
- [DATA_AT_REST_ENCRYPTION.md](./DATA_AT_REST_ENCRYPTION.md) – driver‑level encryption (dm‑crypt/LUKS, fscrypt, inline).
- [ONE_TIME_PAD.md](./ONE_TIME_PAD.md) – narrowly scoped OTP mode for manual, small outbound transfers.
- [THREAT_MODEL.md](./THREAT_MODEL.md) – assets, attack surfaces, mitigations.
- [LOGGING_RETENTION.md](./LOGGING_RETENTION.md) – where logs live and retention policy.
- [BACKUP_AND_RECOVERY.md](./BACKUP_AND_RECOVERY.md) – backup strategy and rollback procedures.
- [SUPPORT_IR_PLAYBOOK.md](./SUPPORT_IR_PLAYBOOK.md) – incident response process.
- [REGULATORY.md](./REGULATORY.md) – US compliance overview.
- [ACCESSIBILITY_LOCALIZATION.md](./ACCESSIBILITY_LOCALIZATION.md) – WCAG and localization plan.

## Hardware & Deployment
- [HARDWARE_DEPLOYMENT.md](./HARDWARE_DEPLOYMENT.md) – device specs, network defaults, install steps.

## Versioning & Audits
- [VERSIONS.md](./VERSIONS.md) – pinned versions and runtime snapshot.
- [version_audit.sh](./version_audit.sh) – on‑device script to capture versions.

## Examples
- Police Record config example: `spec/examples/police-record.json` (device loads from `/userdata/config/police-record.json`).

## Conventions
- Each spec begins with `Owner:` and `Last updated:` in ISO‑8601 (YYYY‑MM‑DD).
- Use relative paths prefixed with `./` for intra‑spec links (e.g., `./CUSTOMIZATION_AND_OTA.md`).
- Keep descriptions brief (one sentence) and consistent across sections.

## Maintenance
- Track ongoing work in [DOCUMENTATION_TODO.md](./DOCUMENTATION_TODO.md) and ensure this index stays in sync when new docs are added or renamed.
- Review `PRIVACY_AND_REMOTE_ACCESS.md` and `REGULATORY.md` monthly for policy changes.

## Rendering API Specs
- View OpenAPI files (`api_spec.yaml`, `user.yaml`, `subject.yaml`) in Swagger Editor or ReDoc. Optionally host a local Swagger UI instance and point it at these YAMLs for interactive browsing.
