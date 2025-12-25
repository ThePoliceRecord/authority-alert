# CVE Triage Priorities – 0.1.0

Owner: Authority Alert Team
Last updated: 2025-12-19

This triage is based on the NVD CPE match output in `spec/releases/0.1.0/security/out/NVD_SUMMARY.md`.

## Important Caveat
NVD CPE matching can over-report (a CVE may not be exploitable in your build, may require a feature you don’t enable, or may be fixed by downstream patches). Treat this as a starting point:
- Confirm actual on-device versions using `spec/version_audit.sh`.
- Confirm service exposure (is it listening? reachable from untrusted networks?).

## Highest Priority (Network-exposed)
1) `openssh` (remote access)
- If SSH is enabled on any untrusted network, prioritize updating OpenSSH and hardening configuration (keys-only, disable password auth, rate limiting).

2) `wpa_supplicant` / `hostapd` (Wi‑Fi stack)
- Wi‑Fi parsing issues can be remotely triggerable in some cases. Treat as high priority on devices that operate in AP/STA modes.
- If AP mode is not required, disable it and remove `hostapd`.

3) `mosquitto` (MQTT broker)
- If MQTT is local-only, restrict binding to localhost and firewall it.
- If exposed remotely for any reason, upgrade promptly and add authentication/TLS.

4) `libopenssl` (TLS / crypto)
- OpenSSL `1.1.1k` is old; prioritize upgrading to a patched line for any device using TLS (OTA over HTTPS, remote analysis, OAuth, etc.).

## High Priority (Untrusted input parsing)
5) `ffmpeg`
- If the device decodes untrusted media inputs (uploads, streams, remote clips), ffmpeg CVEs matter.
- If ffmpeg is not needed, removing it reduces attack surface.

6) `busybox`
- BusyBox CVEs can matter if applets are reachable through exposed services or used to parse attacker-controlled input.

## Medium Priority / Context-dependent
7) `python`
- Python is often used for tooling; risk depends on whether Python code runs with untrusted inputs or exposes network services.

8) `nodejs`
- If Node.js/Node-RED is being removed for the Go UI approach, this becomes lower priority once removed from the production image.

## Immediate Hardening Actions (Even Before Upgrades)
- Reduce exposed services: disable `ttyd`, disable Node-RED editor, restrict SSH, disable Avahi if not needed.
- Firewall: default-deny inbound; only allow required ports and interfaces.
- Bind MQTT to localhost unless remote clients are explicitly needed.

