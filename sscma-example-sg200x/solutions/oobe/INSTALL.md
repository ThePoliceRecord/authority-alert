# OOBE Installation Guide

This guide explains how to install the OOBE (Out-of-Box Experience) service on your Authority Alert device.

## Prerequisites

- Built OOBE binary (from `make build-riscv`)
- Access to the device (SSH or serial console)
- Root/sudo access on the device

## Quick Start (Temporary Workaround)

If you just want to test the OOBE quickly without full installation:

### 1. Copy Files to Device

```bash
# Copy binary
scp build/oobe root@<device-ip>:/tmp/

# Copy web files
scp -r web root@<device-ip>:/tmp/oobe-web/
```

### 2. Run Manually

```bash
# SSH into device
ssh root@<device-ip>

# Run OOBE with HTTP (TLS not compiled in Mongoose)
/tmp/oobe --listen http://0.0.0.0:8081 --root /tmp/oobe-web
```

### 3. Access OOBE

Open browser and go to:
```
http://<device-ip>:8081
```

### 4. Browser Setup for Mixed Content

Since OOBE runs on HTTP and supervisor on HTTPS:

1. First visit `https://<device-ip>` and accept the certificate
2. Then go to `http://<device-ip>:8081`
3. Allow mixed content in browser:
   - **Chrome**: Click shield icon → "Load unsafe scripts"
   - **Firefox**: Click lock icon → "Disable protection"

### 5. Login Credentials

- **Username**: `recamera` (not `admin`)
- **Password**: Your SSH password

---

## Installation Methods

### Method 1: Using opkg Package (Recommended)

This is the cleanest method and handles all installation steps automatically.

#### Step 1: Build the Package

```bash
cd sscma-example-sg200x/solutions/oobe
make opkg
```

This creates: `build/oobe_0.1.0_riscv64.ipk`

#### Step 2: Transfer to Device

```bash
# Using SCP
scp build/oobe_0.1.0_riscv64.ipk root@<device-ip>:/tmp/

# Or using a USB drive, SD card, etc.
```

#### Step 3: Install on Device

```bash
# SSH into the device
ssh root@<device-ip>

# Install the package
opkg install /tmp/oobe_0.1.0_riscv64.ipk

# The service will start automatically
```

#### Step 4: Verify Installation

```bash
# Check if service is running
ps | grep oobe

# Check if listening on port 8081
netstat -tlnp | grep 8081

# Test the health endpoint
curl http://localhost:8081/api/health
```

Expected output:
```json
{"ok":true,"service":"oobe"}
```

---

### Method 2: Manual Installation

If you don't have opkg or prefer manual installation:

#### Step 1: Build the Binary

```bash
cd sscma-example-sg200x/solutions/oobe
make build-riscv
```

This creates: `build/oobe`

#### Step 2: Transfer Files to Device

```bash
# Transfer binary
scp build/oobe root@<device-ip>:/usr/local/bin/

# Transfer web files
scp -r web/* root@<device-ip>:/usr/share/oobe/www/

# Transfer init script
scp rootfs/etc/init.d/S94oobe root@<device-ip>:/etc/init.d/
```

#### Step 3: Set Permissions

```bash
# SSH into the device
ssh root@<device-ip>

# Make binary executable
chmod 755 /usr/local/bin/oobe

# Make init script executable
chmod 755 /etc/init.d/S94oobe

# Create web directory if it doesn't exist
mkdir -p /usr/share/oobe/www
```

#### Step 4: Update Init Script for HTTP

Since Mongoose is compiled without TLS support, edit the init script:

```bash
vi /etc/init.d/S94oobe
```

Change line 5 to use HTTP:
```bash
DAEMON_ARGS="--listen http://0.0.0.0:8081 --root /usr/share/oobe/www"
```

#### Step 5: Start the Service

```bash
# Start the service
/etc/init.d/S94oobe start

# Verify it's running
ps | grep oobe

# Verify it's listening
ss -tulnp | grep 8081
```

---

### Method 3: Integration into Firmware Build

To include OOBE in your firmware image:

#### Step 1: Add to Buildroot Configuration

Add the OOBE package to your Buildroot external tree or overlay.

#### Step 2: Build with Firmware

The OOBE will be included in the firmware image and installed automatically on first boot.

---

## Post-Installation

### Access the OOBE Interface

1. Connect to the device's network
2. Open a web browser
3. Navigate to: `http://<device-ip>:8081`

### Service Management

```bash
# Start service
/etc/init.d/S94oobe start

# Stop service
/etc/init.d/S94oobe stop

# Restart service
/etc/init.d/S94oobe restart

# Check status
ps | grep oobe
```

### Logs

```bash
# View system logs
logread | grep oobe

# Or if using syslog
tail -f /var/log/messages | grep oobe
```

---

## Troubleshooting

### Service Won't Start

**Check if binary exists:**
```bash
ls -l /usr/local/bin/oobe
```

**Check if web files exist:**
```bash
ls -l /usr/share/oobe/www/
```

**Try running manually:**
```bash
/usr/local/bin/oobe --listen http://0.0.0.0:8081 --root /usr/share/oobe/www
```

### Port Already in Use

```bash
# Check what's using port 8081
netstat -tlnp | grep 8081

# Kill the process if needed
kill <pid>
```

### Cannot Connect to Supervisor

**Verify supervisor is running:**
```bash
ps | grep supervisor
netstat -tlnp | grep 443
```

**Test supervisor API:**
```bash
curl -k https://localhost/api/version
```

### Web Files Not Loading

**Check file permissions:**
```bash
ls -la /usr/share/oobe/www/
```

**Ensure all files are readable:**
```bash
chmod -R 644 /usr/share/oobe/www/*
chmod 755 /usr/share/oobe/www
chmod 755 /usr/share/oobe/www/css
chmod 755 /usr/share/oobe/www/js
```

---

## Uninstallation

### Using opkg

```bash
opkg remove oobe
```

### Manual Uninstallation

```bash
# Stop the service
/etc/init.d/S94oobe stop

# Remove files
rm -f /usr/local/bin/oobe
rm -rf /usr/share/oobe
rm -f /etc/init.d/S94oobe
```

---

## Configuration

### Change Listen Port

Edit `/etc/init.d/S94oobe` and modify the `DAEMON_ARGS` line:

```bash
DAEMON_ARGS="--listen http://0.0.0.0:9000 --root /usr/share/oobe/www"
```

Then restart:
```bash
/etc/init.d/S94oobe restart
```

### Change Web Root

If you want to serve files from a different location:

```bash
DAEMON_ARGS="--listen http://0.0.0.0:8081 --root /path/to/custom/www"
```

---

## Integration with Supervisor

The OOBE service is designed to work alongside the supervisor service:

- **OOBE**: Port 8081 (HTTP) - Initial setup wizard
- **Supervisor**: Port 443 (HTTPS) - Main device management

### Typical Workflow

1. Device boots for the first time
2. User connects to device network
3. User accesses OOBE at `http://<device-ip>:8081`
4. OOBE guides through setup using supervisor APIs
5. After setup, user is redirected to supervisor UI at `https://<device-ip>`

### Disabling OOBE After Setup

If you want to disable OOBE after initial setup:

```bash
# Stop and disable the service
/etc/init.d/S94oobe stop
chmod -x /etc/init.d/S94oobe
```

Or remove it entirely:
```bash
opkg remove oobe
```

---

## Security Considerations

1. **HTTP Only**: OOBE runs on HTTP (not HTTPS) for simplicity during initial setup
2. **Local Network**: Should only be accessible on local network
3. **Firewall**: Consider blocking port 8081 from external access
4. **Temporary Service**: Can be disabled after initial setup is complete

### Recommended Firewall Rules

```bash
# Allow OOBE only from local network
iptables -A INPUT -p tcp --dport 8081 -s 192.168.0.0/16 -j ACCEPT
iptables -A INPUT -p tcp --dport 8081 -j DROP
```

---

## Advanced: Custom Build Integration

### Adding to reCamera-OS Build

1. Create a package directory in your Buildroot external tree:
   ```
   package/oobe/
   ├── Config.in
   ├── oobe.mk
   └── S94oobe
   ```

2. Add to `Config.in`:
   ```
   config BR2_PACKAGE_OOBE
       bool "oobe"
       help
         Out-of-Box Experience setup wizard
   ```

3. Create `oobe.mk`:
   ```makefile
   OOBE_VERSION = 0.1.0
   OOBE_SITE = $(TOPDIR)/../sscma-example-sg200x/solutions/oobe
   OOBE_SITE_METHOD = local
   
   define OOBE_BUILD_CMDS
       $(MAKE) -C $(@D) build-riscv
   endef
   
   define OOBE_INSTALL_TARGET_CMDS
       $(INSTALL) -D -m 0755 $(@D)/build/oobe $(TARGET_DIR)/usr/local/bin/oobe
       $(INSTALL) -d $(TARGET_DIR)/usr/share/oobe/www
       cp -r $(@D)/web/* $(TARGET_DIR)/usr/share/oobe/www/
       $(INSTALL) -D -m 0755 $(@D)/rootfs/etc/init.d/S94oobe $(TARGET_DIR)/etc/init.d/S94oobe
   endef
   
   $(eval $(generic-package))
   ```

4. Enable in menuconfig and rebuild firmware

---

## Support

For issues or questions:
- Check the [README.md](README.md) for general information
- Review [SUPERVISOR_API_REFERENCE.md](SUPERVISOR_API_REFERENCE.md) for API details
- Check device logs: `logread | grep -E '(oobe|supervisor)'`
