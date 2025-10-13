# System Configuration Keys

Owner: Authority Alert Team
Last updated: 2025-10-12

This document lists configuration keys expected by system services and the UI. The configuration is stored at `/userdata/config/system.json`.

## Keys
- `privacy.remote_analysis_enabled` (bool, default `false`)
  - Enables remote analysis uploads (see `PRIVACY_AND_REMOTE_ACCESS.md`, `REMOTE_ANALYSIS.md`).
- `privacy.otp_export_enabled` (bool, default `false`)
  - Enables manual OTP export feature in the UI (see `ONE_TIME_PAD.md`).
- `privacy.otp_export_max_mb` (int, default `10`)
  - Size cap for OTP export payloads in megabytes.
- `privacy.remote_analysis_base_url` (string, default empty)
  - Endpoint base URL for analysis server.
- `privacy.remote_analysis_token` (string, default empty)
  - Bearer token for analysis API; store securely; rotate regularly.

## Example
```json
{
  "privacy": {
    "remote_analysis_enabled": false,
    "remote_analysis_base_url": "",
    "remote_analysis_token": "",
    "otp_export_enabled": false,
    "otp_export_max_mb": 10
  }
}
```

## Notes
- The UI reads these keys to decide which features to expose. Services must re-read or be signaled when values change.
- Secrets (e.g., tokens) should be stored with restrictive permissions and never logged.
