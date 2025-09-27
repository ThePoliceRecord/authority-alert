# Out-of-Box Experience (OOBE)

Owner: YOU
Last updated: YYYY-MM-DD

## Purpose
- Guide first-time setup with empathy and transparency, reminding the owner that Authority Alert uses AI to support them, not surveil them.
- Collect essential configuration (privacy consent, network, updates, remote analysis) using opt-in defaults.

## Trigger
- OOBE runs when the system detects no prior configuration (`/userdata/config/system.json` missing or flag `oobe_complete=false`).
- Once completed, flag stored; wizard skipped on subsequent boots unless manually reset.

## Flow Overview
1. **Welcome**
   - Headline: “Authority Alert – AI watching with you, not on you.”
   - Brief message: explains AI assists with situational awareness and respects privacy.
   - Button: “Get Started”.

2. **Privacy & Consent**
   - Explain opt-in philosophy (“No cloud contact until you approve it.”).
   - Checkboxes (all unchecked):
     - “Enable automatic update checks” (shows destination URL when set later).
     - “Allow remote analysis uploads when I approve them.”
     - “Share anonymous diagnostics to help improve reliability” (optional future feature).
   - Reminder text: “AI for you, not against you — you stay in control.”

3. **Network Setup**
   - If device is in AP mode, show available Wi-Fi networks (scan).
   - Form: SSID, password, optional static IP.
   - Option: “Skip and stay in Access Point mode” (warn about limited remote access).
   - Save config to `/etc/wpa_supplicant.conf` and mark for restart.

4. **Account & Security**
   - Create admin password for device UI + Node-RED editor (if enabling).
   - Option to disable Node-RED editor access (recommended for most users).
   - Provide guidance on password strength.
    - Optional OAuth2 sign-in (Auth0 or Keycloak) to link Police Record account if the organization provides credentials.

5. **Review & Confirm**
   - Show summary of choices (privacy toggles, network, security).
   - “Finish setup” button; triggers configuration save, restarts relevant services.
    - Optional section for remote connectivity: prompt to configure self-hosted relay now or skip for later.

6. **Completion Screen**
   - Message: “Setup complete. Authority Alert AI is on your side. You can change any setting anytime.”
   - Buttons: “Open Dashboard”, “Configure later”, “View documentation”.

## UX Notes
- Use palette from `spec/UI_THEME.md` and background image for visual consistency.
- Provide progress indicator (step 1–5) and option to go back.
- Include contextual help tooltips for each consent item.
- For accessibility: ensure keyboard navigation, descriptive text for screen readers.

## Technical Implementation
- OOBE SPA served from `/authority-alert/oobe/` (part of new UI bundle per `spec/UI_REDESIGN.md`).
- Backend endpoint `POST /api/oobe` handles config persistence.
- Config stored under `/userdata/config/system.json`, including:
  ```json
  {
    "oobe_complete": true,
    "consent": {
      "auto_update": true,
      "remote_upload": false,
      "diagnostics": false
    },
    "network": {
      "mode": "sta",
      "ssid": "...",
      "ip": "dhcp"
    },
    "security": {
      "admin_password_hash": "...",
      "node_red_editor_enabled": false
    }
  }
  ```
- Backend triggers necessary scripts (write wpa_supplicant, enable update scripts, set Node-RED admin auth).
- Logging: store timestamp and user consent in `/var/log/authority-alert/oobe.log`.

## Reminders / Notifications
- Status screen alerts if OOBE incomplete (rare cases like reset).
- Settings page includes “AI for you” banner summarizing consent choices with quick toggles.
- Periodic reminder (e.g., once per year) to review consent toggles, triggered via Node-RED flow or scheduled job.

## Reset Procedure
- Provide CLI (`authority-config reset-oobe`) and UI button (under advanced settings) to rerun wizard.
- Reset clears consent toggles and prompts run on next boot.

## Open Items
- Define authentication mechanism for OOBE API (initial session uses temporary token).
- Determine whether Node-RED flows automatically adjust based on consent (e.g., disable remote upload nodes).
- Plan translation/localization for messaging (if needed).
