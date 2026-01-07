# Camera Watchdog Implementation Guide

## Current Status ⚠️

**Hardware Watchdog:** Currently **NOT enabled** on your system
- `/dev/watchdog` device does not exist
- Kernel driver not loaded or compiled
- **Needs kernel reconfiguration and rebuild**

**What Works Now:**
- ✅ Software-based monitoring (Options 1, 3, 4)
- ✅ Process monitoring via PID files
- ✅ Cron-based health checks
- ✅ Monit daemon (if installed)

**To Enable Hardware Watchdog:**
- Requires rebuilding kernel/Buildroot image
- See "Enabling Hardware Watchdog" section below

## Overview
This document outlines multiple approaches to implement a watchdog mechanism for auto-restarting the camera services (`camera-streamer` and `camera-recorder`) in case of failures.

## Current System Architecture

### Camera Services
1. **camera-streamer** - WebSocket H.264 streaming service
   - Binary: `/usr/local/bin/camera-streamer`
   - Init script: `/etc/init.d/S95camera-streamer`
   - PID file: `/var/run/camera-streamer.pid`
   
2. **camera-recorder** - H.264 video recording service
   - Binary: `/usr/local/bin/camera-recorder`
   - Expected init script: `/etc/init.d/S96camera-recorder`
   - Expected PID file: `/var/run/camera-recorder.pid`

### Current Init System
- Uses SysV-style init scripts (Buildroot default)
- Scripts run via `start-stop-daemon`
- No automatic restart on failure

## Hardware Watchdog Support

### SG200X / CV181x Chip Hardware Watchdog

**Good News:** Your SG200X/CV181x chip has **built-in hardware watchdog timer (WDT)** support!
**Current Status:** ⚠️ **Not Currently Enabled** - requires kernel configuration

#### Hardware Specifications

Based on the codebase analysis:

**Location:** Memory-mapped at `0x03010000`
**Driver:** DesignWare (Synopsys) Watchdog Timer (DW-WDT)
**Device Tree:** `/dev/watchdog0`

**Key Features:**
- Hardware-level system reset capability
- Configurable timeout periods
- Independent of software crashes
- Can reset entire SoC if not serviced
- Available in both u-boot and Linux kernel

**Device Tree Configuration:**
```dts
watchdog0: cv-wd@0x3010000 {
    compatible = "snps,dw-wdt";
    reg = <0x0 0x03010000 0x0 0x1000>;
    resets = <&rst RST_WDT>;
    clocks = <&pclk>;
    interrupts = <58 IRQ_TYPE_LEVEL_HIGH>;  // RISC-V variant
};
```

**Hardware Register Access:**
- Base Address: `0x03010000`
- Control Register: `WDT_CR` (enable/disable)
- Timeout Register: `WDT_TORR` (timeout range)
- Current Value: `WDT_CCVR` (current counter)
- Status Register: `WDT_STAT`

#### How Hardware Watchdog Works

1. **Initialization:** Kernel/bootloader enables the WDT with a timeout (e.g., 30 seconds)
2. **Kicking:** Software must periodically "kick" or "pet" the watchdog by writing to `/dev/watchdog`
3. **Failure:** If software crashes and stops kicking, hardware automatically resets the system
4. **Recovery:** System reboots and services restart automatically

#### Accessing Hardware Watchdog in Linux

**Device Interface:** `/dev/watchdog` or `/dev/watchdog0`

**Simple Test:**
```bash
# Check if hardware watchdog exists
ls -l /dev/watchdog*

# Start feeding the watchdog (system will NOT reset while this runs)
while true; do
    echo 1 > /dev/watchdog
    sleep 5
done
```

**From C Code:**
```c
#include <fcntl.h>
#include <unistd.h>

int fd = open("/dev/watchdog", O_WRONLY);
if (fd < 0) {
    perror("Failed to open watchdog");
    return -1;
}

// Keep alive - write any byte to reset the timer
while (1) {
    write(fd, "1", 1);
    sleep(5);  // Sleep less than timeout
}

close(fd);  // Closing will disable if CONFIG_WATCHDOG_NOWAYOUT not set
```

## Recommendations

### For Your Current Situation (HW Watchdog Not Enabled)

**Immediate Solution (Works Today):** Use **Option 1 (Cron-based watchdog)**
- ✅ No kernel rebuild required
- ✅ Works with current system
- ✅ Can implement in 10 minutes
- ✅ Provides service restart capability
- ⚠️ Polling-based (1-5 minute detection delay)
- ⚠️ No protection against kernel crashes

**Short-term Solution:** Use **Option 3 (Monit)**
- Add Monit package to Buildroot
- Rebuild image (faster than kernel rebuild)
- Professional monitoring
- Web interface
- Still works without HW watchdog

**Long-term Solution:** **Enable Hardware Watchdog** + Monit
- Rebuild with HW watchdog enabled
- Best of both worlds
- Ultimate reliability

### Implementation Priority

**Phase 1 - Today (No Rebuild Required):**
1. Implement cron-based watchdog (Option 1)
2. Test and verify camera restart works
3. Deploy immediately for basic protection

**Phase 2 - This Week (Image Rebuild):**
1. Add Monit to Buildroot packages
2. Rebuild image
3. Deploy Monit configuration
4. Better monitoring and control

**Phase 3 - Future (Full Rebuild with HW WDT):**
1. Enable hardware watchdog in kernel
2. Rebuild complete image
3. Configure HW watchdog integration
4. Ultimate system reliability

### Quick Start Script (Works Now)

Create `/usr/local/bin/camera-watchdog.sh`:
```bash
#!/bin/sh
# Simple software watchdog for camera services
# No hardware watchdog required - works on current system

LOG_TAG="camera-watchdog"

check_and_restart() {
    SERVICE=$1
    PIDFILE=$2
    INIT_SCRIPT=$3
    
    if [ -f "$PIDFILE" ]; then
        PID=$(cat "$PIDFILE")
        if ! kill -0 "$PID" 2>/dev/null; then
            logger -t $LOG_TAG "$SERVICE crashed (PID $PID not found), restarting..."
            $INIT_SCRIPT restart
            return 1
        fi
    else
        logger -t $LOG_TAG "$SERVICE PID file missing, starting service..."
        $INIT_SCRIPT start
        return 1
    fi
    
    return 0
}

# Check camera-streamer
check_and_restart "camera-streamer" \
    "/var/run/camera-streamer.pid" \
    "/etc/init.d/S95camera-streamer"

# Check camera-recorder (if exists)
if [ -x "/etc/init.d/S96camera-recorder" ]; then
    check_and_restart "camera-recorder" \
        "/var/run/camera-recorder.pid" \
        "/etc/init.d/S96camera-recorder"
fi

exit 0
```

Make executable and add to crontab:
```bash
chmod +x /usr/local/bin/camera-watchdog.sh

# Add to root's crontab (run every 2 minutes)
echo "*/2 * * * * /usr/local/bin/camera-watchdog.sh" | crontab -
```

**Test it:**
```bash
# Kill camera-streamer
kill -9 $(cat /var/run/camera-streamer.pid)

# Wait up to 2 minutes and check logs
tail -f /var/log/messages | grep camera-watchdog

# Should see restart message and service comes back up
```

## Watchdog Implementation Options

### Option 1: Simple Cron-based Watchdog (Easiest)

**Description:** Use cron to periodically check if processes are running and restart them if needed.

**Pros:**
- Simple to implement
- No additional dependencies
- Easy to debug
- Works on any Linux system

**Cons:**
- Polling-based (not immediate)
- Less sophisticated than other options
- Typically 1-5 minute restart delay

**Implementation:**

Create `/usr/local/bin/camera-watchdog.sh`:
```bash
#!/bin/sh
# Camera services watchdog script

check_and_restart() {
    SERVICE=$1
    PIDFILE=$2
    INIT_SCRIPT=$3
    
    if [ -f "$PIDFILE" ]; then
        PID=$(cat "$PIDFILE")
        if ! kill -0 "$PID" 2>/dev/null; then
            logger -t camera-watchdog "$SERVICE is not running, restarting..."
            $INIT_SCRIPT restart
        fi
    else
        logger -t camera-watchdog "$SERVICE PID file not found, starting..."
        $INIT_SCRIPT start
    fi
}

# Check camera-streamer
check_and_restart "camera-streamer" \
    "/var/run/camera-streamer.pid" \
    "/etc/init.d/S95camera-streamer"

# Check camera-recorder (if enabled)
if [ -x "/etc/init.d/S96camera-recorder" ]; then
    check_and_restart "camera-recorder" \
        "/var/run/camera-recorder.pid" \
        "/etc/init.d/S96camera-recorder"
fi
```

Add to crontab (runs every 2 minutes):
```
*/2 * * * * /usr/local/bin/camera-watchdog.sh
```

---

### Option 2: BusyBox Hardware Watchdog Daemon (Recommended)

**Description:** Use the Linux hardware watchdog with BusyBox's `watchdog` daemon to monitor processes and use SG200X's built-in HW watchdog.

**Pros:**
- Hardware-level protection (complete system reset on total failure)
- Standard Linux approach
- Can perform system reboot on complete failure
- Process-level monitoring available
- Uses SG200X's built-in WDT hardware

**Cons:**
- Requires hardware watchdog support (✅ Available on SG200X)
- More complex configuration
- Will cause full system reboot on failures (this is a feature!)

**Implementation:**

1. Enable watchdog in Buildroot config:
```
BR2_PACKAGE_BUSYBOX_WATCHDOG=y
BR2_LINUX_KERNEL_EXT_DW_WDT=y
```

2. Verify hardware watchdog device exists:
```bash
# Check device node
ls -l /dev/watchdog0

# Check kernel module
lsmod | grep watchdog
dmesg | grep -i watchdog
```

3. Create `/etc/watchdog.conf`:
```
# Hardware watchdog device (auto-detected usually)
watchdog-device = /dev/watchdog
watchdog-timeout = 60

# Monitor camera services by PID file
pidfile = /var/run/camera-streamer.pid
pidfile = /var/run/camera-recorder.pid

# Monitor shared memory
file = /dev/shm/video_stream_ch0
change = 30

# System limits
max-load-1 = 24
max-load-5 = 18
max-load-15 = 12

# Memory monitoring
min-memory = 1

# Logging
log-dir = /var/log/watchdog
```

4. Create init script `/etc/init.d/S15watchdog`:
```bash
#!/bin/sh

case "$1" in
    start)
        echo "Starting hardware watchdog daemon..."
        # Start watchdog daemon with config file
        watchdog -c /etc/watchdog.conf
        ;;
    stop)
        echo "Stopping watchdog daemon..."
        # Gracefully stop watchdog (if CONFIG_WATCHDOG_NOWAYOUT not set)
        killall -TERM watchdog
        ;;
    restart)
        $0 stop
        sleep 2
        $0 start
        ;;
    *)
        echo "Usage: $0 {start|stop|restart}"
        exit 1
        ;;
esac
```

5. Make executable and enable:
```bash
chmod +x /etc/init.d/S15watchdog
```

**Advanced: Hardware Watchdog with Custom Script**

For full control, write directly to `/dev/watchdog`:

Create `/usr/local/bin/camera-hw-watchdog.sh`:
```bash
#!/bin/sh
# Hardware watchdog keeper for camera services

WATCHDOG_DEV="/dev/watchdog"
CHECK_INTERVAL=10  # Check every 10 seconds
WATCHDOG_TIMEOUT=30  # HW watchdog timeout (must be > CHECK_INTERVAL)

# Open watchdog device
exec 3>$WATCHDOG_DEV

while true; do
    # Check camera-streamer
    if [ -f /var/run/camera-streamer.pid ]; then
        PID=$(cat /var/run/camera-streamer.pid)
        if ! kill -0 "$PID" 2>/dev/null; then
            logger -t hw-watchdog "camera-streamer failed, restarting..."
            /etc/init.d/S95camera-streamer restart
        fi
    fi
    
    # Check camera-recorder  
    if [ -f /var/run/camera-recorder.pid ]; then
        PID=$(cat /var/run/camera-recorder.pid)
        if ! kill -0 "$PID" 2>/dev/null; then
            logger -t hw-watchdog "camera-recorder failed, restarting..."
            /etc/init.d/S96camera-recorder restart
        fi
    fi
    
    # Feed the hardware watchdog (prevent system reset)
    echo "1" >&3
    
    sleep $CHECK_INTERVAL
done

# Close watchdog (will disable if not CONFIG_WATCHDOG_NOWAYOUT)
exec 3>&-
```

**Watchdog Timeout Configuration:**

```bash
# From userspace, configure timeout before feeding
echo "V" > /dev/watchdog     # Magic close character
echo "30" > /dev/watchdog    # Set 30 second timeout

# Or use ioctl from C code:
#include <linux/watchdog.h>
int timeout = 30;
ioctl(fd, WDIOC_SETTIMEOUT, &timeout);
```

---

### Option 3: Monit Process Monitoring (Most Featured)

**Description:** Use Monit, a dedicated process monitoring and restart daemon.

**Pros:**
- Purpose-built for process monitoring
- Rich configuration options
- Web interface for monitoring
- Can check process health via various methods
- Email/alert notifications
- Can monitor system resources

**Cons:**
- Additional package dependency (~500KB)
- More complex configuration
- Overkill for simple use cases

**Implementation:**

1. Add to Buildroot config:
```
BR2_PACKAGE_MONIT=y
```

2. Create `/etc/monitrc`:
```
set daemon 30  # Check every 30 seconds
set log syslog

# Web interface (optional)
set httpd port 2812 and
    use address localhost
    allow localhost

# Monitor camera-streamer
check process camera-streamer 
    with pidfile /var/run/camera-streamer.pid
    start program = "/etc/init.d/S95camera-streamer start"
    stop program = "/etc/init.d/S95camera-streamer stop"
    if failed host 127.0.0.1 port 8765 protocol websocket then restart
    if 5 restarts within 5 cycles then timeout
    if cpu > 80% for 5 cycles then alert
    if memory > 100 MB then alert

# Monitor camera-recorder
check process camera-recorder
    with pidfile /var/run/camera-recorder.pid
    start program = "/etc/init.d/S96camera-recorder start"
    stop program = "/etc/init.d/S96camera-recorder stop"
    if 5 restarts within 5 cycles then timeout
    if cpu > 80% for 5 cycles then alert
    if memory > 200 MB then alert

# Monitor shared memory (video pipeline)
check file video-shm path /dev/shm/video_stream_ch0
    if does not exist then alert
```

3. Create init script `/etc/init.d/S90monit`:
```bash
#!/bin/sh
start() {
    echo "Starting monit..."
    monit -c /etc/monitrc
}

stop() {
    echo "Stopping monit..."
    monit quit
}

case "$1" in
    start) start ;;
    stop) stop ;;
    restart) stop; sleep 2; start ;;
    *) echo "Usage: $0 {start|stop|restart}"; exit 1 ;;
esac
```

---

### Option 4: Modified Init Script with Auto-restart (Simple)

**Description:** Enhance existing init scripts to automatically respawn the process.

**Pros:**
- No external dependencies
- Immediate restart
- Simple modification

**Cons:**
- Tight restart loop risk
- No backoff mechanism
- Limited monitoring capabilities

**Implementation:**

Modify `/etc/init.d/S95camera-streamer`:
```bash
#!/bin/sh

DAEMON=/usr/local/bin/camera-streamer
NAME=camera-streamer
PIDFILE=/var/run/$NAME.pid
RESPAWN_LIMIT=5
RESPAWN_PERIOD=60

start_with_respawn() {
    echo "Starting $NAME with auto-restart"
    
    # Create a wrapper script that monitors and restarts
    cat > /tmp/${NAME}-monitor.sh <<'EOF'
#!/bin/sh
DAEMON=$1
NAME=$2
PIDFILE=$3
RESPAWN_LIMIT=$4
RESPAWN_PERIOD=$5

RESTART_COUNT=0
LAST_RESTART=0

while true; do
    CURRENT_TIME=$(date +%s)
    
    # Reset counter if enough time has passed
    if [ $((CURRENT_TIME - LAST_RESTART)) -gt $RESPAWN_PERIOD ]; then
        RESTART_COUNT=0
    fi
    
    # Check restart limit
    if [ $RESTART_COUNT -ge $RESPAWN_LIMIT ]; then
        logger -t ${NAME}-monitor "Restart limit reached, giving up"
        exit 1
    fi
    
    # Start the daemon
    logger -t ${NAME}-monitor "Starting $NAME (restart #$RESTART_COUNT)"
    $DAEMON &
    PID=$!
    echo $PID > $PIDFILE
    
    # Wait for process
    wait $PID
    EXIT_CODE=$?
    
    # Log failure
    logger -t ${NAME}-monitor "$NAME exited with code $EXIT_CODE, restarting..."
    
    RESTART_COUNT=$((RESTART_COUNT + 1))
    LAST_RESTART=$(date +%s)
    
    # Brief delay before restart
    sleep 5
done
EOF
    
    chmod +x /tmp/${NAME}-monitor.sh
    nohup /tmp/${NAME}-monitor.sh "$DAEMON" "$NAME" "$PIDFILE" "$RESPAWN_LIMIT" "$RESPAWN_PERIOD" > /dev/null 2>&1 &
    echo $! > ${PIDFILE}.monitor
}

# ... rest of init script functions ...
```

---

### Option 5: Supervisor/Systemd (Modern Approach)

**Description:** Use a modern process supervisor (systemd or supervisor daemon).

**Pros:**
- Industry-standard approach
- Automatic restart with backoff
- Service dependencies
- Better logging integration

**Cons:**
- Larger footprint (especially systemd)
- May require significant Buildroot reconfiguration
- Systemd may be overkill for embedded systems

**Implementation (Systemd):**

If using systemd, create `/etc/systemd/system/camera-streamer.service`:
```ini
[Unit]
Description=Camera Streamer Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/camera-streamer
Restart=always
RestartSec=10
StartLimitInterval=200
StartLimitBurst=5
User=root
Environment="LD_LIBRARY_PATH=/lib:/usr/lib:/mnt/system/lib"

# Watchdog configuration
WatchdogSec=30s

[Install]
WantedBy=multi-user.target
```

Enable with:
```bash
systemctl enable camera-streamer.service
systemctl start camera-streamer.service
```

### Enabling Hardware Watchdog

#### Step 1: Check Current Kernel Config

```bash
# Check if watchdog driver is available (module or builtin)
zcat /proc/config.gz | grep -i watchdog
# OR
cat /boot/config-$(uname -r) | grep -i watchdog

# Check kernel modules
lsmod | grep watchdog

# Check device tree
ls -l /proc/device-tree/*/watchdog*
```

#### Step 2: Enable in Buildroot

You need to rebuild your image with watchdog support enabled:

**File:** `reCamera-OS/.config` or via `make menuconfig`

```makefile
# Kernel Configuration
BR2_LINUX_KERNEL_CUSTOM_CONFIG_FILE="path/to/kernel.config"

# Add to kernel config:
CONFIG_WATCHDOG=y
CONFIG_WATCHDOG_CORE=y
CONFIG_DW_WATCHDOG=y
CONFIG_WATCHDOG_HANDLE_BOOT_ENABLED=y
# CONFIG_WATCHDOG_NOWAYOUT is not set  # Allow safe disable

# BusyBox Configuration
BR2_PACKAGE_BUSYBOX_CONFIG="path/to/busybox.config"
# Enable in BusyBox config:
CONFIG_WATCHDOG=y
```

**Via Buildroot menuconfig:**
```bash
cd reCamera-OS
make menuconfig

# Navigate to:
# Kernel -> Linux Kernel
#   [*] Use custom config
#   -> Configure custom kernel
#     Device Drivers ->
#       [*] Watchdog Timer Support ->
#         <*> Watchdog core
#         <*> Synopsys DesignWare watchdog

# Target packages -> 
#   Hardware handling ->
#     [*] watchdog
```

#### Step 3: Update Device Tree (if needed)

Verify the watchdog node is enabled in your device tree:

**File:** `reCamera-OS/build/boards/default/dts/cv181x/cv181x_base.dtsi`

```dts
watchdog0: cv-wd@0x3010000 {
    compatible = "snps,dw-wdt";
    reg = <0x0 0x03010000 0x0 0x1000>;
    resets = <&rst RST_WDT>;
    clocks = <&pclk>;
    interrupts = <58 IRQ_TYPE_LEVEL_HIGH>;
    status = "okay";  /* Make sure it's not "disabled" */
};
```

#### Step 4: Rebuild and Flash

```bash
cd reCamera-OS
make clean
make

# Flash the new image to your device
# (specific instructions depend on your flashing method)
```

#### Step 5: Verify After Reboot

```bash
# Check device exists
ls -l /dev/watchdog*

# Should see:
# crw------- 1 root root 10, 130 Jan  1 00:00 /dev/watchdog
# crw------- 1 root root 252, 0 Jan  1 00:00 /dev/watchdog0

# Check kernel driver
dmesg | grep -i watchdog
# Should see something like:
# [    0.123456] dw_wdt 3010000.watchdog: initialized

# Check sysfs
cat /sys/class/watchdog/watchdog0/status
```

## Hardware Watchdog Testing

### Test Procedure

1. **Verify Hardware Support:**
```bash
# Check device exists
ls -l /dev/watchdog*

# Check kernel driver loaded
dmesg | grep -i watchdog
cat /proc/devices | grep watchdog

# Check device tree
cat /proc/device-tree/*/watchdog*/compatible
```

2. **Test Manual Operation:**
```bash
# Open watchdog (starts countdown)
cat /dev/watchdog &
WATCHDOG_PID=$!

# System will reboot in ~60s if not fed

# Feed the watchdog before timeout
echo 1 > /dev/watchdog  # Resets timer

# Kill process to close watchdog
kill $WATCHDOG_PID
```

3. **Test Auto-Reboot:**
```bash
# Open watchdog and DON'T feed it
echo "Test auto-reboot" > /dev/watchdog
# Wait... system should reboot when timeout expires

# Check logs after reboot
dmesg | grep -i "watchdog\|reset"
```

4. **Test with Camera Services:**
```bash
# Start watchdog script
/usr/local/bin/camera-hw-watchdog.sh &

# Kill camera process manually
kill -9 $(cat /var/run/camera-streamer.pid)

# Watch logs - should see restart
tail -f /var/log/messages | grep watchdog
```

## Integration Points

### Buildroot Package Addition (for Monit)

Add to [`reCamera-OS/external/br2-external/Config.in`](reCamera-OS/external/br2-external/Config.in):
```makefile
source "$BR2_EXTERNAL_BR2EXT_PATH/sscma-camera-recorder/Config.in"
source "$BR2_EXTERNAL_BR2EXT_PATH/sscma-camera-streamer/Config.in"
source "$BR2_EXTERNAL_BR2EXT_PATH/camera-watchdog/Config.in"  # New
```

### Hardware Watchdog Kernel Config

Ensure these are enabled in your Linux kernel config:

```makefile
CONFIG_WATCHDOG=y
CONFIG_WATCHDOG_CORE=y
CONFIG_DW_WATCHDOG=y          # DesignWare WDT (your chip)
CONFIG_WATCHDOG_NOWAYOUT=n    # Allow clean shutdown
```

### Camera Recorder Init Script

Currently, [`camera-recorder`](sscma-example-sg200x/solutions/camera-recorder/main.cpp) doesn't have an init script. Create one similar to camera-streamer:

```bash
# Create init script for camera-recorder
mkdir -p sscma-example-sg200x/solutions/camera-recorder/rootfs/etc/init.d
cp sscma-example-sg200x/solutions/camera-streamer/rootfs/etc/init.d/S95camera-streamer \
   sscma-example-sg200x/solutions/camera-recorder/rootfs/etc/init.d/S96camera-recorder

# Update the new script with camera-recorder specifics
```

Then update the [`.mk` file](reCamera-OS/external/br2-external/sscma-camera-recorder/sscma-camera-recorder.mk) to install rootfs files.

## Conclusion

### Hardware Watchdog - The Ultimate Safety Net

Your **SG200X chip has built-in hardware watchdog timer support** via the DesignWare WDT controller. This is the most reliable watchdog mechanism available because:

1. **Hardware-level:** Independent of software failures
2. **Automatic:** Resets system if not regularly serviced
3. **Bulletproof:** Works even if kernel hangs or crashes
4. **Available:** Already present at `/dev/watchdog` (if driver loaded)

### Recommended Implementation Path

1. **Immediate (Day 1):** Enable hardware watchdog with simple script (Option 2)
   - Verify HW support exists
   - Create simple monitoring script
   - Automatic full system recovery

2. **Short-term (Week 1):** Add BusyBox watchdog daemon
   - Configure process monitoring
   - Add health checks
   - Set appropriate timeouts

3. **Long-term (Production):** Consider adding Monit
   - Fine-grained control
   - Monitoring dashboard
   - Keep HW watchdog as ultimate failsafe

The hardware watchdog provides **guaranteed system recovery** even from the worst failures (kernel crashes, deadlocks, hardware issues), while software monitoring provides **graceful service restarts** for application-level issues.
