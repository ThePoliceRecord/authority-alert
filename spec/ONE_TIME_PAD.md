# One-Time Pad (OTP) Mode

Owner: Authority Alert Team
Last updated: 2025-10-12

## Scope & Warning
- OTP provides information-theoretic secrecy only if the key (pad) is truly random, at least as long as the plaintext, used exactly once, and kept perfectly secret.
- This mode is for narrowly scoped, outbound transfer of small artifacts (e.g., a single report package). It is NOT suitable for persistent data-at-rest encryption on-device.

## When To Use
- High-assurance, manual export from a device to a designated recipient under strict physical controls.
- Situations where the pad can be pre-shared securely and consumed one time by both parties.

## When NOT To Use
- General data-at-rest on the device (see `DATA_AT_REST_ENCRYPTION.md` for LUKS/fscrypt).
- Routine telemetry, streaming, automated uploads, or backups.

## Operational Requirements
- Pad generation: true randomness from a verifiable source (e.g., HWRNG + DRBG with NIST SP 800-90A validation) and offline pad creation tooling.
- Distribution: pre-share pads via offline, tamper-evident media; maintain custody records.
- Storage: pads must NEVER reside on the device’s internal storage. Use removable media (e.g., dedicated, sealed microSD) carried by operator.
- Consumption: enforce one-time use with a monotonic pad cursor stored on the removable pad media, not the device. Refuse reuse or out-of-order access.
- Destruction: shred consumed pad segments after use; zeroize device buffers and temporary files.

## Integrity / Authentication
- OTP gives confidentiality only. Add one-time authentication (e.g., Wegman–Carter MAC) using a separate one-time key segment from the same pad.
- Alternatively, a separate, pre-shared MAC-only pad can be used for tags.

## Workflow
- Prepare pad media: `pads/segment_k.bin` (cipher pad), `pads/segment_t.bin` (tag pad), `cursor.json` (tracks bytes used).
- On device export:
  1. User selects small payload (e.g., zip report ≤ 10 MB).
  2. Insert pad media; UI/CLI verifies pad integrity and available bytes.
  3. Encrypt with XOR using next unused cipher pad bytes; compute MAC using next unused tag pad bytes.
  4. Advance cursor on pad media atomically; write encrypted output to operator media; zeroize working buffers.
  5. Provide receipt with byte counts consumed; do NOT store plaintext or pad on device.
- On recipient side:
  1. Load same pad media (or synchronized copy) and cursor; verify no reuse.
  2. Decrypt via XOR; verify MAC; advance cursor; zeroize memory.

## CLI (Concept)
- `aa-otp encrypt --in payload.zip --out payload.zip.otp --pad /media/pad/ --max-bytes 10485760`
- `aa-otp decrypt --in payload.zip.otp --out payload.zip --pad /media/pad/`

## Risks & Caveats
- Any pad reuse or pad exposure compromises security.
- Pads are operationally heavy: logistics, inventory, loss/theft management.
- No audit trail of content is retained (by design). Keep a minimal receipt log without PII (time, size, destination identity code).

## Alternatives
- For most use cases, prefer AES-GCM with ephemeral keys over TLS or LUKS/fscrypt for data at rest. See `DATA_AT_REST_ENCRYPTION.md`.

