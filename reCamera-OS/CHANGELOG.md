## 0.2.8 (2026-02-16)

### sscma-example-sg200x

- **New Features:**

    - **ML Detection Pipeline**
      - Added new `camera-detector` solution with ML-based object detection
      - Shared memory IPC for efficient video frame sharing between processes (`video_raw_shm`)
      - Detection queue with automatic platform upload
      - CameraDB component for local detection storage and management
      - WebSocket-based detection client for real-time results
      - Files: [`camera-detector/`](sscma-example-sg200x/solutions/camera-detector/), [`cameradb/`](sscma-example-sg200x/components/cameradb/), [`detectionqueue/`](sscma-example-sg200x/solutions/supervisor/internal/detectionqueue/), [`detection.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/detection.go), [`detection_ws.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/detection_ws.go)

    - **Detections Web UI**
      - New detections view in Supervisor web interface for viewing detection history
      - API endpoints for detection data retrieval
      - Files: [`views/detections/index.tsx`](sscma-example-sg200x/solutions/supervisor/www/src/views/detections/index.tsx), [`api/detection/index.ts`](sscma-example-sg200x/solutions/supervisor/www/src/api/detection/index.ts)

    - **NTP Time Synchronization**
      - Added NTP client for automatic time synchronization
      - Files: [`ntp/ntp.go`](sscma-example-sg200x/solutions/supervisor/internal/ntp/ntp.go)

    - **Auto Updates**
      - Automatic firmware update checking and installation
      - Improved update UI in Supervisor web interface
      - Files: [`views/updates/index.tsx`](sscma-example-sg200x/solutions/supervisor/www/src/views/updates/index.tsx), [`upgrade/upgrade.go`](sscma-example-sg200x/solutions/supervisor/internal/upgrade/upgrade.go)

    - **Model Update System**
      - OTA model update functionality for ML models
      - Files: [`modelupdate/modelupdate.go`](sscma-example-sg200x/solutions/supervisor/internal/modelupdate/modelupdate.go), [`model_update.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/model_update.go)

    - **Upgrade Signing & Verification**
      - Added cryptographic signing for firmware upgrades
      - FreeTSA timestamp authority integration
      - Files: [`upgrade/signing.go`](sscma-example-sg200x/solutions/supervisor/internal/upgrade/signing.go), [`timestamp/`](sscma-example-sg200x/solutions/supervisor/internal/timestamp/)

    - **Security Configuration**
      - Added security configuration management
      - Updated security views with improved user management
      - Files: [`security/config.go`](sscma-example-sg200x/solutions/supervisor/internal/security/config.go), [`views/security/`](sscma-example-sg200x/solutions/supervisor/www/src/views/security/)

    - **Storage Management**
      - Storage handler for device storage operations
      - Files: [`handler/storage.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/storage.go)

    - **Network Utilities**
      - Added system network utilities
      - Files: [`system/network.go`](sscma-example-sg200x/solutions/supervisor/internal/system/network.go)

- **Configuration Changes:**

    - **Camera Streamer Updates**
      - Significant updates to camera streaming main application
      - Files: [`camera-streamer/main/main.cpp`](sscma-example-sg200x/solutions/camera-streamer/main/main.cpp)

    - **Camera Recorder Updates**
      - Updates to camera recording functionality
      - Files: [`camera-recorder/main.cpp`](sscma-example-sg200x/solutions/camera-recorder/main.cpp)

    - **OOBE Improvements**
      - Updated OOBE wizard with improved camera registration flow
      - UI/UX improvements to setup process
      - Files: [`oobe/web/`](sscma-example-sg200x/solutions/oobe/web/)

    - **API URL Updates**
      - Updated platform API URL configuration
      - Files: [`config/config.go`](sscma-example-sg200x/solutions/supervisor/internal/config/config.go)

    - **WiFi Handler Updates**
      - Improved WiFi management
      - Files: [`handler/wifi.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/wifi.go)

- **Bug Fixes:**
    - Various stability fixes
    - WiFi connectivity improvements
    - Error handling improvements

- **Statistics:**
    - **Total Changes:** 77 files changed
    - **Additions:** +15,837 lines
    - **Deletions:** -1,030 lines
    - **Net:** +14,807 lines

### reCamera-OS

- **Maintenance:**
    - Code cleanup and optimization
    - Files: Various cleanup across 8 files

- **Statistics:**
    - **Total Changes:** 8 files changed
    - **Additions:** +59 lines
    - **Deletions:** -594 lines
    - **Net:** -535 lines

## 0.2.7 (2026-02-02)

### sscma-example-sg200x

- **New Features:**

    - **Remote Video Streaming (Relay Integration)**
      - Added relay forwarder system enabling remote video streaming through Authority Alert platform
      - Camera can now forward H.264 streams to relay server for remote viewing
      - Supports WebSocket-based streaming with automatic reconnection
      - Files: [`relay_control.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/relay_control.go), [`relay_discovery.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/relay_discovery.go), [`external_relay.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/external_relay.go), [`forwarder_manager.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/forwarder_manager.go), [`RELAY_INTEGRATION.md`](sscma-example-sg200x/solutions/camera-streamer/RELAY_INTEGRATION.md)

    - **Simplified Device Registration (Claim Code)**
      - Replaced OAuth flow with simple 6-character claim code registration
      - Users now enter a code displayed on device into the web dashboard
      - Added network resilience with automatic retry on connectivity issues
      - Internet status checking endpoint (`/api/device/internet-status`)
      - Files: [`device.go`](sscma-example-sg200x/solutions/supervisor/internal/handler/device.go), [`oobe-app.js`](sscma-example-sg200x/solutions/oobe/web/js/oobe-app.js)

    - **Platform TLS Support**
      - Added platform CA certificate handling for secure communication
      - Files: [`platform_ca.go`](sscma-example-sg200x/solutions/supervisor/internal/tls/platform_ca.go)

    - **API Documentation**
      - Added comprehensive OpenAPI 3.0.3 specification for Supervisor API
      - Documents all endpoints: authentication, device management, network, file operations, camera streaming, system administration
      - Files: [`supervisor-api-spec.yaml`](sscma-example-sg200x/solutions/supervisor/supervisor-api-spec.yaml)

    - **Security Scanning**
      - Added Codacy security scan GitHub workflow for automated code analysis
      - Files: [`.github/workflows/codacy.yml`](sscma-example-sg200x/.github/workflows/codacy.yml)

- **Configuration Changes:**

    - **QR Code Reader Packaging**
      - Updated build system with improved Makefile
      - Added opkg package control file for distribution
      - Files: [`Makefile`](sscma-example-sg200x/solutions/sscma-qrcode-reader/Makefile), [`opkg/CONTROL/control`](sscma-example-sg200x/solutions/sscma-qrcode-reader/opkg/CONTROL/control)

    - **Supervisor Web UI Updates**
      - Updated security views and hooks for improved user management
      - Files: [`hook.ts`](sscma-example-sg200x/solutions/supervisor/www/src/views/security/hook.ts), [`index.tsx`](sscma-example-sg200x/solutions/supervisor/www/src/views/security/index.tsx)

    - **API URL Configuration**
      - Updated platform API URL configuration
      - Files: [`config.go`](sscma-example-sg200x/solutions/supervisor/internal/config/config.go)

- **Breaking Changes:**

    - **Registration Flow Changed**
      - OAuth-based registration removed in favor of claim code system
      - OOBE wizard now generates a code for users to enter on web dashboard
      - Existing OAuth callback endpoints removed

- **Statistics:**
    - **Total Changes:** 33 files changed
    - **Additions:** +7,159 lines
    - **Deletions:** -945 lines
    - **Net:** +6,214 lines

## 0.2.6 (2026-01-12)

### sg2002_recamera_emmc

- **New Features:**
    
    - **Per-device hostname + local discovery (mDNS/Bonjour)**
      - Added init script to set hostname early based on saved device name or MAC suffix (`authorityalert-<ddeeef>`)
      - Avahi now uses the system hostname (no hardcoded `host-name`) to avoid collisions across devices
      - Advertises local web UI ports over mDNS (HTTP/80 and HTTPS/443)
      - Files: [`S06device-identity`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S06device-identity), [`avahi-daemon.conf`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/avahi/avahi-daemon.conf), [`authorityalert-web.service`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/avahi/services/authorityalert-web.service)

- **Configuration Changes:**

    - **AP / captive-portal friendliness**
      - Added DHCP option 114 (RFC 8910) to dnsmasq configs to hint captive portal URL
      - Updated default AP SSID from `AUOK` to `AuthorityAlert`
      - Files: [`default.conf`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/dnsmasq/default.conf), [`usb_wlan1.conf`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/dnsmasq/usb_wlan1.conf), [`hostapd_2g4.conf`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/hostapd_2g4.conf), [`hostapd_5g.conf`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/hostapd_5g.conf)

    - **OOBE Buildroot package updates**
      - OOBE now builds via explicit CMake configure/build steps
      - Always installs the OOBE binary to `/usr/local/bin/oobe` and web assets to `/usr/share/oobe/www`
      - Added Buildroot dependency on `libhv`
      - Files: [`oobe.mk`](reCamera-OS/external/br2-external/oobe/oobe.mk), [`oobe/Config.in`](reCamera-OS/external/br2-external/oobe/Config.in)

    - **Branding defaults**
      - Updated default Buildroot hostname: `reCamera` → `authorityalert`
      - Updated `/etc/issue` banner to `AuthorityOS`
      - Files: [`cvitek_CV181X_musl_riscv64_defconfig`](reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig), [`issue`](reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/rootfs/etc/issue)

- **Statistics:**
    - **Total Changes:** 11 files changed
    - **Additions:** +140 lines
    - **Deletions:** -36 lines
    - **Net:** +104 lines

## 0.2.5 (2026-01-10)

### sg2002_recamera_emmc

- **New Features:**
    
    - **Out-of-Box Experience (OOBE) Wizard**
      - Added new `oobe` Buildroot external package for first-time device setup
      - Includes backend + web assets packaging via Buildroot
      - Integrated into Buildroot external package menu
      - Files: [`oobe/Config.in`](reCamera-OS/external/br2-external/oobe/Config.in), [`oobe.mk`](reCamera-OS/external/br2-external/oobe/oobe.mk), [`br2-external/Config.in`](reCamera-OS/external/br2-external/Config.in)

- **Configuration Changes:**
    - Enabled CA certificates bundle in Buildroot for TLS trust store support
      - Updated [`cvitek_CV181X_musl_riscv64_defconfig`](reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig)

- **Statistics:**
    - **Total Changes:** 4 files changed
    - **Additions:** +55 lines
    - **Deletions:** -1 line
    - **Net:** +54 lines

## 0.2.4 (2026-01-07)

### sg2002_recamera_emmc

- **New Features:**
    
    - **Camera Recording Support**
      - Added `sscma-camera-recorder` Buildroot external package
      - Provides H.264 video recording to MP4 for later playback
      - Files: [`sscma-camera-recorder/Config.in`](reCamera-OS/external/br2-external/sscma-camera-recorder/Config.in), [`sscma-camera-recorder.mk`](reCamera-OS/external/br2-external/sscma-camera-recorder/sscma-camera-recorder.mk)

    - **Nix Flake Build Environment**
      - Added Nix flake for reproducible builds and a development shell
      - Files: [`flake.nix`](reCamera-OS/flake.nix), [`flake.lock`](reCamera-OS/flake.lock)

    - **Filesystem Support**
      - Enabled kernel exFAT filesystem support (`CONFIG_EXFAT_FS=y`)
      - Updated [`cvitek_sg2002_recamera_emmc_defconfig`](reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/linux/cvitek_sg2002_recamera_emmc_defconfig)

- **Bug Fixes:**
    
    - **WiFi Stability Improvements**
      - Updated default `wpa_supplicant` configuration to reduce disconnects
      - Updated [`wpa_supplicant.conf`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/wpa_supplicant.conf)

- **Configuration Changes:**
    - Updated SD hotplug automount behavior and mountpoint to `/mnt/sd`
      - Updated [`automount.sh`](reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/hotplug/automount.sh)
    - Updated OTA download/upgrade URLs to `ThePoliceRecord/authority-alert-OS` releases and removed fallback URL list
      - Updated [`auto_download.sh`](reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/auto_download.sh) and [`upgrade.sh`](reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/upgrade.sh)
    - Updated package selection/behavior for streaming + supervisor components
      - Install path change: `camera-streamer` now installs to `/usr/local/bin`
      - Supervisor version set to `development`
      - Updated [`sscma-camera-streamer.mk`](reCamera-OS/external/br2-external/sscma-camera-streamer/sscma-camera-streamer.mk) and [`sscma-supervisor.mk`](reCamera-OS/external/br2-external/sscma-supervisor/sscma-supervisor.mk)
    - Documentation updated for new repository URLs
      - Updated [`README.md`](reCamera-OS/README.md)

- **Breaking Changes:**
    
    - **camera-streamer install path change**
      - Path changed from `/usr/bin/camera-streamer` to `/usr/local/bin/camera-streamer`
      - Any scripts/services with hardcoded paths must be updated

    - **SD mountpoint behavior change**
      - SD mountpoint moved from `/tmp/sd` to `/mnt/sd`
      - Any scripts relying on the old mountpoint or symlink behavior must be updated

- **Statistics:**
    - **Total Changes:** 12 files changed
    - **Additions:** +318 lines
    - **Deletions:** -30 lines
    - **Net:** +288 lines

## 0.2.3 (2025-12-27)

### sg2002_recamera_emmc

- **New Features:**
    
    - **Camera Streaming Support**
      - Added sscma-camera-streamer package for video streaming functionality
      - Integrated with SSCMA supervisor for camera stream management
      - Provides real-time video streaming capabilities for remote monitoring
      - Configuration updates in br2-external packages

- **Configuration Changes:**
    - Updated [`Config.in`](reCamera-OS/external/br2-external/Config.in) to include camera-streamer package
    - Modified [`sscma-supervisor.mk`](reCamera-OS/external/br2-external/sscma-supervisor/sscma-supervisor.mk) for camera streaming integration
    - Added complete [`sscma-camera-streamer`](reCamera-OS/external/br2-external/sscma-camera-streamer/sscma-camera-streamer.mk) package definition

- **Statistics:**
    - **Total Changes:** 4 files changed
    - **Additions:** +50 lines
    - **Deletions:** -1 line
    - **Net:** +49 lines

## 0.2.2 (2025-12-27)

### sg2002_recamera_emmc

- **New Features:**
    
    - **OTA Security Enhancement (Breaking Change)**
      - Migrated from MD5 to SHA256 hash verification
      - **Server Changes:**
        - `build.sh`: Generate `sha256sum.txt` instead of `md5sum.txt` in OTA artifacts
        - `ota_server/prepare_release.sh`: Create `sg2002_recamera_emmc_sha256sum.txt` manifests
        - `.gitlab-ci.yml`: Package stage generates SHA256 checksums
      - **Client Changes:**
        - `upgrade.sh`: Complete rewrite with SHA256-only verification (177 line changes)
        - Automatic hash algorithm detection for safer upgrades
        - Removed all MD5 fallback logic
      - **Security:** SHA256 provides 256-bit cryptographic security vs MD5's broken 128-bit
    
    - **Filesystem Support**
      - Added full exFAT support (`BR2_PACKAGE_EXFATPROGS=y`)
      - Tools: mkfs.exfat, fsck.exfat, exfatlabel
      - Enables formatting/checking exFAT on external media
    
    - **Build System Overhaul**
      - **Buildroot Package Updates:**
        - **Networking:** Added c-ares 1.32.3 (CVE fixes), openssh 9.9p1, dropbear 2024.86
        - **System:** Added coreutils 9.5, util-linux 2.40.2, expat 2.6.4
        - **Kernel:** Added kmod 33 with musl/Python 3.13 compatibility
        - **Security:** Added hostapd 2.11, libgpg-error 1.51, libopenssl 3.3.2
        - **Python:** Upgraded to Python 3.13.1 with full packaging infrastructure
          - python-flit-core, python-installer, python-pypa-build
          - python-pyproject-hooks, python-setuptools, python-wheel
          - python-pip, python-py, python-packaging
          - python-bcrypt, python-passlib for authentication
        - **Gettext:** Added gettext-gnu 0.22.5 / gettext-tiny 0.3.2 with GETTEXTIZE fix
        - **Build Tools:** Updated autoconf 2.72, automake 1.17
        - **Busybox:** Updated with kernel 6.8+ support + security patches (CVE-2023-42366)
        - **WebSockets:** Added libwebsockets 4.3.3
        - **Networking:** Added zerotier-one 1.14.2 for VPN support
      
      - **Downgraded Packages:**
        - libuv: 1.51.0 → 1.44.2 (fixes pthread_getname_np musl incompatibility)
      
      - **Removed Packages:**
        - Removed nodejs package (cross-compilation issues on musl/RISC-V)
        - Removed sscma-node (nodejs dependency conflicts)
      
      - **Docker Build:**
        - Enhanced `.devcontainer/Dockerfile` with 7 new lines
        - Improved `docker_build.sh` script
        - Added `external/setenv.sh` for environment setup
      
      - **Linux Kernel:**
        - Updated `cvitek_sg2002_recamera_emmc_defconfig` (5 line changes)
        - Maintained exFAT driver support (CONFIG_EXFAT_FS=y)

- **Breaking Changes:**
    
    - **OTA Incompatibility**
      - Devices with firmware < 0.2.2 cannot OTA update from 0.2.2+ servers
      - Manifest filename changed: `*_md5sum.txt` → `*_sha256sum.txt`
      - No MD5 backward compatibility - enhanced security priority
      - **Migration Required:** Manual flash for devices < 0.2.2
    
    - **Removed Software**
      - Node.js and sscma-node no longer included
      - Applications depending on Node.js must be refactored

- **Configuration Changes:**
    - **Buildroot defconfig** (`cvitek_CV181X_musl_riscv64_defconfig`): 38 modified lines
      - Enabled/disabled packages per above lists
      - Updated package versions and dependencies
      - Configured new build infrastructure

- **Bug Fixes:**
    - Fixed musl libc pthread compatibility issues
    - Fixed Python 3.13 build infrastructure compatibility
    - Resolved gettext GETTEXTIZE variable conflicts
    - Applied busybox security patches

- **Documentation:**
    - Added `spec/MD5_TO_SHA256_MIGRATION_PLAN.md` - comprehensive migration guide
    - Updated all OTA documentation for SHA256 workflow
    - Updated README.md, CUSTOMIZATION_AND_OTA.md, OTA_SERVER_DOCKER.md
    - Added ttyd security hardening guide
    - Added eFuse decoder utility documentation
    - Updated `.gitignore` (3 line changes)

- **Testing & Validation:**
    - Static code analysis confirms complete MD5 removal
    - Build system validated with musl/RISC-V architecture
    - CI/CD pipelines tested for SHA256 generation
    - Docker build improvements verified

- **Statistics:**
    - **Total Changes:** 168 files changed
    - **Additions:** +9,930 lines
    - **Deletions:** -1,142 lines
    - **Net:** +8,788 lines
    - **New Packages:** 29 added
    - **Removed Packages:** 2 deleted
    - **Updated Packages:** 15 modified

## 0.2.1 (2025-09-12)

### sg2002_recamera_emmc

- New Features:
    - File browser
    - SSH on/off
    - Optimize network connection
    - Upgrade node-red to 4.1.0
    - Add CDC support
    - Specified node-reddash@1.26.0
    - Increase ION size to 60M
    - Set TPU max to 700MHz

- Bug Fixes:
    - Resolved 'isp err chk:7271(): CSIBDG A fifo overflow'
    - Resolved some other bugs

## 0.2.0 (2025-03-31)

### sg2002_recamera_emmc

- New Features:
    - Add gimbal automatic calibration
    - Add Node-red gimbal flow

- Bug Fixes:
    - Resolved various bugs in the SSCMA

## 0.1.5 (2025-01-22)

### sg2002_recamera_emmc

- New Features:
    - Added support for device discovery
    - Added support for 63-byte WiFi passwords
    - Added support for audio recording
    - Added ability to enable/disable nodes in the SSCMA Node
    - Introduced a more user-friendly interface for improved interaction

- Bug Fixes:
    - Resolved various bugs in the SSCMA Node
    - Resolved various bugs in the SSCMA Supervisor

If you need any more modifications, feel free to ask!
## 0.1.4 (2024-12-23)

### sg2002_recamera_emmc

- New features:
    - support fip and boot partition auto update
    - support /mnt/system/upgrade.sh start *ota.zip
    - support gimbal (spi-can mcp2518fd)
    - add cvi_pinmux tool
    - use udev instead of mdev
    - udev rules for sd auto mount

- Fix bugs:
    - factory reset failed (if sg2002_recamera_emmc_md5sum.txt is not exist)
    - solve the problem that the pc cannot access the Internet when connected via usb
    - dnsmasq fails to run without wifi

## 0.1.3 (2024-11-08)

### sg2002_recamera_emmc

- New features:
    - support 64G emmc

## 0.1.2 (2024-11-01)

### sg2002_recamera_emmc

- New features:
    - support global.gc operation
    - built-in model files

- Fix bugs:
    - solve some supervisor bugs
    - solve some sscma-node bugs

## 0.1.1 (2024-10-30)

### sg2002_recamera_emmc

- New features:
    - limit node-red to version 3.1.14

- Fix bugs:
    - solve some supervisor bugs
    - fix sd_gen_recovery_image.sh script bugs

## 0.1.0 (2024-10-28)

### sg2002_recamera_emmc

- New features:
    - update ov5647 isp params (denoise)
    - optimize ota

## 0.0.9 (2024-10-26)

### sg2002_recamera_emmc

- New features:
    - enable nodejs --v8-lite-mode (disable WebAssembly)
    - optimize system upgrade operations

- Fix bugs:
    - remove node-red-dashboard from node-red
    - solve some sscma-node bug

## 0.0.8 (2024-10-22)

### sg2002_recamera_emmc

- New features:
    - auto swapon /userdata/.swapfile
    - reduce node-red startup time (skip npm -v)
    - supervior
        - add the operation that AP will automatically turn on or off according to the status of WiFi
        - split the wifi scan into two operations: scan wifi and get scan results
        - add a judgement that an upgrade is in progress

- Fix bugs:
    - solve some supervisor bug
    - solve some sscma-node bug

## 0.0.7 (2024-10-16)

### sg2002_recamera_emmc

- New features:
    - reduce ION_SIZE to 50M
    - kernel support swap
    - kernel support advise syscalls
    - update c-ares to 1.32.2
    - update libuv to 1.48.0
    - update nodejs to 22.8.0
    - update node-red to 4.0.0
    - sd supports hotplug
    - close swupdate auto start (`sudo /usr/lib/swupdate/swupdate.sh`)

- Fix bugs:
    - solve some supervisor bug
    - solve some sscma-node bug

## 0.0.6 (2024-10-12)

### sg2002_recamera_emmc

- New features:
    - sensor auto detection (ov5647/sc530ai)
    - supports restore to factory
    - get mac & sn from efuse
    - add sscma-node program
    - supervisor
        - support https service
        - add service status detection
        - add a feature for file management
        - add wifi password verification operation

- Fix bugs:
    - solve some compilation issues
    - solve some supervisor bugs

## 0.0.5 (2024-09-25)

### sg2002_recamera_emmc

- New features:
    - update node-red interface style
    - supervisor
        - add APIs for uploading models and getting model information
        - allow CORS

- Fix bugs:
    - fix the problem that some configuration files did not exist
    - node-red start failed when reset system
    - solve some supervisor bugs

- Docs:
    - add compilation notes

## 0.0.4 (2024-09-12)

### sg2002_recamera_emmc

- New features:
    - buildin supervisor
    - buildin npm@8.11.0 and node-red@v3.1.11
    - update ov5647 isp params
    - supports rootfs overlay (/bin /etc /lib /home /root /usr /var)
    - add recamera as default user
    - supports sudo
    - upgrade icu to 73-2

- Fix bugs:
    - upgrade.sh checksum failed
    - fixed ov5647 mirror

## 0.0.3 (2024-08-30)

### sg2002_recamera_emmc
- update sdk upstream (6cd7a5b)
- support more buildroot packages (mosquitto/avahi/opkg/live555)
- remove reCamera app
- change ota (not compatible with last version) and supports swupdate
- use ncm replace of rndis
- expand rootfs size to 512M, rootfs default readonly (rootfs_rw (on|off))
- auto mount /dev/mmcblk0p6 to /userdata
- update nodejs to 17.9.1

## 0.0.2 (2024-06-24)

### sg2002_recamera_emmc
- update sdk upstream
- support booting from sd (sg2002_reCamera_0.0.2_emmc_sd_compat.zip)
- support sd recovery (sg2002_reCamera_0.0.2_emmc_recovery.zip)

## 0.0.1 (2024-06-21)

### sg2002_recamera_emmc
- First beta release
- SDK: supported emmc/sdcard/leds/wifi/uart/ethernet/ov5647 sensor
- APP: complete basic functions(Overview/Security/Newwork/Terminal/Setting)
