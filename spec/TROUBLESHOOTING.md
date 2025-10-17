# Troubleshooting – OTA, Network, UI/Streams

Owner: Authority Alert Team
Last updated: 2025-10-17

This page captures the common field issues we’ve seen and the quickest working fixes.

## OTA: set server and run without the UI
- Symptom: UI rejects the URL or API returns 401.
- Fix (device shell):

```
mkdir -p /etc/recamera.conf
echo '1,http://<host>:8080/releases/<ver>/sg2002_recamera_emmc_md5sum.txt' > /etc/recamera.conf/upgrade
/mnt/system/upgrade.sh latest
/mnt/system/upgrade.sh download
/mnt/system/upgrade.sh start
```

Notes
- The manifest is a plain text file `sg2002_recamera_emmc_md5sum.txt` with `<MD5> <FILENAME>` and no extra spaces.
- `latest` accepts the manifest URL directly; if a non‐`.txt` URL is used, it tries a GitHub‑style redirect.

## Network: can’t reach the camera
- Some builds use USB‑NCM/AP on `192.168.42.1/24`, others on `192.168.16.1/24`.
- On the camera, run `ip -br a` to confirm which subnet is active.
- USB‑NCM quick bring‑up (if DHCP is missing):

```
IF=$(ls /sys/class/net | grep -E 'usb|ncm' | head -n1)
ip addr flush dev "$IF" || true
ip addr add 192.168.16.1/24 dev "$IF"
ip link set "$IF" up
dnsmasq --interface="$IF" --bind-interfaces \
        --dhcp-range=192.168.16.50,192.168.16.150,12h \
        --dhcp-option=3,192.168.16.1 --dhcp-option=6,1.1.1.1
```

## Boot logo not visible
- U‑Boot logo is gated by `ENABLE_BOOTLOGO` in the build.
- Build with: `ENABLE_BOOTLOGO=1 ./docker_build.sh sg2002_recamera_emmc`.
- Replace the image at `reCamera-OS/external/build/tools/common/bootlogo/logo.jpg` if you need a custom logo.

## UI: Node‑RED/supervisor connection drops
- Gather logs (device): `logread | tail -n 200` and `pgrep -a node` to confirm the process.
- Check CPU/RAM pressure: `top`, `free -m`. Disable heavy flows temporarily by moving `/home/recamera/.node-red/flows.json` aside and restarting supervisor.
- If it keeps flapping, capture `/tmp/supervisor/*.log` (if present) and `dmesg | tail -n 100` for driver resets.

## Choppy live stream
- First sanity: move the device and client to the same AP or wire; 2.4GHz congestion shows up as stutter.
- Reduce encoder load: start with 1280x720 @ 15fps, moderate bitrate (2–3 Mbps). If using flows, adjust the camera node to lower resolution/FPS.
- Watch CPU while streaming (`top`); if pegged, the stream will be uneven. Also try RTSP over LAN instead of Wi‑Fi to isolate RF.

## Verify OTA artifacts quickly
- Manifest reachable: `curl -I http://<host>:8080/releases/<ver>/sg2002_recamera_emmc_md5sum.txt`
- OTA zip size: `curl -sI http://<host>:8080/releases/<ver>/<zip> | awk '/Content-Length/ {printf "%.1f MB\n",$2/1048576}'`

