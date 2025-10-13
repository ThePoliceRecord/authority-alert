# Police Record System Integration Requirements

Owner: YOU
Last updated: 2025-10-12

Based on `spec/user.yaml` (The Police Record User Management API) and current production backend notes (Auth0) with optional support for Keycloak.

## Authentication & Sessions
- Production backend uses **Auth0** (OAuth2/OpenID Connect); some deployments may use **Keycloak** instead.
  - Obtain client ID, domain (Auth0) or realm URL (Keycloak), and audience from administrators.
  - Implement OAuth2 Authorization Code + PKCE flow reusable for both providers (parameterized endpoints).
  - Store returned ID/access/refresh tokens securely (encrypted at rest) and handle refresh flows (Auth0 refresh tokens or Keycloak refresh tokens).
- Maintain compatibility with legacy Firebase/Django session scheme from `spec/user.yaml` until API spec updated; detect supported auth via configuration.
- Provide logout/reset option that clears tokens and revokes refresh tokens via appropriate provider endpoint (Auth0 revocation API or Keycloak logout endpoint).

## Roles & RBAC
- Role detection is driven by a configurable claim path to support both providers.
  - Preferred `role_claim` (configurable):
    - Auth0: namespaced custom claim, e.g. `https://product.example/roles` or `permissions` depending on tenant policy.
    - Keycloak: `realm_access.roles` or `resource_access[client_id].roles`.
  - Fallback order when `role_claim` not set: `resource_access[client_id].roles` → `realm_access.roles` → `permissions`.
- Required roles:
  - `admin`: grants access to admin panel and sensitive exports.
  - `investor`: grants investor-only informational pages.
  - `camera_user`: baseline device user for submissions and profile.
- Scopes (if enforced server-side): `profile:read`, `profile:write`, `submission:write`, `admin:read`, `admin:export`.

## Token Validation
- Validate `iss`, `aud`, `exp`, and (if present) `nbf`/`iat` with up to 300s clock skew.
- Fetch JWKS from provider and cache keys for 12h with background refresh; honor `kid` rotation.
- On `kid` miss, refetch JWKS once per 60s to avoid thundering herd; fail closed with actionable error.
- Enforce TLS verification; configurable CA bundle path for air‑gapped installs.
- Refresh tokens: rotate on use where supported; immediately persist updated refresh token.
- Logout must call provider revocation endpoint (Auth0) or Keycloak logout and clear local secrets.

## User Profile Management
- Allow authorized users to view and update their profile via `/profile/`.
  - Fields: display name, email, bio, profile picture URL, mailing list preference, achievements, strikes, etc.
  - Provide validation and feedback on update success/failure.

## Officer Submission Workflow
- Provide form for `/submit/{department_id}/` with multipart upload.
  - Fields include department ID selector, officer name, rank, badge number, demographics, undercover flag, status, and optional photo upload.
  - Support both GET (preload submission page) and POST (submit form) flows.
  - Handle image encoding/validation before upload.
- Allow multiple department entries; fetch department list from upstream service (TBD) or static config.
- Display submission confirmation and track status.

## Administrative Features (privileged users)
- Endpoint `/api/users/v1/` returns all user profiles (requires admin role).
  - Provide secure admin panel (role-based access) to list/search/export profiles.
- Endpoint `/download/` delivers ZIP of all faces in database.
  - UI must warn about sensitive data, require explicit confirmation, and restrict to admins.

## API Contract

### Department List
- GET `/api/departments` → 200 OK
```json
{
  "departments": [
    { "id": "dep_123", "name": "Springfield PD", "code": "SPD" },
    { "id": "dep_456", "name": "Shelbyville PD", "code": "SVPD" }
  ]
}
```
- Caching: respect `ETag`/`Last-Modified`; local TTL default 24h with manual refresh.

### Officer Submission
- POST `/api/officers`
- Idempotency: client sends `Idempotency-Key` (UUIDv4) header. Server must return the same result for retries.
- Two supported encodings:
  1) `multipart/form-data`
     - `metadata` (JSON, UTF‑8) with fields below
     - `photo` (0..N files) subject to `allowed_mime` and `max_bytes`
  2) `application/json` with base64 photos

Metadata shape (required unless marked optional):
```json
{
  "department_id": "dep_123",
  "officer": {
    "name": "Jane Doe",
    "rank": "Sergeant",
    "badge_number": "S-1024",
    "demographics": { "age": 38, "gender": "female", "ethnicity": "Hispanic" },
    "undercover": false,
    "status": "active"  // enum: active, suspended, retired
  },
  "notes": "Observed on 5th Ave.",
  "photos": [
    {
      "content_type": "image/jpeg",
      "base64": "...",
      "sha256": "<hex>"
    }
  ]
}
```

Responses:
- 201 Created (synchronous)
```json
{ "id": "off_789", "status": "created", "duplicate": false }
```
- 202 Accepted (asynchronous)
```json
{ "id": "off_789", "status": "processing", "status_url": "/api/officers/off_789/status" }
```
- Status polling (if async): GET `/api/officers/{id}/status` → `{ "state": "processing|completed|failed", "errors": [] }`

### Errors & Rate Limits
- Standard error model:
```json
{ "code": "UNAUTHORIZED", "message": "token expired", "details": {}, "request_id": "r-abc123" }
```
- 401 → prompt re‑login, 403 → show insufficient role, 404 → not found, 409 → idempotency conflict, 413 → payload too large, 415 → unsupported media type.
- 429 → respect `Retry-After`; apply exponential backoff with jitter.

## Informational Pages (link-outs)
- Provide shortcuts in UI to open platform pages (`/terms/`, `/privacy/`, `/about/`, `/info`, `/info/seed`, `/info/terms`, `/info/faqs`, `/info/financials`).
  - Load via embedded webview or external browser (ensure authentication where required).

## Investor/Admin Separation
- Keep investor-specific pages hidden from standard users.
- UI should detect role from profile or token claims and show relevant links.

## Error Handling
- Implement friendly handling for standard API responses:
  - 401 Unauthorized → prompt re-login.
  - 404 Not found → show department/page missing message.
  - 500 Server error → display contact info / retry.
- Parse `ErrorModel` messages for user feedback.

## Node-RED / Backend Considerations
- Use Node-RED HTTP Request nodes or dedicated backend service to bridge camera to Police Record API.
- Implement OAuth2 token acquisition within backend service (preferred) or via Node-RED OAuth2 client; support both Auth0 and Keycloak endpoints. Cache access tokens with expiry awareness and refresh proactively.
- Ensure TLS validation, retry/back-off strategy, and logging for audit.
- Store API endpoints, provider type (Auth0/Keycloak), client ID/secret, domains, and refresh tokens in secure config (`/userdata/config/police-record.json`).

## Reliability & Offline
- Outbox queue persists pending submissions on disk; flush on connectivity restore.
- Retries: exponential backoff with full jitter, capped by `retries.max_attempts`.
- Circuit breaker after repeated failures; resume after cool‑down.
- Time budgets: per‑request timeout default 10s, uploads 60s; configurable.
- Clock sync required for token validity; ensure NTP.

## Security & Compliance
- PII minimization; only fields listed in metadata shape should be collected. Redact PII from logs.
- Secrets (tokens, refresh tokens, client secret if used) encrypted at rest and permission‑restricted.
- Upload validation: content type allow‑list, size limits, MIME sniffing, and SHA‑256 hashing.
- Retention policy documented and enforced for queued data and uploaded media.
- Consider CJIS‑aligned controls if data classification requires.

## Observability
- Structured logs with `request_id` correlation; avoid raw PII.
- Metrics: `auth_success_total`, `auth_failure_total`, `submission_queued`, `submission_sent_total`, `submission_retry_total`, `submission_latency_ms`, `queue_depth`.
- Admin actions audit log including `/download/` usage (who/when/what).

## Configuration
Location: `/userdata/config/police-record.json` (device) with the following schema (keys shown; types implied):
```json
{
  "provider": "auth0 | keycloak",
  "auth": {
    "domain": "<auth0-domain>",
    "realm_url": "<keycloak-realm-url>",
    "client_id": "<string>",
    "audience": "<api-audience-or-resource>",
    "scopes": ["profile:read", "submission:write"]
  },
  "api": {
    "base_url": "https://api.example",
    "dept_list_url": "https://api.example/departments",
    "timeout_ms": 10000,
    "tls": { "verify": true, "ca_path": "/etc/ssl/certs/ca-certificates.crt" }
  },
  "rbac": {
    "role_claim": "https://product.example/roles",
    "admin_roles": ["admin"],
    "investor_roles": ["investor"],
    "camera_roles": ["camera_user"]
  },
  "upload": {
    "allowed_mime": ["image/jpeg", "image/png"],
    "max_bytes": 5242880,
    "concurrency": 2
  },
  "retries": { "max_attempts": 5, "base_ms": 500, "jitter_ms": 250 },
  "queue": { "enabled": true, "path": "/userdata/queues/police-record", "flush_interval_sec": 30 },
  "logging": { "level": "info", "pii_scrub": true }
}
```

Sample config file is provided at `authority_alert/spec/examples/police-record.json`.

## UI/UX Notes
- Combine with status dashboard: show integration status (connected, auth expired, submissions pending).
- Provide option to manually sync or resubmit failed uploads.
- Respect opt-in privacy stance: integration disabled until user authorizes connection.

## Testing
- OAuth provider mocking (Auth0/Keycloak); token expiry, rotation, and revocation tests.
- E2E: submission (sync and async), offline queue, resubmission, admin export, and permission gating.
- Negative: large file rejection, invalid MIME, missing roles, idempotency retries.

## Open Questions
- Confirm sync vs async submissions and status polling contract.
- Confirm definitive role source (token vs profile API) and claim namespace.
- Department list ownership, cadence, and caching policy.
- Maximum photo size/types and server‑side validation posture.

## Next Steps
- Finalize role mapping and claim path; update config defaults.
- Confirm submission mode (sync/async) and status endpoint shape.
- Publish department list endpoint contract and caching headers.
- Approve upload limits and retention windows.
- Implement Node‑RED/backend service using this spec and add tests.
