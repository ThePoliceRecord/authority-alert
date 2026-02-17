# OOBE (Out-of-Box Experience) — Product Spec (SG200X reCamera)

Owner: TBD  
Last updated: 2026-01-02

## Summary
OOBE is a first-boot wizard that guides a new device from “factory / reset state” to “secure + online + ready to use” with opt-in privacy defaults. It should be fast, clear, and safe; it must not require prior knowledge of networking or Linux.

This spec defines **what** OOBE must do (user experience + acceptance criteria). Implementation details live in `docs/oobe_programming_spec.md`.

## Goals
- Get the camera online (Wi‑Fi STA) from AP mode, with a safe fallback.
- Collect explicit privacy/consent choices before any cloud/remote features activate.
- Set an admin password (or confirm existing) for the device UI and privileged actions.
- Provide a review/confirm step and an unambiguous completion state.
- Store results in a single durable config file and mark OOBE complete.

## Non-goals (v1)
- Captive portal implementation (optional later; provide clear URL instructions instead).
- Full account federation (OAuth2/SSO) unless already supported elsewhere.
- Multi-language localization (design for it, but English-only for v1).
- Deep device customization (advanced settings belong in the main UI).

## Triggering & State
OOBE must run when either is true:
- `/userdata/config/system.json` does not exist, or
- `oobe_complete` is `false`.

OOBE must be skipped when:
- `oobe_complete` is `true` and the user has not requested a reset.

OOBE completion is a durable state change (persisted to `/userdata/config/system.json`).

## User-facing Flow (v1)
The wizard is linear with back navigation and a step indicator.

### Step 1 — Welcome
- Explains what the device does and what setup will cover.
- Primary: “Get started”
- Secondary: “Learn more” (link to local docs page; no external dependency).

### Step 2 — Privacy & Consent (opt-in defaults)
All toggles default to **OFF**.
- Automatic update checks (shows destination base URL when enabled).
- Remote analysis uploads (only when explicitly initiated later).
- Anonymous diagnostics (optional; can be stubbed in v1).

Copy requirements:
- Must state “no cloud contact until you opt in” (or equivalent).
- Must clearly state what data could leave the device for each toggle.

### Step 3 — Network Setup
Primary outcome: connect to Wi‑Fi (STA).

UI requirements:
- Show a list of available SSIDs (scan) with signal + security indicator.
- Manual entry option for hidden SSIDs.
- WPA2/WPA3 passphrase input with show/hide toggle.

Fallback requirements:
- Option: “Skip (stay in Access Point mode)” with warning.
- If connect attempt fails, show reason and allow retry without losing entered data.

Post-conditions:
- Persist chosen network config.
- System performs the minimum disruption needed (service restart or reboot) and clearly communicates what will happen.

### Step 4 — Security / Admin Password
Requirements:
- Prompt to set admin password for the device UI (and any privileged endpoints).
- Password strength guidance (length-based, no unrealistic rules).
- Option to disable any advanced/admin surfaces by default (e.g., Node-RED editor if present).

### Step 5 — Review & Confirm
- Show a summary of:
  - Consent toggles
  - Network choice (SSID or “AP mode”)
  - Security choices
- Primary: “Finish setup”
- Secondary: “Back”

### Step 6 — Completion
- States setup is complete and how to reach the main dashboard.
- Provides next-step links:
  - “Open dashboard”
  - “Change settings later”
  - “View documentation”

## Data & Privacy Requirements
- No outbound network calls beyond what is necessary for Wi‑Fi association unless the user opted in.
- Consent toggles must be logged locally with timestamp (no PII) and persisted.
- Credentials (Wi‑Fi passphrase, admin password):
  - Must never be written to logs.
  - Must be stored in the proper system location (Wi‑Fi) or as a hash (password).

## Accessibility & UX Requirements
- Keyboard navigable (tab order, visible focus).
- High-contrast readable text; avoid reliance on color alone.
- Clear error messages with actionable guidance.
- Loading states for scanning networks and applying changes.

## Reliability Requirements
- OOBE should tolerate partial failures:
  - If Wi‑Fi connect fails, OOBE remains accessible in AP mode.
  - If a reboot is required, OOBE resumes and reflects saved progress.
- If config persistence fails, user must see a blocking error and retry option.

## Acceptance Criteria (v1)
- Fresh device (no config) shows OOBE within 60 seconds of boot.
- Completing OOBE sets `oobe_complete=true` and the wizard does not reappear on next boot.
- User can skip network setup and remain on AP; device UI remains reachable.
- Enabling/disabling consent toggles is reflected in `/userdata/config/system.json` and in the UI review step.
- Admin password is required before enabling privileged actions; stored as a hash (no plaintext).
- Failures (scan, connect, save) are shown as user-friendly errors; no silent failures.

## Open Questions
- Do we require a captive portal, or is “connect to AP and browse to a URL” sufficient for v1?
- Which existing service owns Wi‑Fi scanning/apply (Supervisor vs new OOBE backend)?
- What is the canonical place to store “device admin” credentials (existing Supervisor auth vs separate)?

