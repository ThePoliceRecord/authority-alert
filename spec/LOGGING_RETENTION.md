# Logging & Retention Policy

Owner: YOU
Last updated: YYYY-MM-DD

## Logging Goals
- Provide traceability for security events, OTA activity, AI detections, and user actions.
- Balance privacy—logs stay local unless user opts into remote export.

## Log Sources
- **System**: `syslog`, `klogd`, `journalctl` (if using systemd components).
- **Authority Alert Services**:
  - Node-RED logs (`/tmp/node-red/*.log` or custom path).
  - Supervisor / sscma-node logs.
  - OTA script logs (`/var/log/authority-alert/ota.log`).
  - Upload/analysis logs (`/var/log/authority-alert/upload.log`).
- **Network & Firewall**: iptables/nftables logs (optional, controlled via config).

## Retention
- Default retention: 30 days or 500MB per category (whichever first).
- Use `logrotate` configuration:
  - Rotate weekly.
  - Compress archives (`gzip`).
  - Keep 4 rotations.
  - Secure permissions (600 for sensitive logs).
- For agencies requiring longer retention, provide CLI to extend (`authority-config set log_retention 90`).

## Storage & Export
- Logs stored under `/var/log/authority-alert/` and `/userdata/logs/` (overlay).
- User can export bundles via UI/CLI (`authority-support collect-logs`), producing tar.gz for support.
- No automatic remote upload unless user opts in.
- Provide option to ship logs to SIEM via syslog/TLS if agency requests.

## Privacy Considerations
- Avoid storing raw video/audio in logs.
- Redact personal data where possible (e.g., mask tokens, API keys).
- Provide mechanism to purge logs (`authority-support clear-logs`) with audit entry.

## Monitoring & Alerts
- Implement Node-RED flow or supervisor check to alert on:
  - Repeated login failures.
  - OTA failures.
  - Service crashes.
- Alerts can trigger local dashboard badge and optional email/SMS (requires config).

