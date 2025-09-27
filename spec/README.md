# Documentation Map

Owner: YOU
Last updated: YYYY-MM-DD

## Platform Overview
- `CUSTOMIZATION_AND_OTA.md` – how the firmware is organized and how OTA updates work.
- `BOOT_AND_STARTUP.md` – boot sequence, init scripts, and service order.
- `PACKAGES_ON_DEVICE.md` – major packages/services included on the device.
- `VOLUMES.md` *(if added later)* – storage/partition notes.

## Build, OTA & Automation
- `DEV_DOCKER.md` – Docker build environment usage.
- `OTA_SERVER_DOCKER.md` – lightweight OTA web server (nginx) setup.
- `STREAMING_UPGRADE.md` – plan to replace Live555 with MediaMTX.
- `.gitlab-ci.yml` & `GITLAB_PIPELINE.md` – CI/CD pipeline stages.
- `RELEASE_CHECKLIST.md` – tests and approvals before releasing firmware.

## UI, Node-RED & Experience
- `UI_REDESIGN.md` – overall UX plan, Node-RED integration.
- `UI_STATUS_SCREEN.md` – status/preview dashboard redesign.
- `UI_THEME.md` – color palette & styling.
- `OOBE_FLOW.md` – first-boot wizard.
- `NODERED_UI.md` – Node-RED runtime config and customizations.
- `NODERED_NODES.md` – inventory of Node-RED nodes and integration additions.
- `POLICE_RECORD_INTEGRATION.md` – workflow for connecting with Police Record system (Auth0/Keycloak, submissions, admin tasks).

## Security, Privacy & Compliance
- `PRIVACY_AND_REMOTE_ACCESS.md` – opt-in rules for remote features.
- `SECURITY_HARDENING.md` – post-install locking steps.
- `THREAT_MODEL.md` – assets, attack surfaces, mitigations.
- `LOGGING_RETENTION.md` – where logs live and how long they stay.
- `BACKUP_AND_RECOVERY.md` – backup strategy and rollback procedures.
- `SUPPORT_IR_PLAYBOOK.md` – incident response process.
- `REGULATORY.md` – US compliance overview.
- `ACCESSIBILITY_LOCALIZATION.md` – WCAG + localization plan.

## Hardware & Deployment
- `HARDWARE_DEPLOYMENT.md` – device specs, network defaults, install steps.

## Versioning & Audits
- `VERSIONS.md` – pinned versions + runtime audit snapshot.
- `version_audit.sh` – script to capture versions from a device.

## To Keep in Sync
- `DOCUMENTATION_TODO.md` – reminders for maintenance & future additions.

