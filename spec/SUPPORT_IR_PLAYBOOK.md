# Support & Incident Response Playbook

Owner: YOU
Last updated: YYYY-MM-DD

## Purpose
Provide structured response steps for operational issues, security incidents, and customer support interactions.

## Contact Channels
- Support email / ticket portal (TBD).
- Emergency hotline for critical deployments (optional).
- Internal Slack/Teams channel for on-call engineers.

## Severity Levels
| Level | Description | Response Time |
|-------|-------------|---------------|
| Sev 1 | System outage, security breach, safety risk | Immediate (<15 min) |
| Sev 2 | Major feature failure, affects operations | 1 hour |
| Sev 3 | Minor bug, workaround available | 1 business day |
| Sev 4 | Feature request / info | 3 business days |

## Incident Response Steps
1. **Report Intake**
   - Log incident in ticket system with timestamp, reporter, severity.
   - Collect logs using `authority-support collect-logs`.
2. **Triage**
   - On-call engineer validates severity, gathers additional context (device ID, firmware version).
   - Classify as security vs functional issue.
3. **Containment** (if security incident)
   - Isolate affected device/network (disconnect from external network, disable uploads).
   - Change credentials if compromised.
4. **Analysis**
   - Review logs, version audit, Node-RED flows, network traces.
   - Determine root cause; confirm if other devices affected.
5. **Remediation**
   - Apply patch or configuration change; prepare OTA fix if needed.
   - Document steps for customer; provide instructions or remote assistance.
6. **Recovery**
   - Verify system functionality; restore services.
   - Monitor device for stability.
7. **Post-Incident Review**
   - Summarize timeline, root cause, remediation, preventive actions.
   - Update documentation, add tests to prevent recurrence.

## Communication
- Keep customer informed at agreed intervals.
- For security incidents, provide preliminary update within 24 hours with known information.
- Escalate to management/legal if data exposure suspected.

## Preventive Measures
- Run scheduled health checks (status API) to catch issues early.
- Maintain vulnerability disclosure policy; monitor CVEs affecting components.
- Keep runbooks updated, including release checklist and hardening guide.

## Data Requests
- Provide procedure for agencies requesting footage/logs (legal compliance).
- Verify authorization before sharing data.

## Tools & Scripts
- `authority-support collect-logs` – bundles logs/config for analysis.
- `authority-config` – view/update device settings.
- OTA rollback script for emergency downgrades.

