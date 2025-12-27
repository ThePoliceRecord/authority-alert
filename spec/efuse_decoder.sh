#!/bin/bash
# eFuse Decoder for CV181x/CV180x
# Reads and displays all eFuse data in human-readable format

set -euo pipefail

EFUSE_SHADOW="/sys/class/cvi-base/base_efuse_shadow"
EFUSE_UID="/sys/class/cvi-base/base_uid"

if [ ! -f "$EFUSE_SHADOW" ]; then
    echo "Error: $EFUSE_SHADOW not found"
    echo "Are you running this on a CV181x/CV180x device?"
    exit 1
fi

echo "================================"
echo "  eFuse Decoder for CV181x/CV180x"
echo "================================"
echo ""

# Helper function to read bytes from efuse at specific offset
read_efuse_hex() {
    local offset=$1
    local count=$2
    dd if="$EFUSE_SHADOW" bs=1 skip=$offset count=$count 2>/dev/null | hexdump -ve '/1 "%02X"'
}

# Helper function to read bytes and format as hex with separators
read_efuse_hex_sep() {
    local offset=$1
    local count=$2
    local sep=$3
    dd if="$EFUSE_SHADOW" bs=1 skip=$offset count=$count 2>/dev/null | hexdump -ve '/1 "%02X'"$sep"''
}

# 1. Chip UID (constructed from FTSN3 and FTSN4)
echo "## Chip UID"
if [ -f "$EFUSE_UID" ]; then
    EFUSE_UID_VAL=$(cat "$EFUSE_UID")
    echo "$EFUSE_UID_VAL"
else
    # Manual construction from offsets 0x0C-0x13
    FTSN=$(read_efuse_hex $((0x0C)) 8)
    echo "UID: ${FTSN:0:8}_${FTSN:8}"
fi
echo ""

# 2. MAC Address (offset 0x40, 6 bytes)
echo "## MAC Address"
echo "Offset: 0x40, Size: 6 bytes"
MAC=$(dd if="$EFUSE_SHADOW" bs=1 skip=$((0x40)) count=6 2>/dev/null | hexdump -ve '/1 "%02x:"')
echo "Value:  ${MAC%:}"
echo "(Also available via: fw_printenv ethaddr)"
echo ""

# 3. Serial Number (offset 0x48, 9 bytes - formatted as hex string)
echo "## Serial Number (USER Area)"
echo "Offset: 0x48, Size: 9 bytes"
SN=$(dd if="$EFUSE_SHADOW" bs=1 skip=$((0x48)) count=9 2>/dev/null | hexdump -ve '/1 "%02X"')
echo "Value:  $SN"
echo "(Also available via: fw_printenv sn)"
echo ""

# 4. Chip SN (offset 0x0C, 8 bytes)
echo "## Chip SN (FTSN3+FTSN4)"
echo "Offset: 0x0C, Size: 8 bytes"
CHIP_SN=$(read_efuse_hex $((0x0C)) 8)
echo "Value:  $CHIP_SN"
echo ""

# 5. Device ID (offset 0x8C, 8 bytes)
echo "## Device ID"
echo "Offset: 0x8C, Size: 8 bytes"
DEVICE_ID=$(read_efuse_hex $((0x8C)) 8)
echo "Value:  $DEVICE_ID"
echo ""

# 6. Full USER area dump (offset 0x40, 40 bytes)
echo "## USER Area (Full Dump)"
echo "Offset: 0x40, Size: 40 bytes"
echo "Layout:"
echo "  0x40-0x45: MAC address (6 bytes)"
echo "  0x46-0x47: Reserved (2 bytes)"
echo "  0x48-0x50: Serial Number (9 bytes)"
echo "  0x51-0x67: Reserved (23 bytes)"
echo ""
echo "Hex Dump:"
dd if="$EFUSE_SHADOW" bs=1 skip=$((0x40)) count=40 2>/dev/null | hexdump -C
echo ""

# 7. Security Configuration
echo "## Security Configuration"
echo "Offset: 0xA0, Size: 4 bytes"
SEC_CONF=$(read_efuse_hex $((0xA0)) 4)
echo "Value:  0x$SEC_CONF"
# Check secure boot enable bit (bit 0)
SEC_VAL=$((0x$SEC_CONF))
if [ $((SEC_VAL & 0x3)) -ne 0 ]; then
    echo "Status: Secure Boot ENABLED"
else
    echo "Status: Secure Boot DISABLED"
fi
echo ""

# 8. Lock Status (offset 0xF8, 4 bytes)
echo "## Lock Status"
echo "Offset: 0xF8, Size: 4 bytes"
LOCK_STATUS=$(read_efuse_hex $((0xF8)) 4)
LOCK_VAL=$((0x$LOCK_STATUS))
echo "Value:  0x$LOCK_STATUS (decimal: $LOCK_VAL)"
echo ""
echo "Lock Bit Layout (each area has write lock + read lock bit pair):"
echo "  Bits  0-1:  HASH0_PUBLIC write lock"
echo "  Bits  4-5:  LOADER_EK write lock"
echo "  Bits  6-7:  DEVICE_EK write lock"
echo "  Bits  8-9:  HASH0_PUBLIC read lock"
echo "  Bits 12-13: LOADER_EK read lock"
echo "  Bits 14-15: DEVICE_EK read lock"
echo ""
echo "Lock Status (0x3 = locked, 0x0 = unlocked):"
printf "  HASH0_PUBLIC write: 0x%x %s\n" $(((LOCK_VAL >> 0) & 0x3)) \
    "$([ $(((LOCK_VAL >> 0) & 0x3)) -eq 3 ] && echo '[LOCKED]' || echo '[unlocked]')"
printf "  LOADER_EK write:    0x%x %s\n" $(((LOCK_VAL >> 4) & 0x3)) \
    "$([ $(((LOCK_VAL >> 4) & 0x3)) -eq 3 ] && echo '[LOCKED]' || echo '[unlocked]')"
printf "  DEVICE_EK write:    0x%x %s\n" $(((LOCK_VAL >> 6) & 0x3)) \
    "$([ $(((LOCK_VAL >> 6) & 0x3)) -eq 3 ] && echo '[LOCKED]' || echo '[unlocked]')"
printf "  HASH0_PUBLIC read:  0x%x %s\n" $(((LOCK_VAL >> 8) & 0x3)) \
    "$([ $(((LOCK_VAL >> 8) & 0x3)) -eq 3 ] && echo '[LOCKED]' || echo '[unlocked]')"
printf "  LOADER_EK read:     0x%x %s\n" $(((LOCK_VAL >> 12) & 0x3)) \
    "$([ $(((LOCK_VAL >> 12) & 0x3)) -eq 3 ] && echo '[LOCKED]' || echo '[unlocked]')"
printf "  DEVICE_EK read:     0x%x %s\n" $(((LOCK_VAL >> 14) & 0x3)) \
    "$([ $(((LOCK_VAL >> 14) & 0x3)) -eq 3 ] && echo '[LOCKED]' || echo '[unlocked]')"
echo ""
echo "Note: Locks are one-time programmable (OTP) - once set, cannot be undone"
echo ""

# 9. Summary from U-Boot environment (if available)
echo "## U-Boot Environment Variables"
if command -v fw_printenv >/dev/null 2>&1; then
    echo "ethaddr: $(fw_printenv ethaddr 2>/dev/null | awk -F'=' '{print $NF}' || echo 'not set')"
    echo "sn:      $(fw_printenv sn 2>/dev/null | awk -F'=' '{print $NF}' || echo 'not set')"
else
    echo "(fw_printenv not available)"
fi
echo ""

echo "================================"
echo "Script completed successfully"
echo "================================"
