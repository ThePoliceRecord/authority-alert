# Packages on Device (Overview and Guidance)

Owner: YOU
Last updated: YYYY-MM-DD

Purpose
- Summarize what is installed on the camera image.
- Show where each service is configured/started.
- Provide guidance to keep/trim/swap packages for your use case.

## Core OS
- BusyBox + eudev (dynamic /dev), Coreutils, Util-linux, Kmod, Sudo
- Init system: BusyBox SysV-style init scripts under `/etc/init.d/`
- Useful tools: `strace`, `minicom`

## Remote Access
- OpenSSH server/client
  - Config: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig:385`
  - Keys/users: standard OpenSSH locations under `/etc/ssh/`
- TTYD (web terminal)
  - Service script: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S98ttyd:1`
  - Config file: `/etc/ttyd.conf` (optional). Env-style vars supported by script:
    - `TTYD_PORT` (default 9090), `TTYD_USER` (default `recamera`)
    - `TTYD_CREDENTIAL` (format `user:pass`), `TTYD_READONLY=true|false`
    - `TTYD_MAX_CLIENTS`, `TTYD_DEBUG=true|false`

Guidance
- Pick one remote access path for production (SSH OR TTYD). If SSH is kept, consider disabling TTYD or enforce credentials/TLS.

## Networking and Services
- DHCP client: `dhcpcd`
- DNS/DHCP/TFTP: `dnsmasq`
  - Init script: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S80dnsmasq:1`
  - Default config: `.../overlay/etc/dnsmasq/default.conf:1`
- NTP: `ntpd`/`ntpdate` (time sync)
  - Config: `.../overlay/etc/ntp.conf:1`
- Avahi/mDNS: `avahi-daemon` (+ AutoIPD, DNS‑SD compat)
  - Config: `.../overlay/etc/avahi/avahi-daemon.conf:1`
- Iperf3 (network testing)

Guidance
- Disable Avahi and iperf3 on locked-down networks.

## Wireless and Bluetooth
- Wi‑Fi station/AP: `wpa_supplicant`, `hostapd`, `iw`, `wireless-tools`, `rfkill`
  - WPA supplicant conf: `.../overlay/etc/wpa_supplicant.conf:1`
  - Hostapd templates: `.../overlay/etc/hostapd.conf.default:1`, `hostapd_2g4.conf:1`, `hostapd_5g.conf:1`
- Bluetooth: BlueZ 5 utils (client, OBEX, test, health)
- Firmware (example): CYW43012 blobs in overlay
  - `.../overlay/lib/firmware/brcm/brcmfmac43012-sdio.bin:1`

Guidance
- Keep only what you need (station-only vs AP). Drop BlueZ if BT not required.

## Application Layer
- Node.js + npm + Node‑RED 4.1.0
  - Buildroot config: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig:349`
  - Node‑RED service: `.../overlay/etc/init.d/S03node-red:1`
  - Default Node‑RED opts: `--max-old-space-size=64 --expose-gc` (tuneable)
- Python 3 runtime + `pip`, `passlib`, `bcrypt`
- Mosquitto (MQTT broker)
- OPKG (package manager) with GPG signing enabled

Guidance
- Node‑RED is great for customization but costs RAM; set `--max-old-space-size=32/48` if tight on memory, or disable entirely if not used.
- Mosquitto is needed by SSCMA flows; remove if unused.

## Media / Vision / Audio
- FFmpeg (codec/processing) and Live555 (RTSP)
- ALSA library + tools, mpg123
- ZXing‑CPP (QR/barcode)

Guidance
- FFmpeg/Live555 are heavy—keep if you stream/transcode; otherwise remove to shrink image and boot time.

## OTA / Update
- Primary OTA: `upgrade.sh` A/B updater
  - Script: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/upgrade.sh:1`
  - Auto‑poller: `.../system/auto_download.sh:1`
  - Custom server toggle: `/etc/upgrade` (e.g., `1,<sha256-url>`)
- swupdate available (with website installed) but not auto‑started by default
  - Buildroot: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig:556`
  - Entrypoint: `reCamera-OS/external/buildroot/package/swupdate/swupdate.sh:1`

Guidance
- If you need signed updates/fleet mgmt, consider migrating to swupdate. Otherwise, the built‑in A/B script is simpler.

## CAN / Serial / Diagnostics
- iproute2 + can‑utils (CAN bus)
- minicom (serial), strace (debug)

## Service Management (runtime)
- Start/stop via init scripts, e.g.:
  - `sh /etc/init.d/S03node-red start|stop|status`
  - `sh /etc/init.d/S98ttyd start|stop|status`
  - `sh /etc/init.d/S80dnsmasq start|stop|status`
- Disable a service persistently by removing/chmod‑x its script or moving to `.../init.d/disabled/` in the overlay at build time.

## Build-time Toggles (where to change)
- Buildroot defconfig: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig:1`
  - Node.js/Node‑RED: `BR2_PACKAGE_NODEJS=y`, `BR2_PACKAGE_NODEJS_NPM=y`, `BR2_PACKAGE_NODEJS_MODULES_ADDITIONAL="node-red@4.1.0"`
  - SSH: `BR2_PACKAGE_OPENSSH=y`
  - TTYD: `BR2_PACKAGE_TTYD=y`
  - Wi‑Fi/AP: `BR2_PACKAGE_WPA_SUPPLICANT=y`, `BR2_PACKAGE_HOSTAPD=y`, `BR2_PACKAGE_IW=y`, `BR2_PACKAGE_WIRELESS_TOOLS=y`
  - BT: `BR2_PACKAGE_BLUEZ5_UTILS=y`
  - MQTT: `BR2_PACKAGE_MOSQUITTO=y`
  - Media: `BR2_PACKAGE_FFMPEG=y`, `BR2_PACKAGE_LIVE555=y`
  - OTA: `BR2_PACKAGE_SWUPDATE=y`
  - PKG mgr: `BR2_PACKAGE_OPKG=y`

## Verify on Device
- List installed packages: `opkg list-installed` (if OPKG db is populated)
- Service status: `ps | grep node-red`, `ps | grep ttyd`
- Versions: `node -v`, `npm -v`, `node-red -v`; `ffmpeg -version`; `mosquitto -h`
- Network daemons: `pgrep -a dnsmasq`, `pgrep -a avahi-daemon`, `ntpq -p`

## Security Notes
- SSH: set strong passwords or keys; consider disabling password auth.
- TTYD: set `TTYD_CREDENTIAL` in `/etc/ttyd.conf` or disable TTYD if SSH is sufficient. Consider TLS termination if exposed.
- Avahi/dnsmasq: disable on untrusted networks unless required.
- OPKG: disable or restrict feeds on production devices.
- OTA: current script uses SHA256; prefer signing (or swupdate with signatures) for production.

## Open Decisions (fill in)
- Remote access: [ ] SSH only  [ ] TTYD only  [ ] Both (dev only)
- OTA path: [ ] Built‑in A/B  [ ] swupdate (signed)
- Wi‑Fi modes: [ ] STA  [ ] AP  [ ] Both
- Media stack: [ ] FFmpeg/Live555 needed  [ ] Remove
- MQTT: [ ] Required by flows  [ ] Remove

