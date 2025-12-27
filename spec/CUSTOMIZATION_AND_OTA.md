# Customization and OTA Plan

Owner: YOU
Last updated: YYYY-MM-DD

## 1) Goals
- Understand where and how to customize reCamera-OS.
- Define a simple, robust OTA update workflow and server layout.
- Track decisions, tasks, and open questions.

## 2) Repo Map (what affects customization)
- Build orchestration: `reCamera-OS/Makefile:1`, `reCamera-OS/docker_build.sh:1`
- Board targets (CV181x):
  - EMMC: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc:1`
    - Device tree: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/dts_riscv/sg2002_recamera_emmc.dts:1`
    - Partitioning: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/partition/partition_emmc.xml:1`
    - Board meta: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/config.json:1`
    - Rootfs extras: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/rootfs:1`
  - SD variants exist similarly under `.../sg2002_recamera_sd:1` and `.../sg2002_xiao_sd:1`.
- Rootfs overlay scripts (boot, OTA, overlay, USB networking):
  - `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/auto.sh:1`
  - `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:1`
  - `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/upgrade.sh:1`
  - `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/auto_download.sh:1`
  - `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/usb-ncm.sh:1`
- Bootloader hooks (A/B, boot env): `reCamera-OS/external/u-boot/include/configs/cv181x-asic.h:286`, `:330`, `:340`, `:342`
- Buildroot external packages: `reCamera-OS/external/br2-external:1`
  - Example packages: swupdate (`reCamera-OS/external/buildroot/package/swupdate/swupdate.mk:1`, `swupdate.sh:1`), opencv, nodejs, sscma-supervisor, sscma-node.
- Docs: OTA note in `reCamera-OS/README.md:64` and history in `reCamera-OS/CHANGELOG.md:51`, `:99`, `:208`.

## 3) How to Build + Where Artifacts Land
- Build examples (Docker): `./reCamera-OS/docker_build.sh sg2002_recamera_emmc`
- Native build (host): `make sg2002_recamera_emmc` from `reCamera-OS`
- Output: `reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc:1`
  - Typical artifacts: `*_emmc.zip`, `*_emmc_ota.zip`, `*_emmc_recovery.zip`, `*_emmc_sd_compat.zip`

## 4) Customization Areas (what you can change)
- Board/SoC config
  - Device tree: enable/disable peripherals, GPIOs, clocks
    - `.../dts_riscv/sg2002_recamera_emmc.dts:1`
  - Partition layout (sizes, A/B rootfs toggle): `.../partition_emmc.xml:1`
  - U-Boot env defaults and boot flow: `.../u-boot/include/configs/cv181x-asic.h:342`
- Kernel, U-Boot defconfigs
  - Defconfigs for kernel/u-boot are under the board dirs; adjust features or drivers as needed.
- Rootfs overlay and boot scripts
  - Persistent overlay dirs mounted from `/userdata` are managed by `rootfs_overlay.sh:1`.
  - First-boot/system hooks: modify `auto.sh:1`.
  - USB networking mode: `usb-ncm.sh:1` (default NCM), `usb-rndis.sh:1` also exists.
- Packages and apps
  - Add/enable packages via Buildroot external: `external/br2-external/*:1`, `external/buildroot/package/*:1`.
  - reCamera app packaging: `external/br2-external/reCamera/reCamera.mk:1`.
  - Node-RED / supervisor components under `sscma-*` packages.
- Branding/UI and defaults
  - Web UI/Node-RED flows (via packaged assets), SSH on/off, default services, etc.
- OTA behavior
  - On-device OTA script: `upgrade.sh:1` (URLs, A/B switching, integrity checks, recovery)
  - Auto-poller: `auto_download.sh:1`
  - Custom server toggle file: `.../rootfs/etc/upgrade:1` (see section 6)
  - swupdate (optional) is present but not auto-started by default; see `CHANGELOG.md:172` and `swupdate.mk:1`.

## 5) OTA Update: On-Device Flow (current)
- Partitions (A/B rootfs): `partition_emmc.xml:1`
  - `ROOTFS` and `ROOTFS2` support A/B; `USERDATA` holds overlays and staging.
- Script entrypoint: `upgrade.sh:1`
  - Commands:
    - `latest [url]` -> resolve SHA256 manifest URL from GitHub Release or custom URL.
    - `download` -> fetch `*_ota.zip` into recovery partition and verify sha256.
    - `start [zip]` -> write `fip.bin`, `boot.emmc` if needed, then write `rootfs_ext4.emmc` to inactive slot, switch `use_part_b`, reset counters.
    - `recovery` -> set `factory_reset=1` and exit; handled on next boot by `rootfs_overlay.sh:1`.
  - Integrity check: SHA256 comparison between zip's checksum file and streamed writes.
- Auto download helper: `auto_download.sh:1` loops to check latest, then download.
- Recovery + first-boot hooks: `rootfs_overlay.sh:1` can trigger factory reset or force write boot from a bundled `boot_ota.zip`.

Quick path: set a custom server without UI/token
- The supervisor reads the OTA channel from `/etc/recamera.conf/upgrade` (migrated from `/etc/upgrade`).
- Put the device on your LAN/USB subnet and run (on the device):

```
mkdir -p /etc/recamera.conf
echo '1,http://<host>:8080/releases/<ver>/sg2002_recamera_emmc_sha256sum.txt' > /etc/recamera.conf/upgrade
```

- Then run directly:

```
/mnt/system/upgrade.sh latest
/mnt/system/upgrade.sh download
/mnt/system/upgrade.sh start
```

Notes
- The file format is strict: `1,<full manifest URL>` (no spaces in the URL).
- The manifest is a text file containing `<HASH> <FILENAME>`:
  - `sg2002_recamera_emmc_sha256sum.txt` (SHA256, 64 hex characters)
- If you pass a URL ending with `.txt` to `latest`, it is used as‑is. Otherwise, `upgrade.sh` expects a GitHub Release URL and appends the manifest name.

Subnet reminder
- Some images ship USB‑NCM/AP on `192.168.42.1/24`; others use `192.168.16.1/24`.
- If you cannot reach the device, check `ip -br a` on the camera to confirm which subnet is active and adjust URLs accordingly.

## 6) OTA Artifact + Server Spec (simple, static)
- Artifact naming (example): `sg2002_reCamera_0.2.1_emmc_ota.zip`
  - Must contain at least: `rootfs_ext4.emmc`, `sha256sum.txt`
  - Optional: `fip.bin`, `boot.emmc` (bootloader/boot partition updates)
- Release manifest file (server-side):
  - `sg2002_recamera_emmc_sha256sum.txt`
  - Contains at least one line matching `.*ota.zip`
  - Format per `upgrade.sh:1` parser: `<HASH> <FILENAME>` (space separated)
    - SHA256 Example: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 sg2002_reCamera_0.2.1_emmc_ota.zip`
- Server layout (static HTTP is fine):
  - Directory: `https://your.server/reCamera/<version>/`
    - Files: `sg2002_recamera_emmc_sha256sum.txt`, `sg2002_reCamera_0.2.1_emmc_ota.zip`
  - Latest pointer options accepted by `upgrade.sh:1`:
    - A GitHub Release URL (auto-redirect, then `tag` -> `download` path rewrite).
    - Or a direct URL to `sg2002_recamera_emmc_sha256sum.txt`.
- Device configuration for custom server:
  - File: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/rootfs/etc/upgrade:1`
  - Content: `1,https://your.server/reCamera/0.2.1/sg2002_recamera_emmc_sha256sum.txt`
    - Format: `1,<url>` enables custom; `0` disables.
- Security notes:
  - **Current implementation uses SHA256.**
  - For production, consider adding optional signature verification in `upgrade.sh:1` or migrate to swupdate with signed images.

## 7) swupdate (optional path)
- Buildroot package present: `external/buildroot/package/swupdate/swupdate.mk:1`
- Entrypoint script: `external/buildroot/package/swupdate/swupdate.sh:1`
- Not auto-started by default (see `CHANGELOG.md:172`).
- If you prefer swupdate:
  - Define a `.swu` image format and server (HTTP or Suricatta-based polling).
  - Enable webserver or Suricatta in config and wire to your server endpoints.
  - Replace/augment `upgrade.sh:1` flow or keep both for flexibility.

## 8) Typical Customization Examples
- Change product name, LEDs, or peripheral mapping: edit DTS at `.../sg2002_recamera_emmc.dts:1`.
- Increase rootfs size or disable A/B: edit `partition_emmc.xml:1`.
- Add packages (e.g., mosquitto, opkg, your app): enable in Buildroot external and rebuild.
- Brand the WebUI/Node-RED: update packaged assets in the app packages, rebuild, flash or OTA.
- Default networking mode: adjust `usb-ncm.sh:1` or provide your own script.
- Point OTA to your server: set `/etc/upgrade` as described above.

## 9) Build and Release Process (OTA)
- Build: `make sg2002_recamera_emmc` -> produces `*_emmc_ota.zip`.
- Publish: upload zip and `sg2002_recamera_emmc_sha256sum.txt` to your server.
- On device CLI flow (manual):
  - `sudo /mnt/system/upgrade.sh latest <url>`
  - `sudo /mnt/system/upgrade.sh download`
  - `sudo /mnt/system/upgrade.sh start`  (writes inactive slot, flips boot)
  - Reboot.
- Automated: run `auto_download.sh:1` as a service to poll and prep.

## 10) Decisions, Tasks, Open Questions
- Decisions
  - OTA transport: [ ] GitHub Releases [ ] Static HTTP [ ] swupdate
  - Integrity: [x] SHA256 (current) [ ] Signed
  - Update style: [x] A/B rootfs [ ] In-place rootfs
- Tasks
  - [x] Migrate from MD5 to SHA256 integrity verification
  - [ ] Choose server option and publish a test release dir
  - [ ] Create and host `sg2002_recamera_emmc_sha256sum.txt`
  - [ ] Publish one `*_emmc_ota.zip` with valid `sha256sum.txt`
  - [ ] Set `/etc/upgrade` to custom and verify `latest`+`download`
  - [ ] Run `start`, confirm partition switch and rollback safety
  - [ ] Add OTA status to WebUI or logs, if desired
  - [ ] Optional: extend `upgrade.sh` to signature verification
- Open Questions
  - Do we need delta updates? (rdiff/rsync)
  - Do we need update staging approvals/device groups?
  - Should we prefer swupdate end-to-end for signing support?

## 11) References (quick)
- Readme (build, artifacts): `reCamera-OS/README.md:1`
- OTA script: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/upgrade.sh:1`
- Auto download: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/auto_download.sh:1`
- Overlay + recovery: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:1`
- Partitions (A/B): `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/partition/partition_emmc.xml:1`
- Custom server toggle: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/rootfs/etc/upgrade:1`
- U-Boot env/boot: `reCamera-OS/external/u-boot/include/configs/cv181x-asic.h:342`
- swupdate: `reCamera-OS/external/buildroot/package/swupdate/swupdate.mk:1`
