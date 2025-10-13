# Remote Analysis Architecture

Owner: Authority Alert Team
Last updated: 2025-10-12

## Decision
- Preferred deployment: self-hosted analysis service under customer/agency control to minimize data sharing and meet CJIS/state privacy constraints.
- Third-party use is optional and requires: DPA (Data Processing Agreement), documented purpose limitation, encryption in transit, and on-device anonymization before upload.
- Data minimization: only anonymized media and minimal, pseudonymous metadata leave the device. No user PII is collected or transmitted.

## Anonymization Pipeline (On-Device)
- Strip EXIF and embedded metadata from images/videos.
- Redact faces and license plates (blur/mosaic) when not strictly required for the selected analysis task.
- Remove audio unless analysis explicitly requires it and the user consents.
- Replace device identifiers with ephemeral pseudonymous IDs (rotating UUIDs). Do not transmit account names/emails/IPs.
- Coarse location (optional): snap to ≥1 km grid or omit entirely; default is omit.

## Upload Policy & Retention
- Uploads disabled by default; require explicit, revocable opt-in.
- Retention (server side): 24 hours max, then hard delete (media + derived artifacts). Earlier deletion is supported via API.
- A local audit entry records upload time, destination, and delete timestamp (no PII).

## API
- OpenAPI stub: `spec/remote_analysis.yaml` defines endpoints for `/v1/analyze`, `/v1/analysis/{id}`, deletion, and health.
- Authentication: Bearer token (short-lived). For higher assurance, add mTLS.
- No PII fields; schema restricts metadata to event ID, timestamps, and pseudonymous device ID.

## Architecture
- Device packages anonymized media + minimal JSON metadata → HTTPS POST to analysis service.
- Service returns `analysis_id` and status. Device polls or receives webhook for results (optional future addition).
- Device deletes local upload cache upon confirmed receipt; server enforces 24h TTL.

## Operations
- Configuration: users set endpoint/base URL and token via settings; stored under `/userdata/config/system.json`.
- Deletion: user can trigger immediate deletion via device UI (calls server DELETE) or wait for TTL.
- Auditing: retain minimal audit logs locally; do not include content or PII.

## Security
- TLS 1.2+ required. Enforce strong cipher suites.
- Scope tokens to least privilege and short lifetimes (≤1h).
- Rate-limit uploads and validate content types.

