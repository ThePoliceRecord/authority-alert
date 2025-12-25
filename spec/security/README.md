# Security Audit (Known CVEs) – 0.1.0

Owner: Authority Alert Team
Last updated: 2025-12-19

This folder contains a targeted “known CVE” audit workflow for the Authority Alert stack without scanning the entire repository.

## What this is (and isn’t)
- This is a **targeted component audit**: it checks a curated list of high-risk packages (network-facing + core crypto/media tooling) against the NVD CVE database using CPE identifiers.
- It is **not** a complete SBOM of the firmware image. The most accurate source is an on-device version snapshot (`spec/version_audit.sh`) or Buildroot `legal-info` artifacts from a build.

## Inventory
- `INVENTORY.json` lists components + CPEs used for NVD lookups.
- Update the versions/CPEs whenever build configuration changes.

Sources used to derive versions (in this repo):
- Buildroot package versions under `reCamera-OS/buildroot-2021.05/package/*/*.mk`
- Pinned versions noted in `spec/VERSIONS.md`

## Run the audit (host machine)
This query uses the public NVD API (no key). Keep the component list small to avoid long runtimes.

```bash
python3 scripts/security_audit_nvd.py \
  --inventory spec/releases/0.1.0/security/INVENTORY.json \
  --outdir spec/releases/0.1.0/security/out \
  --min-score 7.0
```

Outputs:
- `spec/releases/0.1.0/security/out/NVD_SUMMARY.md` (human summary)
- `spec/releases/0.1.0/security/out/nvd_*.json` (per-component raw)
- `spec/releases/0.1.0/security/out/nvd_summary.json` (aggregate)

## Recommended follow-up
- Run `spec/version_audit.sh` on a real device and compare to this inventory.
- If any component is exposed to untrusted networks (SSH, mosquitto, RTSP/live555), prioritize patching/upgrading those first.

