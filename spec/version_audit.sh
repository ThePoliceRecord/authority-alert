#!/bin/sh

# Version audit script for on-device use.
# Prints versions of common components if binaries are present.

OUTPUT_FILE=${OUTPUT_FILE:-/tmp/version_audit_$(date +%Y%m%d_%H%M%S).txt}
: >"$OUTPUT_FILE"

log() {
  printf '%s\n' "$1" | tee -a "$OUTPUT_FILE"
}

log "== Version Audit =="

try() {
  name="$1"; shift
  for cmd in "$@"; do
    if sh -c "$cmd" >/tmp/.va.out 2>/tmp/.va.err; then
      out=$(head -n1 /tmp/.va.out 2>/dev/null)
      [ -z "$out" ] && out=$(head -n1 /tmp/.va.err 2>/dev/null)
      [ -n "$out" ] && { log "$name: $out"; return 0; }
    fi
  done
  log "$name: not available"
  return 1
}

# Core/base
try BusyBox "busybox | head -n1"
try Coreutils "ls --version | head -n1"
try UtilLinux "lsblk --version | head -n1" "mount --version | head -n1"

# Langs
try Node "node -v"
try NPM "npm -v"
try NodeRED "node-red --version" "node-red -V" "node-red-pi --version" "node-red-pi -V"
try Python3 "python3 --version"

# Remote access
try OpenSSH "ssh -V"
try TTYD "ttyd -v"

# Networking
try Curl "curl --version | head -n1"
try Dnsmasq "dnsmasq -v | head -n1"
try NTP "ntpd -V" "ntpd -v" "ntpq -V"
try Avahi "avahi-daemon -V" "avahi-daemon -v"

# Wi-Fi / BT
try WPA_Supplicant "wpa_supplicant -v"
try Hostapd "hostapd -v"
try BlueZ "bluetoothctl -v"

# Messaging
try Mosquitto "mosquitto -h | head -n1" "mosquitto -v | head -n1"

# Media
try FFmpeg "ffmpeg -version | head -n1"

# OTA
try swupdate "/usr/bin/swupdate -V" "/usr/bin/swupdate -v" "/usr/bin/swupdate -h | head -n1"

# Package manager
try OPKG "opkg --version" "opkg -V"

rm -f /tmp/.va.out /tmp/.va.err 2>/dev/null
log "== Done =="
log "Report saved to $OUTPUT_FILE"
