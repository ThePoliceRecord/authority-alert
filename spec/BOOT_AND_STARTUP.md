# Boot and Startup Flow (Deep Dive)

Owner: YOU
Last updated: YYYY-MM-DD

Purpose
- Explain exactly how the image boots, which applications/services start, in what order, and from where they are loaded.
- Provide precise file references so you can modify or hook into each phase confidently.

## Overview: Chain of Execution
- Boot ROM → FIP/U‑Boot from eMMC
- U‑Boot selects rootfs A/B and boots Linux
- Linux kernel hands off to BusyBox `init`
- BusyBox `inittab` runs sysinit steps (mounts, overlay setup, swap, hostname)
- BusyBox runs `rcS` which starts SXX init scripts in lexical order
- Late init scripts start applications (Node‑RED, dnsmasq, ttyd) and the user hook (`auto.sh`)

## Bootloader (U‑Boot)
- Boot command sequence picks SD fallback, update, A/B selection, then eMMC boot:
  - `CONFIG_BOOTCOMMAND`: `run sdboot || cvi_update || cvi_otp; run boot_record; run is_use_part_b; run emmcboot`
    - Ref: `reCamera-OS/external/u-boot/include/configs/cv181x-asic.h:342`
- Bootargs construction and SD path:
  - `SET_BOOTARGS`/`SD_BOOTM_COMMAND` set `root=` and `console=` parameters
    - Ref: `reCamera-OS/external/u-boot/include/configs/cv181x-asic.h:326`, `reCamera-OS/external/u-boot/include/configs/cv181x-asic.h:330`
- A/B selection uses env `use_part_b`; `upgrade.sh` flips it after writing the inactive slot
  - Flip happens in `switch_partition()` via `fw_setenv use_part_b <0|1>`
    - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/upgrade.sh:118`

## Kernel → /sbin/init (BusyBox)
- BusyBox init uses `/etc/inittab` and does not use runlevels.
- Inittab order (sysinit phase), key entries:
  - Mount proc, remount `/`, create `/dev/pts` and `/dev/shm`, `mount -a`
  - Run overlay/setup: `/mnt/system/rootfs_overlay.sh`
  - `swapon -a`, set stdio symlinks, set hostname
  - Start rc scripts: `/etc/init.d/rcS`
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/inittab:1`
- A getty is spawned on `ttyS0` for console login.

## Early System Setup: rootfs_overlay.sh
- Path executed by inittab: `/mnt/system/rootfs_overlay.sh`
  - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/inittab:28`
- Responsibilities:
  - Force‑write boot partitions if a packaged `boot_ota.zip` is present
    - Drops `boot_ota.zip` into `/mnt/system/resources/` at build time
      - Ref: `reCamera-OS/external/setenv.sh:204`
    - Calls `/mnt/system/upgrade.sh start /mnt/system/resources/boot_ota.zip`, reboots
      - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:53`
  - Factory reset trigger
    - Checks `fw_printenv factory_reset`; when `1`, runs `upgrade.sh start` to restore rootfs, wipes `/dev/mmcblk0p6` (userdata), clears flag, reboots
      - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:81`
  - Mount `/userdata` (format if needed), set up swap at `/userdata/.swapfile`
    - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:120`
  - Mount writable overlay for these directories: `/bin /etc /home /lib /opt /root /sbin /usr /var`
    - Each is mounted as OverlayFS with upper/workdirs under `/userdata/.overlay_fs`
    - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:32`

Notes
- This makes system customization persistent under `/userdata/.overlay_fs/...` without touching the base rootfs.

## rcS: Init Script Ordering (Services)
- BusyBox’s `/etc/init.d/rcS` executes all executable scripts in `/etc/init.d` starting with `S`, in lexical order (S00..S99). Buildroot also adds its own early scripts (e.g., udev, network) not overridden here.
- Custom/overlay scripts present and their roles:
  - S03node-red — start Node‑RED early
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S03node-red:1`
    - Runs `/usr/bin/node-red-pi` with `--max-old-space-size=64 --expose-gc` as a non‑root user (first non‑root shell user, typically `recamera`). Creates `/tmp/node-red/pid` and sets `--userDir /home/<user>/.node-red`.
  - S70hardware — load camera/kernel modules and USB gadget, set static IPs
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S70hardware:1`
    - Loads kernel modules via `/mnt/system/ko/loadsystemko.sh` (ISP/VI/TPU/codec/wifi drivers)
      - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/ko/loadsystemko.sh:1`
    - USB gadget networking: sources `/mnt/system/usb-net.sh` (symlink to `usb-ncm.sh` by default) which configures NCM on `usb0`
      - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/usb-ncm.sh:1`
    - Creates AP virtual iface `wlan1`, sets static IPs (`wlan1=192.168.16.1`, `usb0=192.168.42.1`)
  - S80dnsmasq — start dnsmasq with profile based on USB state and `wlan1`
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S80dnsmasq:1`
    - Picks one of: `/etc/dnsmasq/default.conf`, `usb_wlan1.conf`, or `usb_only.conf` depending on UDC state and `wlan1` address.
  - S98ttyd — start web terminal (ttyd)
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S98ttyd:1`
    - Defaults from `/etc/ttyd.conf` if present; vars include `TTYD_PORT`, `TTYD_USER`, `TTYD_CREDENTIAL`, etc.
  - S99user — final hook to run user automation
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S99user:1`
    - Sources and backgrounds `/userdata/auto.sh` if present, otherwise `/mnt/system/auto.sh`.

Additional default services
- Buildroot typically provides early `S10udev` (eudev), `S40network` (ifup), time seed, and other basics. These are not overridden by the overlay, so they run before S03.

## User/Application Hooks
- auto.sh (user and system)
  - System default: `/mnt/system/auto.sh` runs at S99 (backgrounded)
    - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/auto.sh:1`
    - Behavior:
      - Reset `boot_cnt` to 0 on successful boot (via `fw_setenv`)
      - Set rootfs RO: `rootfs_rw off`
      - If `/mnt/system/default_app` is executable, start it in background
  - User override: place `/userdata/auto.sh` to run your own startup logic instead of the system’s `auto.sh`

- Environment and PATH
  - `/etc/profile` extends PATH/LD_LIBRARY_PATH to include `/mnt/system/usr/{bin,sbin,lib}` so binaries and libs provided under `/mnt/system` are available
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/profile:1`

## USB Networking Details
- Hardware helper: `/etc/uhubon.sh` toggles hub GPIOs and inserts gadget/USB function modules from `/mnt/system/ko`
  - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/uhubon.sh:1`
- Gadget setup:
  - `/etc/run_usb.sh probe ncm` then `start` links functionfs nodes and binds UDC to expose `usb0`
    - Ref: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/run_usb.sh:1`

## OTA/Recovery Integration Points (during boot)
- `boot_ota.zip` autorun:
  - If present, `rootfs_overlay.sh` triggers `/mnt/system/upgrade.sh start /mnt/system/resources/boot_ota.zip`, removes it, and reboots
    - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:53`
- Factory reset via env flag:
  - If `factory_reset=1`, `rootfs_overlay.sh` performs recovery and data wipe, clears flag, and reboots
    - Ref: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:81`

## Execution Order Summary (first boot to apps)
1) U‑Boot selects A/B root, sets `bootargs`, boots kernel
2) BusyBox init sysinit:
   - Mounts -> `/mnt/system/rootfs_overlay.sh` -> swap/hostname -> `rcS`
3) rcS runs SXX scripts in order:
   - [Early Buildroot defaults: udev, network, etc.]
   - S03node‑red
   - S70hardware (drivers, USB gadget, static IFs)
   - S80dnsmasq (select config by USB/WLAN)
   - S98ttyd (web terminal)
   - S99user (user/system `auto.sh`)

## Where to Customize
- Very early boot (A/B/system recovery): edit `rootfs_overlay.sh`
  - Path: `reCamera-OS/external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/system/rootfs_overlay.sh:1`
- Kernel modules and hardware bring‑up: edit `S70hardware` and `loadsystemko.sh`
  - Paths: `.../overlay/etc/init.d/S70hardware:1`, `.../system/ko/loadsystemko.sh:1`
- Network behavior: `S80dnsmasq` and configs under `.../overlay/etc/dnsmasq/*.conf`
- Remote access: `S98ttyd` (and its `/etc/ttyd.conf`), OpenSSH stays enabled via Buildroot config
- App layer: `S03node-red` flags and user; add/remove services by dropping SXX scripts
- User logic: `/userdata/auto.sh` (preferred) or `/mnt/system/auto.sh`
- PATH/LD paths: `/etc/profile`

## Verification Tips
- Check init progress: `dmesg`, `/var/log/ttyd.log`, Node‑RED status via `sh /etc/init.d/S03node-red status`
- See which services are running: `ps`, `netstat -lpn` or `ss -lpn`
- Confirm overlays mounted: `mount | grep overlay`
- Confirm A/B target: `fw_printenv use_part_b`
- Confirm gadget net: `ip addr show usb0`, `/sys/class/udc/*/state`

