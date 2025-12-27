#!/usr/bin/env bash
set -euo pipefail

# Stages the latest OTA zip into ota_content/releases/<version>/
# and generates sg2002_recamera_emmc_sha256sum.txt
#
# Usage:
#   ./prepare_release.sh <version> [build_dir]
#
# Defaults:
#   build_dir = reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc

if [[ ${1-} == "" ]]; then
  echo "Usage: $0 <version> [build_dir_or_zip]" >&2
  exit 1
fi

VERSION="$1"
INPUT_PATH="${2:-reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc}"
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

echo "Staging: $OTA_ZIP -> $DEST_DIR/"
cp -f "$OTA_ZIP" "$DEST_DIR/"

pushd "$DEST_DIR" >/dev/null
echo "Generating SHA256 manifest: sg2002_recamera_emmc_sha256sum.txt"
sha256sum *.zip > sg2002_recamera_emmc_sha256sum.txt
popd >/dev/null

echo "Done. Files in: $DEST_DIR"
echo
echo "Test URLs (replace <host> if remote):"
echo "  http://localhost:8080/releases/${VERSION}/sg2002_recamera_emmc_sha256sum.txt"
echo "  http://localhost:8080/releases/${VERSION}/$(basename "$OTA_ZIP")"
echo
echo "On device, set OTA source and run upgrade:"
echo "  echo '1,http://<host>:8080/releases/${VERSION}/sg2002_recamera_emmc_sha256sum.txt' | sudo tee /etc/upgrade"
echo "  sudo /mnt/system/upgrade.sh latest"
echo "  sudo /mnt/system/upgrade.sh download"
echo "  sudo /mnt/system/upgrade.sh start"
