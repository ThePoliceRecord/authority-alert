# Privacy & Remote Access Policy

Owner: Authority Alert Team
Last updated: 2025-10-12

## Goals
- Default stance: camera does not call home or send data externally unless explicitly enabled by the user.
- Provide optional services (update notifications, remote analysis) that can be turned on/off by configuration.
- Document data paths, network endpoints, and UI/UX for user consent.

## Default Behavior (Out-of-the-box)
- **Consent-first**: every cloud/remote function ships disabled. Device never “phones home” until the owner turns a feature on and confirms the destination.
- No outbound connections beyond essential package repositories (if any) during build; runtime should not contact external services.
- OTA checking disabled until the user sets `/etc/upgrade` or toggles “Check for updates” in UI.
- Remote upload/analysis features disabled; only local processing occurs.
- Telemetry logging restricted to local storage.

## Optional Features (User Opt-In)
1. **Update Notifications**
   - UI toggle: “Automatically check for updates” → when enabled, device polls configured OTA server at defined interval.
   - Endpoint defined by user (default blank). Provide field in settings and update `/etc/upgrade` accordingly.
   - Document data sent (manifest request only). No device telemetry.
   - Show confirmation dialog the first time (“This will contact <host>. Proceed?”). Record timestamp of consent.

2. **Remote Analysis / Image Upload**
   - Provide UI/API for user to submit selected images/video clips to an analysis endpoint.
   - Require explicit agreement before first upload; show destination URL and privacy policy.
   - Each upload session asks for confirmation unless the user explicitly “remembers” the choice.
   - Support multiple modes:
     - **Manual Upload**: user selects timeframe/clip; device packages and posts to configured server.
     - **Scheduled Upload** (optional): user-defined triggers (e.g., high-risk detection) that queue for remote review.
   - Ensure data encrypted in transit (HTTPS/TLS). Provide local copy for audit trail.
   - Data minimization: anonymize on device (strip EXIF, redact faces/plates where not required). No user PII collected or transmitted.
   - Retention: server‑side 24 hours maximum (see `REMOTE_ANALYSIS.md`).

3. **Remote Support / Diagnostics** (optional future feature)
   - Allow user to enable remote support session, generating temporary access token for support team.
   - Disabled by default.
4. **Secure Remote Connectivity (self-hosted)**
   - Optional WireGuard/WebRTC-based tunnel (see `spec/REMOTE_ACCESS_STRATEGY.md`).
   - Requires user-provided relay/coordination server; feature disabled until user supplies details and consents.
   - Show clear warning: enabling remote access exposes device to external networks—review hardening guide before activating.

## Configuration Storage
- Store user choices in `/userdata/config/system.json` (example). Snapshot during backup/OTA.
- Provide CLI tool (`authority-config`) to view/set toggles for scripts and automation.
- Document in UI help: how to disable/enable features and wipe data.

Suggested keys:
```json
{
  "privacy": {
    "remote_analysis_enabled": false,
    "otp_export_enabled": false,
    "otp_export_max_mb": 10
  }
}
```

## Network Controls
- Use IPTables/firewall rules to block outbound traffic unless required by enabled features.
- Provide command`status` screen showing active endpoints and last contact time.
- Logging: keep record of external requests (`/var/log/authority-alert/telemetry.log`).

## UX Considerations
 - OOBE wizard includes privacy step:
   - Step prompts user to opt into updates and analysis—checkboxes start unchecked and require explicit confirmation.
  - Provide short summary + link to detail page.
- Settings page lets user change decision anytime.
- “Call home” state displayed on status screen.

## Implementation Tasks
- Modify `/mnt/system/upgrade.sh` auto download script to respect settings flag (only run if opt-in).
- Add config management layer (Node-RED flow or backend service) controlling remote upload endpoints.
- Implement upload workflow (UI + backend) with throttle/queue controls.
- Ensure Node-RED flows or other services do not post data externally without checking opt-in state.
- Update documentation to cover privacy policy and user controls.

## Validation
- Network sniff test: confirm no outbound connections until toggled.
- Consent logs: verify opt-in stored and can be exported for compliance (optional).
- Upload test: simulate manual upload, confirm encryption, server response, local logs.

## Open Questions
- Evaluate third‑party analysis providers against DPA and anonymization requirements.
- Confirm UX copy for consent dialogs referencing 24‑hour retention.
- Align consent and logging with broadest applicable privacy standards.
5. **OTP Export (Manual, Small Payloads)**
   - Disabled by default. When enabled, exposes a manual export flow using a physical, removable one-time pad (see `ONE_TIME_PAD.md`).
   - Requirements: pad media inserted; size cap (default 10 MB); no PII in payload; local minimal receipt only.
   - Each export consumes unique pad bytes; device refuses reuse or out-of-order access. No pads stored on device.
