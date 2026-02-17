#!/bin/bash

OFFICIAL_URL="https://github.com/ThePoliceRecord/authority-alert-OS/releases/latest"

# Manifest filename - SHA256 only
SHA256_FILE=sg2002_recamera_emmc_sha256sum.txt
URL_FILE=url.txt

# Hash algorithm - SHA256 only
HASH_CMD=""        # Command to use (sha256sum)
HASH_FILE="$SHA256_FILE"
HASH_TYPE="sha256"

FIP_PART=/dev/mmcblk0boot0
BOOT_PART=/dev/mmcblk0p1
RECV_PART=/dev/mmcblk0p5
ROOTFS=/dev/mmcblk0p3
ROOTFS_B=/dev/mmcblk0p4

UPGRADE_TMP=/userdata/.upgrade
UPGRADE_FILES=/tmp/upgrade
mkdir -p "$UPGRADE_FILES" "$UPGRADE_TMP"

# global variable
RunCase=""
ResultFile=""
MountPath=""
Step=0
DD_Calc_Hash=""    # Result from dd_calc_hash()

step_log() {
    let Step++
    echo "Step$Step: $1"
}

step_result() { echo "Result: $1"; }

# Detect available hash algorithm - SHA256 only
detect_hash_algorithm() {
    if command -v sha256sum >/dev/null 2>&1; then
        HASH_CMD="sha256sum"
        return 0
    elif busybox sha256sum --help >/dev/null 2>&1; then
        HASH_CMD="busybox sha256sum"
        return 0
    else
        echo "ERROR: sha256sum command not available"
        exit 1
    fi
}

# Initialize hash algorithm detection
detect_hash_algorithm

# exit
cleanup() {
    sync
    [ -n "$MountPath" ] && [ -d "$MountPath" ] && {
        umount "$MountPath" 2>/dev/null
        ! mount | grep -qw "$MountPath" && rm -rf "$MountPath"
    }
    rm -f "$ResultFile.mutex" 2>/dev/null
}
trap cleanup SIGINT SIGTERM

exit_upgrade() {
    [ -z "$1" ] && {
        [ -f "$ResultFile" ] || echo "Success" >"$ResultFile"
        echo "Success"
        cleanup
        exit 0
    }
    echo "Failed: $1" | tee "$ResultFile"
    cleanup
    exit 1
}

# ps
ps_mutex() {
    [ -f "$ResultFile.mutex" ] && {
        echo "./upgrade.sh $RunCase is running."
        exit 1
    }
    Step=0
    rm -f ${ResultFile}*
    echo $$ >"$ResultFile.mutex"
}

kill_ps() {
    local pid=$(ps | grep "$1" | grep -v grep | awk '{print $1}')
    [ -n "$pid" ] && kill -9 "$pid"
}

ps_stop() {
    kill_ps "dd"
    kill_ps "unzip -p"
    kill_ps "wget -T 10 -t 3 --no-check-certificate"
    killall $(basename "$0")
    echo "stopped"
    exit 0
}

# utils - generic hash parsing
parse_hash() { # <field> <manifest_file>
    local info=$(grep ".*ota.zip" "$2" 2>/dev/null)
    [ -z "$info" ] && return 1
    case "$1" in
    name) echo "$info" | awk '{print $2}' ;;
    hash) echo "$info" | awk '{print $1}' ;;
    os) echo "$info" | awk '{print $2}' | cut -d'_' -f2 ;;
    version) echo "$info" | awk '{print $2}' | cut -d'_' -f3 ;;
    *) return 1 ;;
    esac
}

zip_get_size() { # zip=$1 file=$2
    local size=$(unzip -l "$1" 2>/dev/null | grep "$2" | awk '{print $1}')
    [ -z "$size" ] && return 1
    echo $((size))
}

# Read hash from inside zip - SHA256 only
zip_read_hash() { # zip=$1 file=$2
    local zip=$1 file=$2 hash=""
    
    hash=$(unzip -p "$zip" sha256sum.txt 2>/dev/null | grep "$file" | awk '{print $1}')
    if [ -n "$hash" ] && [ ${#hash} -eq 64 ]; then
        echo "$hash"
        return 0
    fi
    
    return 1
}

mount_recovery() {
    local fs_type=$(blkid -o value -s TYPE "$RECV_PART")
    [ "$fs_type" != "ext4" ] && {
        mkfs.ext4 "$RECV_PART" || exit_upgrade "format recovery partition"
        fs_type=$(blkid -o value -s TYPE "$RECV_PART")
        [ "$fs_type" != "ext4" ] && exit_upgrade "recovery partition is not ext4!"
    }
    MountPath=$(mktemp -d)
    mount "$RECV_PART" "$MountPath" && return 0
    exit_upgrade "mount $RECV_PART on $MountPath failed."
}

is_rootfs_b() {
    local root_dev=$(mountpoint -n / | awk '{print $1}') || exit_upgrade "get rootfs"
    [ "$root_dev" = "$(realpath "$ROOTFS_B")" ] && return 1 || return 0
}

set_bootenv() {
    fw_setenv "$1" "$2" || exit_upgrade "set $1=$2"
}

switch_partition() {
    step_log "Switch rootfs partition"
    is_rootfs_b
    local part_b=$?
    part_b=$((1 - part_b))
    set_bootenv use_part_b $part_b
    set_bootenv boot_cnt 0
    set_bootenv boot_failed_limits 5
    set_bootenv boot_rollback
    step_log "Please reboot to take effect."
}

get_upgrade_url() {
    local url=$1 full_url=$url
    [[ $url =~ .*\.txt$ ]] || {
        url=$(curl -skLi "$url" --connect-timeout 30 --max-time 60 | grep -i '^location:' | awk '{print $2}' | sed 's/^"//;s/"$//')
        url=$(echo "$url" | sed 's/tag/download/g')
        [ -z "$url" ] && return 1
        
        # Use SHA256 manifest
        full_url="$url/$SHA256_FILE"
    }
    echo "$full_url"
}

wget_file() {
    local url="$1" file="$2" size
    step_log "Download $url"
    local tmpfile=$(mktemp)
    wget -T 10 -t 3 --no-check-certificate --spider "$url" -o "$tmpfile"
    size="$(grep -i 'Length' "$tmpfile" | awk '{print $2}')"
    rm -f "$tmpfile"
    [ -z "$size" ] && exit_upgrade "get size $(basename $file)"
    echo "$file" >"$ResultFile.file"
    echo "$size" >"$ResultFile.size"

    wget -T 10 -t 3 --no-check-certificate -q -c --show-progress "$url" -O "$file" -o /dev/null || exit_upgrade "download $url"
    [ ! -s "$file" ] && exit_upgrade "download $(basename $file) is empty"
}

# Generic hash write with verification
zip_write_part() {
    local zip=$1 part=$2 file=$3
    local size read_hash calc_hash
    size=$(zip_get_size "$zip" "$file") || exit_upgrade "get size $zip $file"
    read_hash=$(zip_read_hash "$zip" "$file") || exit_upgrade "read hash $zip $file"
    step_log "Write $part with $file size: $size (using $HASH_TYPE)"
    echo "$size" >"$ResultFile.$file.size"

    local tmpfile=$(mktemp)
    unzip -p "$zip" "$file" 2>/dev/null | tee >($HASH_CMD >"$tmpfile") | dd of="$part" bs=1M status=progress 2>&1 | tee -a "$ResultFile.$file.prog"
    local ret=$?
    calc_hash=$(cat $tmpfile | awk '{print $1}')
    rm -rf $tmpfile
    [ $ret -ne 0 ] && exit_upgrade "write $part with $file"

    step_log "Check $HASH_TYPE $part"
    [ "$calc_hash" = "$read_hash" ] || exit_upgrade "check $HASH_TYPE $part (expected: $read_hash, got: $calc_hash)"
}

# Generic hash calculation
dd_calc_hash() {
    DD_Calc_Hash=""
    local file=$1 size=$2
    [ -e "$file" ] || exit_upgrade "file $file not exist"
    [ -z "$size" ] && size=$(stat -c %s "$file" 2>/dev/null) || size=$2
    [ $((size)) -eq 0 ] && exit_upgrade "unknown size $file"
    local tmpfile=$(mktemp)
    dd if="$file" bs=1M status=progress 2>/dev/null | head -c $size | $HASH_CMD >"$tmpfile"
    local ret=$?
    DD_Calc_Hash=$(cat $tmpfile | awk '{print $1}')
    rm -f $tmpfile
    [ $ret -ne 0 ] && exit_upgrade "calc $HASH_TYPE $file"
}

ota_boot() { # fip.bin boot.emmc
    local zip=$1 part=$2 file=$3
    local size read_hash calc_hash
    step_log "Check $part and $file"
    size=$(zip_get_size $zip $file)
    read_hash=$(zip_read_hash $zip $file)
    ([ -z "$size" ] || [ -z "$read_hash" ]) && {
        step_result "skip with no valid $file"
        return 0
    }

    dd_calc_hash $part $size
    calc_hash=$DD_Calc_Hash
    [ "$read_hash" = "$calc_hash" ] && {
        step_result "skip with $HASH_TYPE matched"
        return 0
    }

    calc_hash=$(unzip -p $zip $file 2>/dev/null | $HASH_CMD | awk '{print $1}')
    [ "$calc_hash" != "$read_hash" ] && {
        step_result "skip with $file damaged"
        return 0
    }

    [ "$part" = "$FIP_PART" ] && echo 0 >"/sys/block/mmcblk0boot0/force_ro" 2>/dev/null
    unzip -p "$zip" "$file" 2>/dev/null | dd of="$part" bs=1M status=progress || exit_upgrade "write $part with $file"
    calc_hash=$(dd if="$part" bs=1M count=$(( (size + 1048575) / 1048576 )) 2>/dev/null | head -c $size | $HASH_CMD | awk '{print $1}')
    [ "$calc_hash" != "$read_hash" ] && exit_upgrade "check $HASH_TYPE $part"
    [ "$part" = "$FIP_PART" ] && echo 1 >"/sys/block/mmcblk0boot0/force_ro" 2>/dev/null
}

#####
latest_cmd() {
    local cmd=$2
    [ "$cmd" = "x" ] && ps_stop
    [ "$cmd" = "q" ] && {
        [ -f $ResultFile ] && {
            cat $ResultFile
            exit 0
        }
        [ ! -f "$ResultFile.mutex" ] && {
            echo "Stopped"
            exit 0
        }
        exit 0
    }
}

latest() {
    latest_cmd $@
    ps_mutex
    local url="$2" hash_url=""
    if [ -z "$url" ]; then
        url="$OFFICIAL_URL"
        step_log "Parse $url"
        hash_url=$(get_upgrade_url "$url") || exit_upgrade "parse $url"
        step_result "$hash_url (using $HASH_TYPE)"
    else
        step_log "Parse $url"
        local hash_path
        hash_url=$(get_upgrade_url "$url") || exit_upgrade "parse $url"
        step_result "$hash_url (using $HASH_TYPE)"
    fi

    local hash_path="$UPGRADE_FILES/$HASH_FILE"
    rm -f "$hash_path"
    wget_file "$hash_url" "$hash_path"

    # RESULT
    local os_name version
    os_name=$(parse_hash os "$hash_path") || exit_upgrade "parse os $hash_path"
    version=$(parse_hash version "$hash_path") || exit_upgrade "parse version $hash_path"
    echo "Success: $os_name $version ($HASH_TYPE)" >$ResultFile
    echo "$(echo ${hash_url%/*})" >"$UPGRADE_FILES/$URL_FILE"
    step_result "$os_name@$version (integrity: $HASH_TYPE)"
    exit_upgrade
}

download_cmd() {
    local cmd=$2
    [ "$cmd" = "x" ] && ps_stop
    [ "$cmd" = "q" ] && {
        [ -f $ResultFile ] && {
            cat $ResultFile
            exit 0
        }
        [ ! -f "$ResultFile.mutex" ] && {
            echo "Stopped"
            exit 0
        }

        local total=$(cat "$ResultFile.size" 2>/dev/null)
        local file=$(cat "$ResultFile.file" 2>/dev/null)
        local size=$(stat -c %s "$file" 2>/dev/null)
        echo "Progress: $((size)) $((total))"
        exit 0
    }
    [ ! -z "$cmd" ] && {
        echo "Usage: $0 $RunCase [q]"
        exit 1
    }
}

download() {
    download_cmd $@
    ps_mutex
    mount_recovery

    local tmpdir="$UPGRADE_TMP"
    local url_path_tmp="$tmpdir/$URL_FILE"
    
    local hash_path_tmp="$tmpdir/$HASH_FILE"
    local hash_path_latest="$UPGRADE_FILES/$HASH_FILE"
    
    step_log "Check files (using $HASH_TYPE)"
    [ -s $hash_path_latest ] && {
        local hash_path_now="$MountPath/$HASH_FILE"
        [ -s "$hash_path_now" ] && {
            [ -z "$(diff "$hash_path_latest" "$hash_path_now")" ] && {
                step_result "OTA is up to date."
                exit_upgrade
            }
        }
        ([ ! -s "$hash_path_tmp" ] || [ ! -z "$(diff "$hash_path_latest" "$hash_path_tmp")" ]) && {
            step_log "Copy latest files"
            rm -rf $tmpdir/*
            cp -f "$hash_path_latest" "$hash_path_tmp"
            cp -f "$UPGRADE_FILES/$URL_FILE" "$url_path_tmp"
        }
    }
    ([ -s "$hash_path_tmp" ] && [ -s "$url_path_tmp" ]) || {
        exit_upgrade "no latest files, please run 'upgrade.sh latest [url]' first"
    }

    local filename hash url
    filename=$(parse_hash name "$hash_path_tmp") || exit_upgrade "parse name $hash_path_tmp"
    hash=$(parse_hash hash "$hash_path_tmp") || exit_upgrade "parse hash $hash_path_tmp"
    url=$(cat "$url_path_tmp")/$filename
    wget_file "$url" "$tmpdir/$filename"

    step_log "Check $HASH_TYPE $filename"
    dd_calc_hash "$tmpdir/$filename"
    [ "$hash" != "$DD_Calc_Hash" ] && exit_upgrade "$HASH_TYPE mismatch (expected: $hash, got: $DD_Calc_Hash)"

    step_log "Sync files"
    rm -rf $MountPath/*
    cp -f $tmpdir/*.zip $MountPath/ || exit_upgrade "copy $tmpdir/*.zip"
    cp -f $tmpdir/*.txt $MountPath/ || exit_upgrade "copy $tmpdir/*.txt"
    rm -rf $tmpdir

    exit_upgrade
}

start_cmd() {
    local cmd=$2
    [ "$cmd" = "x" ] && ps_stop
    [ "$cmd" = "q" ] && {
        [ -f "$ResultFile" ] && {
            cat "$ResultFile"
            exit 0
        }
        [ ! -f "$ResultFile.mutex" ] && {
            echo "Stopped"
            exit 0
        }

        local total=$(cat "$ResultFile.rootfs_ext4.emmc.size" 2>/dev/null)
        local size=$(tr '\r' '\n' <"$ResultFile.rootfs_ext4.emmc.prog" | awk 'END {print $1}')
        echo "Progress: $((size)) $((total))"
        exit 0
    }
}

start() {
    start_cmd $@
    ps_mutex
    mount_recovery

    local zip="$2"
    step_log "Check ota pack $zip"
    [ -z "$zip" ] && {
        local manifest_file="$MountPath/$SHA256_FILE"
        [ -f "$manifest_file" ] || exit_upgrade "no manifest file found in $MountPath"
        
        file="$(parse_hash name "$manifest_file")" || exit_upgrade "parse name $manifest_file"
        zip="$MountPath/$file"
    }
    [ -e "$zip" ] || { exit_upgrade "not found $zip"; }
    step_result "OTA will use $zip (integrity: $HASH_TYPE)"

    ota_boot "$zip" "$FIP_PART" "fip.bin"
    ota_boot "$zip" "$BOOT_PART" "boot.emmc"

    local rootfs_size=$(zip_get_size "$zip" "rootfs_ext4.emmc")
    [[ -n "$rootfs_size" && $((rootfs_size)) -ne 0 ]] && {
        local target="$ROOTFS_B"
        is_rootfs_b || target="$ROOTFS"
        zip_write_part "$zip" "$target" "rootfs_ext4.emmc"
        switch_partition
    }
    exit_upgrade
}

recovery() {
    step_log "Set recovery flag"
    set_bootenv factory_reset 1
    step_log "Please reboot to take effect."
    exit_upgrade
}

clean() {
    rm -rf $UPGRADE_FILES
    rm -rf $UPGRADE_TMP/*
    echo "Success"
}

# call function
RunCase=$1
ResultFile="$UPGRADE_FILES/$RunCase"
FUNCS=(latest download start rollback recovery clean)
for func in ${FUNCS[@]}; do
    [ "$RunCase" = "$func" ] && {
        $func $@
        exit 0
    }
done
echo "Usage: $0 {clean|latest|download|start|rollback|recovery}"
