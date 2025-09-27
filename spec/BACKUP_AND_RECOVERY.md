# Backup & Recovery Manual

Owner: YOU
Last updated: YYYY-MM-DD

## Objectives
- Preserve system configuration, Node-RED flows, and user data.
- Provide procedures for firmware rollback and disaster recovery.

## Backup Targets
- `/home/recamera/.node-red/` (flows.json, credentials, custom nodes).
- `/userdata/config/` (OOBE results, privacy settings, network config).
- `/etc/wpa_supplicant.conf`, `/etc/upgrade` (network + OTA settings).
- Logs (optional) if needed for investigations.

## Backup Methods
1. **Manual CLI**
   ```bash
   mkdir -p /userdata/backups
   tar czf /userdata/backups/aa-backup-$(date +%Y%m%d).tgz \
     /home/recamera/.node-red \
     /userdata/config \
     /etc/wpa_supplicant.conf \
     /etc/upgrade
   ```
2. **UI-triggered Backup**
   - Provide button in settings → backend creates archive in `/userdata/backups/` and offers download.
3. **Automated Schedule**
   - Optional cron job (weekly) with retention of N copies.

## Restore Procedure
1. Copy backup archive to device (SCP or USB).
2. Stop services: `sudo /etc/init.d/S03node-red stop` (and other dependent services).
3. Extract archive:
   ```bash
   sudo tar xzf aa-backup-YYYYMMDD.tgz -C /
   ```
4. Fix ownership: `sudo chown -R recamera:recamera /home/recamera/.node-red`
5. Restart services: `sudo /etc/init.d/S03node-red start`
6. Verify UI, flows, network connectivity.

## Firmware Rollback / Recovery
- Device uses A/B rootfs partitions.
- To rollback:
  1. Run `/mnt/system/upgrade.sh start <previous_ota.zip>` targeting other slot.
  2. Alternatively, set boot env manually: `fw_setenv use_part_b 0` (or 1) and reboot.
  3. After rollback, re-apply backup.
- For factory reset:
  - Set `fw_setenv factory_reset 1` and reboot (per `rootfs_overlay.sh`).
  - Ensure backup exists before reset.

## Disaster Scenarios
- **Corrupted Userdata**: Format `/dev/mmcblk0p6` and restore from backup.
- **Node-RED flows missing**: Restore `/home/recamera/.node-red/flows.json` from archive.
- **Device unresponsive**: Use recovery SD image (documented in build output) to reflash base firmware, then restore backup.

## Verification
- After each backup/restore, run health check:
  - Node-RED flows deployed.
  - OTA settings correct (`/etc/upgrade`).
  - Privacy toggles intact.
  - Camera preview works.

## Storage & Retention
- Recommend storing copies on secure server with encryption.
- Keep at least last 3 backups; rotate older ones as policies allow.

