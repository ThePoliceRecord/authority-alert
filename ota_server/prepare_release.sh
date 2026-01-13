#!/usr/bin/env bash
set -euo pipefail

# Stages the latest OTA zip into ota_content/releases/<version>/
# and generates a checksum manifest.
#
# Default: SHA256 manifest (newer reCamera OS)
# Optional: MD5 manifest + md5sum.txt inside the OTA zip (older upgrade.sh compatibility)
#
# Usage:
#   ./prepare_release.sh <version> [build_dir_or_zip] [--sha256|--md5|--both]
#
# Defaults:
#   build_dir = reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc

if [[ ${1-} == "" ]]; then
  echo "Usage: $0 <version> [build_dir_or_zip] [--sha256|--md5|--both]" >&2
  exit 1
fi

VERSION="$1"
shift

INPUT_PATH=""
MODE="sha256" # sha256 | md5 | both

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sha256) MODE="sha256"; shift ;;
    --md5) MODE="md5"; shift ;;
    --both) MODE="both"; shift ;;
    -h|--help)
      echo "Usage: $0 <version> [build_dir_or_zip] [--sha256|--md5|--both]" >&2
      exit 0
      ;;
    *)
      if [[ -z "$INPUT_PATH" ]]; then
        INPUT_PATH="$1"
        shift
      else
        echo "Unexpected argument: $1" >&2
        exit 2
      fi
      ;;
  esac
done

INPUT_PATH="${INPUT_PATH:-reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc}"
DEST_DIR="$(cd "$(dirname "$0")" && pwd)/ota_content/releases/${VERSION}"

mkdir -p "$DEST_DIR"

# Resolve OTA zip
OTA_ZIP=""

# Case 1: user passed a zip file
if [[ -f "$INPUT_PATH" && "$INPUT_PATH" == *.zip ]]; then
  OTA_ZIP="$INPUT_PATH"
fi

# Case 2: user passed a directory that contains zips
if [[ -z "$OTA_ZIP" && -d "$INPUT_PATH" ]]; then
  OTA_ZIP="$(ls -t "$INPUT_PATH"/*_emmc_ota.zip 2>/dev/null | head -n1 || true)"
fi

# Case 3: try to auto-detect latest zip under common output roots
if [[ -z "$OTA_ZIP" ]]; then
  OTA_ZIP="$(
    find reCamera-OS/output -type f -name '*_emmc_ota.zip' -printf '%T@ %p\n' 2>/dev/null |
      sort -nr | awk 'NR==1{print $2}'
  )"
fi

# Case 4: fallback to any known buildroot output
if [[ -z "$OTA_ZIP" && -d buildroot-2021.05/output ]]; then
  OTA_ZIP="$(
    find buildroot-2021.05/output -type f -name '*_emmc_ota.zip' -printf '%T@ %p\n' 2>/dev/null |
      sort -nr | awk 'NR==1{print $2}'
  )"
fi

if [[ -z "$OTA_ZIP" ]]; then
  echo "Could not locate an *_emmc_ota.zip. Specify a build directory or a zip explicitly." >&2
  echo "Example: $0 ${VERSION} reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc" >&2
  echo "Or:      $0 ${VERSION} /absolute/path/to/sg2002_reCamera_X.Y.Z_emmc_ota.zip" >&2
  exit 3
fi

staged_zip="$DEST_DIR/$(basename "$OTA_ZIP")"

echo "Staging: $OTA_ZIP -> $DEST_DIR/"
cp -f "$OTA_ZIP" "$staged_zip"

zip_has_entry() { # <zip> <entry>
  unzip -l "$1" "$2" >/dev/null 2>&1
}

ensure_md5sum_in_zip() { # <zip>
  local z="$1"

  if zip_has_entry "$z" "md5sum.txt"; then
    echo "md5sum.txt already present in $(basename "$z")"
    return 0
  fi

  command -v md5sum >/dev/null 2>&1 || {
    echo "ERROR: md5sum command not available" >&2
    return 1
  }

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' RETURN

  : >"$tmpdir/md5sum.txt"

  local files=(fip.bin boot.emmc rootfs_ext4.emmc)
  local any=0

  for f in "${files[@]}"; do
    if zip_has_entry "$z" "$f"; then
      any=1
      # Stream from the zip to avoid extracting large images to disk.
      local h
      h="$(unzip -p "$z" "$f" 2>/dev/null | md5sum | awk '{print $1}')"
      if [[ -z "$h" || ${#h} -ne 32 ]]; then
        echo "ERROR: failed to compute MD5 for $f inside $(basename "$z")" >&2
        return 1
      fi
      printf "%s  %s\n" "$h" "$f" >>"$tmpdir/md5sum.txt"
    fi
  done

  if [[ $any -eq 0 ]]; then
    echo "ERROR: no OTA payload files found in $(basename "$z") to hash" >&2
    return 1
  fi

  # Add md5sum.txt at zip root (same level as fip.bin/rootfs_ext4.emmc)
  (cd "$tmpdir" && zip -q -u "$z" md5sum.txt)
  echo "Injected md5sum.txt into $(basename "$z")"
}

pushd "$DEST_DIR" >/dev/null
case "$MODE" in
  sha256)
    echo "Generating SHA256 manifest: sg2002_recamera_emmc_sha256sum.txt"
    sha256sum *.zip > sg2002_recamera_emmc_sha256sum.txt
    ;;
  md5)
    ensure_md5sum_in_zip "$staged_zip"
    echo "Generating MD5 manifest: sg2002_recamera_emmc_md5sum.txt"
    md5sum *.zip > sg2002_recamera_emmc_md5sum.txt
    ;;
  both)
    ensure_md5sum_in_zip "$staged_zip"
    echo "Generating SHA256 manifest: sg2002_recamera_emmc_sha256sum.txt"
    sha256sum *.zip > sg2002_recamera_emmc_sha256sum.txt
    echo "Generating MD5 manifest: sg2002_recamera_emmc_md5sum.txt"
    md5sum *.zip > sg2002_recamera_emmc_md5sum.txt
    ;;
  *)
    echo "ERROR: unknown mode '$MODE' (expected: sha256|md5|both)" >&2
    exit 2
    ;;
esac
popd >/dev/null

echo "Done. Files in: $DEST_DIR"

echo
if [[ "$MODE" == "sha256" || "$MODE" == "both" ]]; then
  echo "Test URLs (replace <host> if remote):"
  echo "  http://localhost:8080/releases/${VERSION}/sg2002_recamera_emmc_sha256sum.txt"
  echo "  http://localhost:8080/releases/${VERSION}/$(basename "$OTA_ZIP")"
  echo
  echo "On device (SHA256 upgrader), set OTA source and run upgrade:"
  echo "  echo '1,http://<host>:8080/releases/${VERSION}/sg2002_recamera_emmc_sha256sum.txt' | sudo tee /etc/upgrade"
  echo "  sudo /mnt/system/upgrade.sh latest"
  echo "  sudo /mnt/system/upgrade.sh download"
  echo "  sudo /mnt/system/upgrade.sh start"
fi

if [[ "$MODE" == "md5" || "$MODE" == "both" ]]; then
  echo
  echo "MD5 compatibility mode enabled. Manifest generated: sg2002_recamera_emmc_md5sum.txt"
  echo "Test URL:"
  echo "  http://localhost:8080/releases/${VERSION}/sg2002_recamera_emmc_md5sum.txt"
fi
