# Data‑at‑Rest Encryption (Driver Level)

Owner: Authority Alert Team
Last updated: 2025-10-12

## Goal
Protect sensitive on‑device data (uploads, queues, logs, configs, credentials) if the device is lost, stolen, or serviced. Meet common requirements (CJIS, state privacy laws) with minimal UX friction and acceptable performance on CV18xx‑class hardware.

## Threat Model
- Attacker with physical access to storage (eMMC/SD) or console.
- Cold boot or powered‑off device; attacker attempts offline extraction.
- Out of scope: live, authenticated admin with console access; RAM scraping.

## Targets (What to Encrypt)
- `/userdata/` (uploads, queues, Node‑RED flows/storage, configs, keys).
- `/var/log/authority-alert/` and other sensitive logs.
- Credential stores (OAuth tokens, API keys).
- Optional: full overlay upperdir if rootfs overlay uses `/userdata/`.

## Approaches
1) dm‑crypt/LUKS2 (block‑device encryption)
- Encrypts an entire partition (e.g., eMMC userdata). Transparent to filesystems.
- Pros: strong, mature, simple policy boundary. Cons: single trust domain per volume.
- Recommended default for `/userdata`.

2) fscrypt (per‑directory/file encryption on ext4)
- Application‑scoped keys; encrypt selected directories (e.g., `/userdata/uploads`).
- Pros: selective, easier key rotation for specific data. Cons: app changes; metadata leaks.
- Recommended where multi‑tenant separation is needed.

3) Inline encryption (blk‑crypto / eMMC inline)
- Uses storage controller crypto (if supported) via kernel blk‑crypto.
- Pros: low overhead; offload. Cons: SoC/driver support varies.
- Use opportunistically; fall back to dm‑crypt.

## Recommended Baseline
- LUKS2 on `/dev/mmcblk0p3` (example userdata partition) → `/dev/mapper/userdata` mounted at `/userdata`.
- Optional fscrypt for sub‑dirs with different retention/rotation needs (`/userdata/uploads`, `/userdata/queues`).

## Boot & Unlock Flow
- Secure Boot + signed kernel/DTB/U‑Boot (see vendor SecureBoot guide) to protect boot chain.
- Early init script:
  1. Locate encrypted volume and LUKS header.
  2. Load key material from secure source (see Key Management) or prompt (agency mode).
  3. `cryptsetup open` → map device; mount `/userdata`.
  4. Mount overlayfs (if using root overlay with upperdir on `/userdata`).
  5. Continue normal boot.
- Failure path: drop to maintenance shell or read‑only mode with explicit warning.

## Key Management
- Master Content Encryption Key (CEK) never leaves device in plaintext.
- Key sources (choose per deployment):
  - Device‑Unique Key (DUK) derived from SoC HUK/eFuse via key ladder (preferred if available in CV18xx). Use AEAD to wrap a random CEK and store wrapped blob on disk (`/etc/keys/userdata.wrapped`).
  - User/Agency passphrase (OOBE) → use Argon2id/PBKDF2 to derive KEK; add as LUKS keyslot; store no passphrase.
  - Remote KMS (agency): fetch wrapped CEK using mutual TLS at boot (device holds client cert); cache ephemeral session key only in RAM.
- Rotation: add new LUKS keyslot → remove old; for fscrypt, rewrap directory policy keys.
- Recovery: optional recovery code escrow (printed once) or out‑of‑band KMS unseal.

## Retention & Wipe
- Respect global 24‑hour retention for uploaded media on `/userdata/uploads`; enforce via cron/systemd timer and application logic.
- Provide `authority-support wipe-data` to securely purge uploads/queues/logs and remove CEK (crypto‑erase).
- On decommission, remove all LUKS keyslots and zero header backups; optionally re‑partition.

## Buildroot / Kernel Configuration
- Kernel:
  - `CONFIG_BLK_DEV_DM=y`, `CONFIG_DM_CRYPT=y`
  - `CONFIG_CRYPTO_USER_API_SKCIPHER=y`
  - `CONFIG_CRYPTO_AES_ARM64_CE=y` (or platform‑specific AES accel)
  - `CONFIG_BLK_INLINE_ENCRYPTION=y` and `CONFIG_SCSI_EMMC_ENCRYPTION` if supported
  - For fscrypt: `CONFIG_FS_ENCRYPTION=y`, `CONFIG_EXT4_ENCRYPTION=y`
- Userspace:
  - `cryptsetup` (with LUKS2), `busybox` or `util-linux` mount tools
  - `e2fsprogs` with fscrypt tooling (`fscrypt`), `keyutils`
- Init scripts:
  - `/etc/init.d/S05-crypt-userdata` (open LUKS, mount `/userdata`)
  - Adjust overlay mounts to ensure upperdir exists post‑unlock

## Provisioning Steps (Example)
- Partition and format:
  - `parted /dev/mmcblk0 mkpart userdata ...`
  - `cryptsetup luksFormat /dev/mmcblk0p3` (add KEK from device DUK or passphrase)
  - `cryptsetup open /dev/mmcblk0p3 userdata`
  - `mkfs.ext4 /dev/mapper/userdata && mkdir -p /userdata`
- Boot integration:
  - Store wrapped CEK at `/etc/keys/userdata.wrapped` (600 perms)
  - Early boot script unwraps CEK using DUK → dm‑crypt `--key-file -`

## Performance Notes
- Prefer AES‑XTS for dm‑crypt; tune sector size to storage.
- Enable CPU crypto extensions (ARMv8 CE) where available.
- Test throughput with `fio`; monitor CPU via `top` during AI workload.

## Compliance
- At‑rest encryption with managed keys supports CJIS 5.10.1.2 (encryption), state privacy acts, and common RFP checklists.
- Document consent + retention separately (`PRIVACY_AND_REMOTE_ACCESS.md`, `LOGGING_RETENTION.md`).

## Open Items
- Confirm CV18xx inline encryption/blk‑crypto support in current kernel tree; enable if available.
- Decide default key source per SKU (passphrase vs device‑sealed key).
- Add factory provisioning script and QA checklist for key enrollment.

## Note on One‑Time Pad (OTP)
- OTP is not appropriate for persistent data‑at‑rest on-device because the pad must be at least as large as the data and remain separate and secret. Storing the pad alongside ciphertext defeats security.
- If a narrow, transfer‑only OTP mode is required, see `ONE_TIME_PAD.md` for a strictly limited outbound workflow with removable pad media and one‑time consumption enforcement.
