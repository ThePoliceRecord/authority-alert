# Police Record System Integration Requirements

Owner: YOU
Last updated: YYYY-MM-DD

Based on `spec/user.yaml` (The Police Record User Management API) and current production backend notes (Auth0) with optional support for Keycloak.

## Authentication & Sessions
- Production backend uses **Auth0** (OAuth2/OpenID Connect); some deployments may use **Keycloak** instead.
  - Obtain client ID, domain (Auth0) or realm URL (Keycloak), and audience from administrators.
  - Implement OAuth2 Authorization Code + PKCE flow reusable for both providers (parameterized endpoints).
  - Store returned ID/access/refresh tokens securely (encrypted at rest) and handle refresh flows (Auth0 refresh tokens or Keycloak refresh tokens).
- Maintain compatibility with legacy Firebase/Django session scheme from `spec/user.yaml` until API spec updated; detect supported auth via configuration.
- Provide logout/reset option that clears tokens and revokes refresh tokens via appropriate provider endpoint (Auth0 revocation API or Keycloak logout endpoint).

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

## UI/UX Notes
- Combine with status dashboard: show integration status (connected, auth expired, submissions pending).
- Provide option to manually sync or resubmit failed uploads.
- Respect opt-in privacy stance: integration disabled until user authorizes connection.

## Next Steps
- Obtain API documentation for department listings and additional endpoints (if any).
- Define role mapping (camera user vs admin vs investor) and how tokens are issued.
- Implement Node-RED flows or backend microservice to manage submissions and profile updates.
- Update testing checklist to include Police Record integration scenarios.
