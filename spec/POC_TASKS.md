## POC Task List – The Authority Alert (TAA)

Owner: Authority Alert Team
Last updated: 2025-10-14

Goal
- Ship a branded “lipstick” POC build with OTA delivery using the existing reCamera-OS pipeline.

Branding
- Product name: The Authority Alert
- Short name: TAA
- Tagline: Open-source camera programmed with TPR goodness
 - Colors: match The Police Record site (https://thepolicerecord.com) — see UI_THEME.md “TAA Theme”.

Scope (POC)
- Rename strings, titles, hostnames, and mDNS only (no feature changes).
- Provide web/UI logos and favicon; keep UI mostly stock.
- Produce one OTA package and host it on the included local OTA server.

Tasks
- Names and strings
  - Change hostname: `reCamera-OS/external/buildroot/configs/cvitek_CV181X_musl_riscv64_defconfig:BR2_TARGET_GENERIC_HOSTNAME` -> `taa`.
  - Change device name file: `reCamera-OS/external/build/boards/cv181x/sg2002_recamera_emmc/rootfs/etc/device-name` -> `The Authority Alert`.
  - mDNS host: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/avahi/avahi-daemon.conf:host-name` -> `taa`.
  - Node‑RED editor title: `.../node-red/settings.js:title` -> `The Authority Alert`.
- Logos and theme
  - Provide logo assets per “Logo Specs” below (web/UI only).
  - Web favicon and header logo for Node‑RED UI.
  - Optional: simple CSS color token update in `custom.css`.
- OTA packaging and server
  - Build: `./reCamera-OS/docker_build.sh sg2002_recamera_emmc`.
  - Stage and host: `ota_server/prepare_release.sh <version>` then `docker compose up -d` in `ota_server/`.
  - Device upgrade: set `/etc/upgrade` to server manifest and run `upgrade.sh`.
- Verification
  - Boot: confirm hostname and mDNS announce as `taa`.
  - Web UI: title shows “The Authority Alert”, favicon/logo present.
  - OTA: download + start succeeds, A/B switch works, preserve settings.

Logo Specs (POC-ready)
- Web/UI logos
  - App header logo (light background): SVG preferred, fallback PNG 512x128.
  - App header logo (dark background): SVG preferred, fallback PNG 512x128 (inverted/white).
  - Square icon: PNG 512x512 (used in app cards and OpenGraph).
  - Favicon: provide `favicon.svg` and PNG fallbacks 48x48, 32x32, 16x16.
  - Safe zones: keep logotype legible at 24 px height.
- File naming suggestion
  - `taa-logo-dark.svg`, `taa-logo-light.svg`
  - `taa-logo-dark.png`, `taa-logo-light.png` (512x128)
  - `taa-icon-512.png`, `favicon.svg`, `favicon-48.png`, `favicon-32.png`, `favicon-16.png`

Copy Points (for UI/about)
- Long: “The Authority Alert (TAA) is an open-source camera programmed with TPR goodness.”
- Short: “Open-source camera, TPR powered.”

Out of Scope (POC)
- swupdate migration or signing changes (stay on MD5 manifest).
- Deep Node‑RED theme overhaul; only title/logo/optional colors.
- Kernel/driver changes, new services, or storage layout changes.
- U‑Boot branding and boot logo changes.

Open Questions
- Do we want the boot logo shown at all on headless units? If not, skip U‑Boot logo asset.
- Preferred primary color for accenting the UI?
- Final domain/host for OTA beyond local testing?

Next Steps
- Provide the logo files listed above.
- I can apply the string changes and wire header logo + favicon placements next.
