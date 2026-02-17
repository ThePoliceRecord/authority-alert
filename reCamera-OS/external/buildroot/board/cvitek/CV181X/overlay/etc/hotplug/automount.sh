#!/bin/sh

MOUNT_POINT=/mnt/sd

# echo "<<<<<<<<<<<<<$1 $2>>>>>>>>>>>>>"
if [ "$1" == "mmcblk1p1" ]; then
  disk="/dev/$1"
  if [ "$2" = "add" ]; then
      rootfs_rw on
      mkdir -p $MOUNT_POINT
      rootfs_rw off

      umount $MOUNT_POINT
      mount $disk $MOUNT_POINT
  else
      umount $MOUNT_POINT
      rootfs_rw on
      rmdir $MOUNT_POINT
      rootfs_rw off
  fi
fi