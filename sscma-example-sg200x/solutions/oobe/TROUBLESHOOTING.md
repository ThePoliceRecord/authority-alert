# OOBE Troubleshooting Guide

## Common Issues and Solutions

### Issue: "Cannot connect to supervisor"

**Symptoms:**
- OOBE shows error: "Cannot connect to supervisor"
- Browser console shows network errors

**Possible Causes & Solutions:**

#### 1. Supervisor Not Running

**Check:**
```bash
ps | grep supervisor
netstat -tlnp | grep 443
```

**Fix:**
```bash
/etc/init.d/S93sscma-supervisor start
```

#### 2. Mixed Content (HTTP/HTTPS) Issue

**Problem:** OOBE runs on HTTP (port 8081) but supervisor uses HTTPS (port 443). Modern browsers block HTTP pages from accessing HTTPS resources.

**Solution A - Access OOBE via HTTPS:**
If you have a reverse proxy or can access the device via HTTPS, use:
```
https://<device-ip>:8081
```

**Solution B - Browser Settings:**
Allow insecure content for this site:
- Chrome: Click the shield icon in address bar → "Load unsafe scripts"
- Firefox: Click the lock icon → "Disable protection for now"
- Safari: Develop menu → "Disable Cross-Origin Restrictions"

**Solution C - Use Same-Origin:**
Access OOBE through the supervisor's domain:
```
https://<device-ip>/oobe/
```
(Requires supervisor to proxy OOBE)

#### 3. CORS (Cross-Origin Resource Sharing) Issue

**Problem:** Browser blocks requests from OOBE (port 8081) to supervisor (port 443).

**Check Browser Console:**
Look for errors like:
```
Access to fetch at 'https://...' from origin 'http://...' has been blocked by CORS policy
```

**Fix:** The supervisor should already have CORS middleware enabled. Verify in [`server.go`](../supervisor/internal/server/server.go):
```go
middleware.CORS,
```

If not present, the supervisor needs to be updated to allow CORS from OOBE origin.

#### 4. Self-Signed Certificate Issue

**Problem:** Browser rejects supervisor's self-signed HTTPS certificate.

**Symptoms:**
- Browser console shows: `net::ERR_CERT_AUTHORITY_INVALID`
- Fetch fails with certificate error

**Fix:**
1. First, visit the supervisor directly: `https://<device-ip>`
2. Accept the certificate warning
3. Then return to OOBE: `http://<device-ip>:8081`

**Alternative - Add Certificate Exception:**
```bash
# On device, get certificate
openssl s_client -connect localhost:443 -showcerts

# Import to browser's trusted certificates
```

#### 5. Firewall Blocking

**Check:**
```bash
iptables -L -n | grep 443
```

**Fix:**
```bash
# Allow HTTPS traffic
iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```

#### 6. Wrong Hostname/IP

**Problem:** OOBE is trying to connect to wrong address.

**Check Browser Console:**
Look for the URL being accessed. The OOBE auto-detects based on how you access it:
- If you access OOBE at `http://192.168.1.100:8081`, it tries supervisor at `https://192.168.1.100`
- If you access OOBE at `http://localhost:8081`, it tries supervisor at `https://localhost`

**Fix:**
Access OOBE using the correct IP/hostname that matches where supervisor is running.

---

### Issue: OOBE Loads But Shows Blank Page

**Possible Causes:**

#### 1. JavaScript Errors

**Check Browser Console:**
Press F12 and look for JavaScript errors.

**Common Errors:**
- `SupervisorAPI is not defined` → [`supervisor-api.js`](web/js/supervisor-api.js) not loaded
- `oobeApp is not defined` → [`oobe-app.js`](web/js/oobe-app.js) not loaded

**Fix:**
Verify all files are present:
```bash
ls -la /usr/share/oobe/www/
ls -la /usr/share/oobe/www/js/
ls -la /usr/share/oobe/www/css/
```

#### 2. Missing CSS

**Check:**
View page source and verify CSS link is correct:
```html
<link rel="stylesheet" href="/css/style.css" />
```

**Fix:**
```bash
chmod 644 /usr/share/oobe/www/css/style.css
```

---

### Issue: WiFi Scan Shows No Networks

**Possible Causes:**

#### 1. WiFi Adapter Not Present

**Check:**
```bash
ifconfig -a | grep wlan
iw dev
```

**Fix:**
Ensure WiFi hardware is present and drivers loaded:
```bash
lsmod | grep wifi
modprobe <wifi_driver>
```

#### 2. WiFi Disabled

**Check:**
```bash
rfkill list
```

**Fix:**
```bash
rfkill unblock wifi
```

#### 3. Supervisor WiFi API Not Working

**Test Directly:**
```bash
curl -k https://localhost/api/wifiMgr/getWiFiInfoList \
  -H "Authorization: Bearer <token>"
```

---

### Issue: Password Change Fails

**Symptoms:**
- Error: "Failed to set password"
- Login fails after password change

**Possible Causes:**

#### 1. Wrong Old Password

**Fix:**
Default password is usually `admin`. Try:
- Old password: `admin`
- Or check device documentation

#### 2. Password Too Weak

**Requirements:**
- Minimum 8 characters
- Must match confirmation

#### 3. Supervisor Auth Issue

**Test:**
```bash
curl -k -X POST https://localhost/api/userMgr/updatePassword \
  -H "Content-Type: application/json" \
  -d '{"oldPassword":"admin","newPassword":"newpass123"}'
```

---

### Issue: Device Name/Timezone Not Saving

**Check Supervisor Logs:**
```bash
logread | grep supervisor
```

**Test API Directly:**
```bash
# Set device name
curl -k -X POST https://localhost/api/deviceMgr/updateDeviceName \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"deviceName":"Test Camera"}'

# Set timezone
curl -k -X POST https://localhost/api/deviceMgr/setTimezone \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"timezone":"America/Chicago"}'
```

---

### Issue: WiFi Connection Fails

**Symptoms:**
- "Failed to connect to WiFi"
- Connection times out

**Possible Causes:**

#### 1. Wrong Password

**Fix:**
Double-check WiFi password, including:
- Case sensitivity
- Special characters
- Spaces

#### 2. Unsupported Security Type

**Check:**
OOBE supports: WPA2, WPA, WEP, Open

**Fix:**
If network uses WPA3 or other advanced security, it may not be supported.

#### 3. Signal Too Weak

**Check Signal Strength:**
Look at the signal indicator in the WiFi list.

**Fix:**
Move device closer to router or use Ethernet.

#### 4. Network Configuration Issue

**Check:**
```bash
iw dev wlan0 scan | grep -A 10 "SSID: YourNetwork"
```

**Manual Test:**
```bash
wpa_passphrase "YourSSID" "YourPassword" > /tmp/wpa.conf
wpa_supplicant -i wlan0 -c /tmp/wpa.conf
```

---

## Debugging Tips

### Enable Verbose Logging

**OOBE Server:**
```bash
# Run manually with debug output
/usr/local/bin/oobe --listen http://0.0.0.0:8081 --root /usr/share/oobe/www
```

**Supervisor:**
Check supervisor logs:
```bash
logread -f | grep supervisor
```

### Browser Developer Tools

**Open Console (F12):**
- Check for JavaScript errors
- Monitor network requests
- View API responses

**Network Tab:**
- See all API calls
- Check response codes
- View request/response bodies

### Test APIs Manually

**Get Version (No Auth):**
```bash
curl -k https://localhost/api/version
```

**Login:**
```bash
curl -k -X POST https://localhost/api/userMgr/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
```

**Use Token:**
```bash
TOKEN="<token-from-login>"
curl -k https://localhost/api/deviceMgr/getDeviceInfo \
  -H "Authorization: Bearer $TOKEN"
```

---

## Quick Diagnostic Script

Save this as `oobe-diag.sh` and run on the device:

```bash
#!/bin/sh

echo "=== OOBE Diagnostic ==="
echo

echo "1. OOBE Service Status:"
ps | grep oobe || echo "  NOT RUNNING"
echo

echo "2. Supervisor Service Status:"
ps | grep supervisor || echo "  NOT RUNNING"
echo

echo "3. Port Status:"
netstat -tlnp | grep -E '(8081|443|80)'
echo

echo "4. OOBE Files:"
ls -la /usr/local/bin/oobe 2>/dev/null || echo "  Binary not found"
ls -la /usr/share/oobe/www/ 2>/dev/null || echo "  Web files not found"
echo

echo "5. Supervisor API Test:"
curl -k -s https://localhost/api/version || echo "  Cannot connect"
echo

echo "6. WiFi Status:"
ifconfig wlan0 2>/dev/null || echo "  No wlan0 interface"
echo

echo "7. System Logs (last 10 lines):"
logread | grep -E '(oobe|supervisor)' | tail -10
```

Run it:
```bash
chmod +x oobe-diag.sh
./oobe-diag.sh
```

---

## Getting Help

If you're still having issues:

1. Run the diagnostic script above
2. Check browser console for errors (F12)
3. Review supervisor logs: `logread | grep supervisor`
4. Test supervisor API directly with curl
5. Verify network connectivity between OOBE and supervisor

For more information:
- [README.md](README.md) - General documentation
- [INSTALL.md](INSTALL.md) - Installation guide
- [SUPERVISOR_API_REFERENCE.md](SUPERVISOR_API_REFERENCE.md) - API documentation
