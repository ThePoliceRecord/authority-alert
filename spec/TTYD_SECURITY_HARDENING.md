# ttyd Web Terminal - Security Hardening Guide

## Overview

This document outlines security measures for the ttyd web-based terminal service before customer distribution.

## Security Risks

**HIGH RISK:** ttyd provides full shell access through a web browser. Without proper hardening:
- Unauthenticated access to root shell
- Network-wide exposure on port 9090
- No encryption (HTTP)
- Potential for remote code execution
- Session hijacking

## Recommended Security Measures

### Option 1: DISABLE ttyd for Production (RECOMMENDED)

**Most secure approach for customer-facing devices.**

```bash
# In buildroot config:
# BR2_PACKAGE_TTYD is not set

# Or disable service on device:
systemctl disable ttyd  # if using systemd
rm /etc/init.d/S98ttyd  # if using sysvinit
```

**Rationale:** Customers should NOT have shell access to production devices. Use proper device management APIs instead.

---

### Option 2: Enable Only for Development Builds

Use conditional compilation:

```makefile
# In build.sh or Makefile:
ifeq ($(BUILD_TYPE),development)
    BR2_PACKAGE_TTYD=y
else
    # BR2_PACKAGE_TTYD is not set
endif
```

---

### Option 3: Maximum Hardening (if ttyd MUST be enabled)

If ttyd is required for customer support/debugging, apply ALL of the following:

#### 3.1 Require Strong Authentication

Edit `/etc/ttyd.conf`:
```bash
# Use strong randomly generated credentials
TTYD_CREDENTIAL="$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 32):$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 32)"
```

#### 3.2 Bind to Localhost Only

```bash
# In S98ttyd, change port binding:
TTYD_INTERFACE="127.0.0.1"
set -- "$@" -i "$TTYD_INTERFACE"
```

Require SSH tunnel for access:
```bash
ssh -L 9090:localhost:9090 user@recamera-ip
# Then access http://localhost:9090
```

#### 3.3 Enable Read-Only Mode by Default

```bash
TTYD_READONLY=true
```

Only allow viewing, no command execution.

#### 3.4 Use SSL/TLS

```bash
# Generate self-signed certificate:
openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout /etc/ttyd.key -out /etc/ttyd.crt \
    -days 3650 -subj "/CN=recamera"

# In /etc/ttyd.conf:
TTYD_SSL_CERT="/etc/ttyd.crt"
TTYD_SSL_KEY="/etc/ttyd.key"

# In S98ttyd:
[ -n "$TTYD_SSL_CERT" ] && [ -n "$TTYD_SSL_KEY" ] && \
    set -- "$@" -S -C "$TTYD_SSL_CERT" -K "$TTYD_SSL_KEY"
```

Access via: `https://<recamera-ip>:9090`

#### 3.5 Limit to Single Client

```bash
TTYD_MAX_CLIENTS=1
```

Prevents multiple simultaneous access.

#### 3.6 Set Client Idle Timeout

```bash
# In S98ttyd:
TTYD_TIMEOUT=300  # 5 minutes
set -- "$@" -t "$TTYD_TIMEOUT"
```

#### 3.7 Disable URL Writing

```bash
# In S98ttyd:
set -- "$@" --writable false
```

#### 3.8 Run as Unprivileged User

```bash
# Create dedicated user:
adduser --system --no-create-home --shell /bin/false ttyd-user

# In S98ttyd:
start-stop-daemon -S -q -b -m -p "$PID_FILE" \
    -c ttyd-user \  # Run as ttyd-user, not root
    --exec "$TTYD_BIN" -- "$@" "$TTYD_CMD"
```

#### 3.9 Firewall Rules

```bash
# iptables rules:
iptables -A INPUT -p tcp --dport 9090 -s 192.168.1.0/24 -j ACCEPT  # Allow only local network
iptables -A INPUT -p tcp --dport 9090 -j DROP  # Deny all other
```

#### 3.10 Audit Logging

```bash
# Log all commands to syslog:
TTYD_CMD="/bin/bash -c 'exec script -q -c /bin/login /dev/null | tee >(logger -t ttyd-session)'"
```

---

## Implementation Plan

### For Customer Distribution

**Phase 1: Immediate**
1. ✅ Disable ttyd in production builds
2. ✅ Document SSH access as primary method
3. ✅ Create separate dev/prod build configurations

**Phase 2: If ttyd Required**
1. Enable strong authentication
2. Bind to localhost only
3. Require SSH tunneling
4. Enable SSL/TLS
5. Set read-only mode
6. Implement firewall rules
7. Add audit logging

**Phase 3: Customer Documentation**
1. Provide SSH access guide
2. Document ttyd risks
3. Require customer acknowledgment
4. Include security disclosure

---

## Alternative: Secure Remote Management

Instead of ttyd, implement proper device management:

### Recommended: SSH-Only Access

```bash
# Secure SSH configuration (/etc/ssh/sshd_config):
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AllowUsers support-user
ClientAliveInterval 300
ClientAliveCountMax 2
```

### API-Based Management

Implement RESTful API for device control:
- Status monitoring
- Log retrieval
- Configuration updates
- OTA updates

No shell access required.

---

## Risk Assessment Matrix

| Configuration | Risk Level | Use Case |
|---------------|-----------|----------|
| ttyd disabled | **LOW** | Production devices (RECOMMENDED) |
| ttyd + localhost + SSH tunnel + auth | **MEDIUM** | Support/debugging only |
| ttyd + auth + firewall | **HIGH** | Development only |
| ttyd no auth | **CRITICAL** | NEVER USE |

---

## Compliance Notes

### GDPR/Privacy
- Terminal access may expose customer data
- Require explicit consent
- Log all access attempts
- Implement data retention policies

### Security Standards
- **CIS Benchmark:** Recommends disabling unnecessary services
- **NIST SP 800-123:** Avoid web-based admin interfaces
- **OWASP:** Web terminal = high-risk attack surface

---

## Decision Checklist

Before enabling ttyd for customers:

- [ ] Is shell access absolutely necessary?
- [ ] Can we use SSH + key-based auth instead?
- [ ] Can we use API-based management?
- [ ] Have we implemented ALL hardening measures?
- [ ] Have we documented security risks for customers?
- [ ] Have we obtained legal/compliance approval?
- [ ] Do we have incident response plan for compromise?
- [ ] Are we logging all access?
- [ ] Have we tested penetration scenarios?

**If ANY checkbox is unchecked, DO NOT enable ttyd for production.**

---

## Conclusion

**RECOMMENDATION:** Disable ttyd entirely for customer-facing production devices.

Use:
1. SSH with public key authentication
2. RESTful API for device management
3. Proper authentication/authorization
4. Audit logging
5. Firewall protection

ttyd should ONLY be enabled in:
- Development builds
- Internal testing
- Controlled lab environments
- With ALL security measures applied
