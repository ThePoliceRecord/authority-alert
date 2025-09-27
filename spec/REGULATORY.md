# Regulatory & Compliance Overview (US Deployment)

Owner: YOU
Last updated: YYYY-MM-DD

## Scope
- Authority Alert cameras intended for deployment within United States jurisdiction.
- Focus on relevant US laws, standards, and best practices for security/AI camera products.

## Key Compliance Domains
1. **Data Privacy & Collection**
   - Federal-level guidance (no omnibus law) but align with best practices:
     - FTC enforcement (truth in advertising, unfair practices).
     - State privacy laws (e.g., CCPA/CPRA in California, CPA in Colorado) if images/PII stored.
   - Document data retention, user consent (covered in `spec/PRIVACY_AND_REMOTE_ACCESS.md` and `spec/OOBE_FLOW.md`).
   - Provide mechanism for data export/deletion on request.

2. **CJIS (Criminal Justice Information Services)**
   - If integrating with police records/CAD, ensure software & infrastructure meets CJIS policies:
     - Authentication (multi-factor, password complexity, role-based access).
     - Encryption in transit (TLS 1.2+).
     - Audit logs for data access.
     - Physical/logical access controls for servers storing CJIS data.
   - Work with agency to host analytics/upload endpoints in CJIS-compliant environment.

3. **FCC / RF Compliance**
   - Hardware must comply with FCC Part 15 for Wi-Fi/Bluetooth transmissions.
   - Keep documentation/cert IDs with deployment kit.

4. **Export Controls (EAR/ITAR)**
   - Product restricted to US soil per business decision. Document non-export status and ensure build servers do not ship overseas.
   - AI/video analytics likely EAR99 but confirm if any components flagged.

5. **Cybersecurity Standards**
   - Reference NIST guidelines (NIST SP 800-53, NIST Cybersecurity Framework) where feasible.
   - Consider aligning with UL 2900 or OWASP IoT Top 10 recommendations.

6. **Accessibility**
   - Follow Section 508 / WCAG requirements for UI (per `spec/UI_REDESIGN.md`).

## Documentation & Processes
- Include Privacy Policy and Terms targeted at US users.
- Provide Data Processing Agreement for agencies if needed.
- Maintain logs for OTA updates, security patches (helpful for audit).
- Ensure user manuals include warnings about lawful usage (audio/video recording laws vary by state).

## Next Steps
- Audit hardware certifications (FCC, UL) and store copies in repo under `/docs/compliance/`.
- Map out CJIS checklist for integrations with police record systems.
- Draft End User License Agreement (EULA) / Terms referencing US-only deployment.
- Engage legal counsel to verify coverage of state-specific requirements.

