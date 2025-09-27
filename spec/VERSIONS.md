# Software Versions (Audit + Bump Guide)

Owner: YOU
Last updated: YYYY-MM-DD

Purpose
- Capture versions of key software in the image.
- Note where versions are pinned in the repo vs. inherited from Buildroot.
- Provide a quick on-device audit script and bump guidance.

## Known (pinned in this repo)
- Node.js: 22.8.0
  - File: `reCamera-OS/external/buildroot/package/nodejs/nodejs.mk:7`
- Node-RED: 4.1.0
  - File: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig:351`
- OpenSSH: 8.6p1
  - File: `reCamera-OS/external/buildroot/package/openssh/openssh.mk:7`
- libuv: 1.48.0
  - File: `reCamera-OS/external/buildroot/package/libuv/libuv.mk:9`
- c-ares: 1.32.2
  - File: `reCamera-OS/external/buildroot/package/c-ares/c-ares.mk:7`
- ICU: 73-2
  - File: `reCamera-OS/external/buildroot/package/icu/icu.mk:10`
- swupdate: 2022.12
  - File: `reCamera-OS/external/buildroot/package/swupdate/swupdate.mk:7`
- SSCMA Supervisor: 0.2.1
  - File: `reCamera-OS/external/br2-external/sscma-supervisor/sscma-supervisor.mk:7`
- SSCMA Node: 0.2.1
  - File: `reCamera-OS/external/br2-external/sscma-node/sscma-node.mk:7`
- Node-REDDash: 1.26.0 (as noted in changelog)
  - File: `reCamera-OS/CHANGELOG.md:11`

## Inherited from Buildroot (not pinned here)
The following are enabled in the Buildroot defconfig but their exact versions are inherited from the Buildroot baseline (vendor tree). Inspect the Buildroot package .mk files in the actual Buildroot source used at build time, or audit on-device.
- BusyBox (base OS)
- Coreutils, Util-linux, Kmod, Sudo
- dnsmasq, dhcpcd, NTP
- WPA Supplicant, hostapd, iw, wireless-tools
- BlueZ 5 utils
- Avahi (daemon, autoipd)
- Mosquitto
- FFmpeg, Live555
- OPKG
- ALSA (libs + tools)
- TTYD
- iproute2, can-utils
- Python 3 (runtime)

Note
- The vendor Buildroot used here is integrated during the build (see `reCamera-OS/buildroot-2021.05` marker and `external/setenv.sh` sync). Upstream package versions may not be visible in this repo snapshot; use the on-device audit or build logs to confirm.

## On-Device Version Audit
- Script: `spec/version_audit.sh`
  - Usage on device: `sh /mnt/system/spec/version_audit.sh` (or copy to device and run)
  - Prints versions for: BusyBox, Coreutils, Util-linux, Node, npm, Node-RED, OpenSSH, TTYD, curl, dnsmasq, NTP, Avahi, WPA Supplicant, hostapd, BlueZ, Mosquitto, FFmpeg, swupdate, OPKG.

If you prefer, I can bundle this script into the image (S99user or a CLI command) so you can run `version-audit` directly on devices.

## Bump Guidance (where to change)
- Node.js
  - File: `reCamera-OS/external/buildroot/package/nodejs/nodejs.mk`
  - Update `NODEJS_VERSION`, verify patches and V8 flags; re-test Node-RED and any native deps.
- Node-RED
  - File: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig`
  - Update `BR2_PACKAGE_NODEJS_MODULES_ADDITIONAL="node-red@<version>"`.
- OpenSSH
  - File: `reCamera-OS/external/buildroot/package/openssh/openssh.mk`
  - Update `OPENSSH_VERSION_MAJOR/MINOR` and check for patch updates.
- swupdate
  - File: `reCamera-OS/external/buildroot/package/swupdate/swupdate.mk`
  - Update `SWUPDATE_VERSION`, re-test web UI and handlers.
- libuv / c-ares / ICU
  - Files: respective `.../package/<pkg>/*.mk`; bump versions and re-test Node.js runtime.
- Others (dnsmasq, NTP, WPA/hostapd, BlueZ, FFmpeg, Avahi, Mosquitto, OPKG, ALSA, TTYD)
  - Bumped via the underlying Buildroot (vendor) tree. Plan to upgrade Buildroot baseline or vendor overlay for those.

Caveats
- Bumping foundational packages (BusyBox, util-linux, libc) can influence init, mount, and network behavior—test boot and services.
- FFmpeg/BlueZ/hostapd/wpa_supplicant bumps may require kernel config/driver alignment.
- swupdate bump may change config options; re-check `swupdate.config` and service args.

## Next Steps
- Run `spec/version_audit.sh` on a device and paste results here.
- Tell me which components you want to bump; I’ll prep targeted diffs and a validation checklist.

## Runtime Audit Snapshot (2025-09-27)
- BusyBox 1.33.0 (Buildroot build 2025-04-02)
- Coreutils 8.32
- util-linux 2.36.2
- Node.js 22.8.0 / npm 10.8.2
- Python 3.9.5
- OpenSSH 8.6p1 (OpenSSL 1.1.1k)
- ttyd 1.6.3
- curl 7.77.0 (OpenSSL 1.1.1k, c-ares 1.32.2)
- dnsmasq 2.85
- avahi-daemon 0.8
- wpa_supplicant 2.9 (hostapd binary not present)
- BlueZ bluetoothctl 5.58
- Mosquitto 2.0.10
- swupdate 2022.12
- OPKG 0.4.2
- FFmpeg not installed (binary missing)
- Live555 present in build, slated for removal. Migration to MediaMTX tracked in `spec/STREAMING_UPGRADE.md`.
