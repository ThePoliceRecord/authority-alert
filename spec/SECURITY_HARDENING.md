# Security Hardening Guide

Owner: YOU
Last updated: YYYY-MM-DD

## Purpose
Checklist for locking down an Authority Alert camera after provisioning. Apply during factory prep, first deployment, and periodic audits.

## 1. Credentials & Accounts
- Set unique passwords for:
  - Web UI / admin console.
  - Node-RED editor (if enabled) via `settings.js` `adminAuth` (bcrypt hash required).
  - SSH user `recamera` (via `passwd recamera`).
- Disable Node-RED editor unless actively developing (`disableEditor=true`).
- Disable SSH password login if using keys (update `/etc/ssh/sshd_config`).

## 2. Services & Network
- Review enabled init scripts (`/etc/init.d/S*`). Disable unneeded ones (e.g., `S98ttyd` if SSH sufficient).
- Configure firewall (iptables or nftables):
  - Allow inbound: 22 (SSH), 1880/redirect if UI needed, 9090 only if TTYD required, OTA server IP (outbound).
  - Block outbound traffic by default; allow OTA host, optional analysis host.
- Disable AP mode if not needed (`ifconfig wlan1 down`, update config).
- Enforce HTTPS on UI via reverse proxy or TLS termination (Caddy/Nginx).

## 3. Updates
- Apply latest firmware: use OTA zip from trusted server.
- Run `spec/version_audit.sh` post-update and compare to target versions in `spec/VERSIONS.md`.
- Document update consent (per privacy policy) and keep logs (`/var/log/authority-alert/ota.log`).

## 4. Node-RED & Flows
- Remove sample/test flows; deploy only Authority Alert flows.
- Validate third-party nodes against CVE database; freeze versions in `package-lock.json`.
- Separate privileged operations (shell commands) from user-triggered flows with approval step.
- Monitor flow exports/imports; restrict palette manager updates.

## 5. Storage & Data
- Encrypt sensitive archives before off-device transfer. If using removable media, enforce physical security.
- Configure log rotation (`/etc/logrotate.d/authority-alert`).
- Clear leftover setup files (`/userdata/oobe.tmp`, etc.).

## 6. Monitoring & Alerts
- Enable system health checks (supervisor) to watch CPU, disk, service status.
- Forward critical alerts to admin via secure channel (email/SMS) using Node-RED flows.

## 7. Physical Security
- Mount cameras in tamper-resistant locations.
- Lock down debug ports (UART, JTAG) where feasible; cover connectors or epoxy after provisioning.

## 8. Audit Trail
- Maintain spreadsheet or CMDB entry tracking:
  - Device ID, location, firmware version, last hardening date, security contacts.
- Schedule quarterly review of hardening steps and document results.

