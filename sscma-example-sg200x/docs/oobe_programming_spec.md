# OOBE (Out-of-Box Experience) — Programming Spec (SG200X reCamera)

Owner: TBD  
Last updated: 2026-01-02

## Summary
This document defines **how** OOBE is implemented: components, APIs, persistence, security properties, and packaging/deploy expectations. It is written to be compatible with the existing `solutions/supervisor` architecture and SG200X reCamera-OS/authority-alert-OS conventions.

## Architecture (recommended)
Implement OOBE as a feature of the existing Supervisor service:
- **Frontend**: static assets served by Supervisor under a dedicated path (e.g. `/oobe/`).
- **Backend**: Supervisor exposes OOBE endpoints and delegates Wi‑Fi operations to its existing Wi‑Fi manager.

Rationale:
- Avoids introducing another HTTP daemon/port.
- Reuses existing auth/session handling and device APIs.
- Centralizes config storage and logging.

If OOBE must be a separate daemon (alternate), it must still:
- Serve the same frontend bundle.
- Provide the same API surface below.
- Share a single persistence format with Supervisor to avoid split-brain config.

## Persistence
Canonical file:
- `/userdata/config/system.json`

Minimum schema (v1):
```json
{
  "oobe_complete": false,
  "oobe_version": 1,
  "consent": {
    "auto_update": false,
    "remote_upload": false,
    "diagnostics": false
  },
  "network": {
    "mode": "ap",
    "ssid": "",
    "ip": "dhcp"
  },
  "security": {
    "admin_password_hash": "",
    "node_red_editor_enabled": false
  },
  "meta": {
    "created_at": "2026-01-02T00:00:00Z",
    "updated_at": "2026-01-02T00:00:00Z"
  }
}
```

Notes:
- `admin_password_hash` must be a salted password hash (e.g., `bcrypt`/`scrypt`/`argon2id` as available in the chosen runtime).
- Store timestamps in ISO‑8601 UTC.

## HTTP Routes
### Frontend
- `GET /oobe/` → serves OOBE SPA (index.html).
- `GET /oobe/assets/*` → static assets (immutable cache headers).

### OOBE API
All endpoints must return JSON with a stable envelope:
```json
{ "ok": true, "data": { } }
```
Errors:
```json
{ "ok": false, "error": { "code": "SOME_CODE", "message": "Human readable", "details": {} } }
```

#### `GET /api/oobe/status`
Returns:
- whether OOBE is required
- current saved draft values (if any)
- whether the device is currently in AP/STA and current SSID (if available)

Example:
```json
{
  "ok": true,
  "data": {
    "oobe_required": true,
    "oobe_complete": false,
    "saved": { "consent": { "auto_update": false }, "network": { "mode": "ap" } }
  }
}
```

#### `POST /api/oobe/consent`
Request:
```json
{ "auto_update": false, "remote_upload": false, "diagnostics": false }
```
Behavior:
- Persist under `consent`.
- Write a local log line (no credentials/PII).

#### `GET /api/wifi/scan`
Returns a list of SSIDs with:
- `ssid`, `rssi`, `security` (open/wpa2/wpa3/unknown)

Must have a timeout and return partial results if possible.

#### `POST /api/wifi/configure`
Request:
```json
{ "ssid": "MyWifi", "password": "****", "hidden": false }
```
Behavior:
- Validate input (length, allowed chars).
- Apply via Supervisor’s Wi‑Fi manager (or equivalent).
- Persist selected SSID and mode `sta` into `system.json`.
- Must not log the password.

Response includes next step:
```json
{ "ok": true, "data": { "action": "restart_networking" } }
```

#### `POST /api/oobe/security`
Request:
```json
{ "admin_password": "****", "node_red_editor_enabled": false }
```
Behavior:
- Hash password; persist hash.
- Configure dependent services if needed (e.g., Node-RED editor on/off) but v1 may just persist the toggle.

#### `POST /api/oobe/finish`
Behavior:
- Validate required fields:
  - `security.admin_password_hash` exists
  - consent keys exist
  - network section exists (even if “ap”)
- Set `oobe_complete=true`, `updated_at`, and persist atomically.
- Optionally trigger a reboot if required by networking changes; otherwise restart only affected services.

Returns:
```json
{ "ok": true, "data": { "oobe_complete": true, "recommended_redirect": "/" } }
```

## Security Model
Threats:
- Someone connected to AP tries to set credentials or enable remote features.

Baseline protections (v1):
- OOBE endpoints are only available when `oobe_complete=false`, OR when a user is authenticated as admin.
- Rate limit sensitive endpoints (`/api/wifi/configure`, `/api/oobe/security`).
- Never log secrets; redact request bodies for sensitive routes.
- Prefer CSRF protections if cookie sessions are used.

Optional hardening (v2):
- Temporary setup token printed on device label / displayed on HDMI/serial; required to submit OOBE changes.

## Logging
- Local-only log file recommended:
  - `/var/log/oobe.log` or Supervisor’s existing log system
- Log events:
  - consent_changed
  - wifi_scan_requested
  - wifi_config_applied (no password)
  - security_config_applied (no password)
  - oobe_finished

## Packaging & Deployment
If implemented inside Supervisor:
- Bundle OOBE frontend into the Supervisor package under the web root (e.g., `/usr/share/supervisor/www/oobe/`).
- Ensure Supervisor routes serve it without exposing admin surfaces.

If implemented as its own solution package:
- Build an `.ipk` like other solutions (see `solutions/supervisor/README.md` for deploy pattern).
- Install locations:
  - binary: `/usr/local/bin/oobe`
  - init script: `/etc/init.d/S??oobe`
  - web assets: `/usr/share/oobe/www/`

## Testing (must-have)
- Fresh boot with no `/userdata/config/system.json` → OOBE loads.
- Submit consent → persisted correctly.
- Wi‑Fi scan returns results (or a clear error).
- Apply Wi‑Fi config with bad password → shows failure; device remains reachable in AP.
- Set admin password → hash stored; plaintext not present in logs/files.
- Finish → `oobe_complete=true`; subsequent boot skips wizard.

## Implementation Notes / Dependencies
- Prefer using existing Wi‑Fi manager code paths (Supervisor already exposes Wi‑Fi endpoints).
- Prefer atomic writes for `system.json` (write temp + fsync + rename).
- Keep OOBE UI assets small to load quickly over AP.

