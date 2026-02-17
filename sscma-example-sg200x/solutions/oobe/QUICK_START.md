# OOBE Quick Start & Troubleshooting

## Quick Diagnostic

If OOBE process is running but not listening on a port, run these commands on the device:

```bash
# 1. Check if process is running
ps aux | grep oobe

# 2. Check if listening on port
ss -tulnp | grep 8081
# OR
netstat -tulnp | grep 8081

# 3. Check certificates exist
ls -l /etc/supervisor/certs/

# 4. Stop service and run manually to see errors
/etc/init.d/S94oobe stop
/usr/local/bin/oobe --listen https://0.0.0.0:8081 --root /usr/share/oobe/www --cert /etc/supervisor/certs/cert.pem --key /etc/supervisor/certs/key.pem
```

## Common Issues

### Issue 1: Certificates Don't Exist

**Symptoms:**
- Process runs but doesn't listen
- No error messages visible

**Check:**
```bash
ls -l /etc/supervisor/certs/
```

**Fix:**
If certificates don't exist, either:

**Option A - Use HTTP instead (temporary):**
```bash
# Edit init script
vi /etc/init.d/S94oobe

# Change DAEMON_ARGS to:
DAEMON_ARGS="--listen http://0.0.0.0:8081 --root /usr/share/oobe/www"

# Restart
/etc/init.d/S94oobe restart
```

**Option B - Create certificates:**
```bash
# Create directory
mkdir -p /etc/supervisor/certs

# Generate self-signed certificate
openssl req -x509 -newkey rsa:4096 -keyout /etc/supervisor/certs/key.pem \
  -out /etc/supervisor/certs/cert.pem -days 365 -nodes \
  -subj "/CN=authority-alert"

# Set permissions
chmod 600 /etc/supervisor/certs/key.pem
chmod 644 /etc/supervisor/certs/cert.pem
```

### Issue 2: Mongoose TLS Not Compiled

**Symptoms:**
- Certificates exist
- Process runs but doesn't listen
- Manual run shows no errors

**Check:**
```bash
# Run with HTTP to test
/usr/local/bin/oobe --listen http://0.0.0.0:8081 --root /usr/share/oobe/www
```

If HTTP works, Mongoose was compiled without TLS support.

**Fix:**
Use HTTP mode (see Option A above) or recompile Mongoose with TLS:
```bash
# In mongoose component, ensure MG_ENABLE_OPENSSL or MG_ENABLE_MBEDTLS is defined
```

### Issue 3: Port Already in Use

**Check:**
```bash
ss -tulnp | grep 8081
lsof -i :8081
```

**Fix:**
```bash
# Kill process using port
kill <pid>

# Or use different port
/usr/local/bin/oobe --listen https://0.0.0.0:9000 --root /usr/share/oobe/www
```

### Issue 4: Permission Denied

**Symptoms:**
- Error: "Failed to listen on https://0.0.0.0:8081"

**Fix:**
```bash
# Run as root
sudo /usr/local/bin/oobe --listen https://0.0.0.0:8081 --root /usr/share/oobe/www
```

## Testing Steps

### 1. Test with HTTP First

```bash
# Stop service
/etc/init.d/S94oobe stop

# Run with HTTP
/usr/local/bin/oobe --listen http://0.0.0.0:8081 --root /usr/share/oobe/www

# In another terminal, check if listening
ss -tulnp | grep 8081

# Test from browser
curl http://localhost:8081/api/health
```

Expected output:
```json
{"ok":true,"service":"oobe"}
```

### 2. Test with HTTPS

```bash
# Ensure certificates exist
ls -l /etc/supervisor/certs/cert.pem /etc/supervisor/certs/key.pem

# Run with HTTPS
/usr/local/bin/oobe --listen https://0.0.0.0:8081 \
  --root /usr/share/oobe/www \
  --cert /etc/supervisor/certs/cert.pem \
  --key /etc/supervisor/certs/key.pem

# Check if listening
ss -tulnp | grep 8081

# Test from browser (ignore cert warning)
curl -k https://localhost:8081/api/health
```

### 3. Check Logs

```bash
# System logs
logread | grep oobe

# Or if using syslog
tail -f /var/log/messages | grep oobe

# Run in foreground to see output
/usr/local/bin/oobe --listen https://0.0.0.0:8081 --root /usr/share/oobe/www
```

## Recommended Configuration

### For Production (with supervisor)

Use HTTPS with shared certificates:
```bash
DAEMON_ARGS="--listen https://0.0.0.0:8081 --root /usr/share/oobe/www --cert /etc/supervisor/certs/cert.pem --key /etc/supervisor/certs/key.pem"
```

### For Development/Testing

Use HTTP for simplicity:
```bash
DAEMON_ARGS="--listen http://0.0.0.0:8081 --root /usr/share/oobe/www"
```

### For Different Port

```bash
DAEMON_ARGS="--listen https://0.0.0.0:9000 --root /usr/share/oobe/www"
```

## Accessing OOBE

### With HTTPS
```
https://<device-ip>:8081
```

### With HTTP
```
http://<device-ip>:8081
```

### Health Check
```bash
# HTTPS
curl -k https://localhost:8081/api/health

# HTTP
curl http://localhost:8081/api/health
```

Expected response:
```json
{"ok":true,"service":"oobe"}
```

## Integration with Supervisor

### Check Supervisor Status

```bash
# Is supervisor running?
ps aux | grep supervisor

# Is it listening?
ss -tulnp | grep 443

# Test API
curl -k https://localhost/api/version
```

### Verify Certificates Match

```bash
# Check supervisor certs
ls -l /etc/supervisor/certs/

# Check OOBE is using same certs
ps aux | grep oobe
# Look for --cert and --key arguments
```

## Still Having Issues?

1. **Check file permissions:**
   ```bash
   ls -la /usr/local/bin/oobe
   ls -la /usr/share/oobe/www/
   ls -la /etc/supervisor/certs/
   ```

2. **Verify web files exist:**
   ```bash
   ls -la /usr/share/oobe/www/
   ls -la /usr/share/oobe/www/js/
   ls -la /usr/share/oobe/www/css/
   ```

3. **Check system resources:**
   ```bash
   free -m
   df -h
   ```

4. **Try minimal test:**
   ```bash
   # Just serve files, no TLS
   cd /usr/share/oobe/www
   python3 -m http.server 8081
   ```

5. **Review full logs:**
   ```bash
   dmesg | tail -50
   logread | tail -50
   ```

## Getting Help

If still not working, gather this information:

```bash
# System info
uname -a
cat /etc/os-release

# OOBE status
ps aux | grep oobe
ss -tulnp | grep 8081
ls -l /usr/local/bin/oobe
ls -l /etc/supervisor/certs/

# Supervisor status
ps aux | grep supervisor
ss -tulnp | grep 443
curl -k https://localhost/api/version

# Logs
logread | grep -E '(oobe|supervisor)' | tail -20
```

Then refer to:
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Detailed troubleshooting
- [INSTALL.md](INSTALL.md) - Installation guide
- [README.md](README.md) - General documentation
