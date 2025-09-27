# Hardware & Deployment Appendix

Owner: YOU
Last updated: YYYY-MM-DD

## Device Overview
- Model: Authority Alert (based on Seeed reCamera platform, CV181x SoC).
- CPU: C906B (RISC-V), TPU up to 700 MHz.
- Memory: 256 MB DDR; Storage: 8 GB eMMC + optional SD.
- Connectivity: Wi-Fi (AP/STA), Ethernet, USB-NCM, CAN bus, Bluetooth.
- Sensors: camera module (ov5647/sc530ai detection).

## Power & Environment
- Power input: 5V via USB-C (confirm final spec) or PoE module (if installed).
- Operating temperature: 0°C to 50°C typical (verify hardware manual).
- Mounting: tripod mount / enclosure; ensure ventilation.

## Interfaces
- Ports:
  - USB-C (data/power) – default enumerates as NCM for configuration.
  - MicroSD (for recovery images).
  - GPIO headers for CAN/serial (see device tree `sg2002_recamera_emmc.dts`).
- Status LEDs: red/blue/white (configured via `dts`). Document meaning (boot, recording, AP mode).

## Default Network Behavior
- First boot: device hosts AP `AuthorityAlert-XXXX` (WPA2 key printed on label / config). IP `192.168.16.1`.
- USB network: enumerates as `usb0` with IP `192.168.42.1` for direct laptop connection.
- After OOBE network setup, device joins Wi-Fi/Ethernet while optionally keeping AP for fallback.

## Deployment Steps
1. Power device and connect via USB or AP.
2. Run OOBE wizard to set network, admin password, consent.
3. Mount camera and adjust field of view.
4. Verify status dashboard and AI detections.
5. Apply hardening steps (`spec/SECURITY_HARDENING.md`).
6. Document device info (serial, location, firmware version).

## Maintenance
- Schedule OTA updates per release cadence.
- Inspect physical mounting quarterly.
- Check storage usage (`df -h`) and clear old data if necessary.

## Recovery Tools
- Recovery SD image located at `output/<target>/install/..._recovery.zip`.
- USB console instructions: connect via UART header (if accessible) with 115200 baud.
- Reset button (if available) triggers factory reset (see `rootfs_overlay.sh` GPIO 510 check).

## Accessories
- Suggested: weatherproof enclosure, PoE injector (if using PoE module), secure mounting hardware, optional external storage.

