# Threat Model & Mitigations

Owner: YOU
Last updated: YYYY-MM-DD

## Assets
- Live and recorded video streams, AI detection events.
- User credentials, configuration files, OTA packages.
- Authority Alert analytics (integration with police records).
- Device integrity (firmware, boot chain, Node-RED flows).

## Adversaries
- Opportunistic attackers on local network.
- Malicious insiders with physical access.
- Remote attackers targeting exposed services.
- Supply chain adversaries tampering with firmware/OTA packages.

## Attack Surfaces & Controls
### Physical Access
- **Risks**: SD card removal, UART/JTAG access, power cycles.
- **Mitigations**: Tamper-resistant enclosure; disable debug ports; require authentication for console.

### Network Interfaces
- **Risks**: Unauthenticated HTTP endpoints, weak Wi-Fi, exposed SSH/Node-RED.
- **Mitigations**: HTTPS enforced; firewall restricts inbound/outbound; strong WPA2/WPA3; disable unused services; rate limiting.

### Firmware / OTA Updates
- **Risks**: Malicious OTA zip or MITM.
- **Mitigations**: Serve OTA over HTTPS; verify md5/sha signatures; future plan for signed packages; manual consent logs.

### Node-RED Flows & Custom Nodes
- **Risks**: Vulnerable third-party nodes, unauthorized flow changes.
- **Mitigations**: Lock flow deploy with admin password; freeze node versions; audit flows; disable palette manager.

### Remote Analysis Uploads
- **Risks**: Data exfiltration without consent, man-in-the-middle.
- **Mitigations**: Opt-in only; show destination; TLS; store local log of uploads; allow revoke.

### Integration APIs (Police Records)
- **Risks**: Credential theft, API abuse.
- **Mitigations**: Use per-agency API keys stored securely; rotate keys; log all API calls; support IP allow-lists; align with CJIS.

### Supply Chain
- **Risks**: Compromised build environment/OTA server.
- **Mitigations**: Reproducible builds via Docker; GitLab CI logs; restrict OTA server access; integrity checks; maintain SBOM (future).

## Residual Risks & Future Work
- Implement signed OTA packages and secure boot.
- Deploy automated vulnerability scanning for Node-RED nodes and system packages.
- Investigate hardware root of trust (TPM) for credential storage.
- Provide anomaly detection for camera events (alert on unusual login patterns).

