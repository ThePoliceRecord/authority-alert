# Streaming Stack Upgrade: MediaMTX Migration

Owner: YOU
Last updated: YYYY-MM-DD

## Summary
- Replace the current Live555-based streaming components (including `hls_proxy_service`) with **MediaMTX** (formerly rtsp-simple-server) before full Authority Alert release.
- Goal: simplify streaming pipeline, reduce resource usage, and gain modern protocol support (RTSP, RTMP, HLS, WebRTC) in a single service.

## Rationale
- Live555 binaries add weight and require separate proxy for HLS.
- MediaMTX provides a single Go binary with multi-protocol support, simpler configuration, and active maintenance.
- Integrates better with custom UI preview and remote streaming use cases.

## Migration Plan
1. **Evaluate MediaMTX**
   - Build static binary for CV181x (ARM64/RISC-V) or cross-compile via Docker.
   - Confirm resource footprint on device (CPU, RAM) under typical load.
2. **Pipeline Integration**
   - Configure MediaMTX to ingest from existing camera pipeline (FFmpeg replacement or direct GStreamer feed).
   - Expose RTSP/HLS endpoints used by status screen preview and third-party clients.
   - Ensure Node-RED flows can trigger recordings/streams via MediaMTX APIs.
3. **Packaging**
   - Add new Buildroot package or vendor binary under `external/br2-external/`.
   - Remove Live555-related packages, scripts (`hls_proxy_service`).
   - Update init scripts to manage MediaMTX service (systemd or `/etc/init.d/Sxxmediamtx`).
4. **Configuration**
   - Store config file under `/mnt/system/etc/mediamtx.yml` (overlay-friendly).
   - Provide UI toggles for enabling/disabling protocols; default to RTSP/HLS for preview.
5. **Testing**
   - Verify low-latency preview in new status screen (WebRTC or low-latency HLS options).
   - Test remote viewing, multi-client scenarios, and network resets.
   - Validate security (auth tokens, TLS termination if needed).
6. **Documentation Updates**
   - Update `spec/HARDWARE_DEPLOYMENT.md` and UI docs to reference new streaming endpoints.
   - Include MediaMTX version in `spec/VERSIONS.md` once integrated.

## Timeline
- Target completion: before first production Authority Alert release (align with UI redesign milestone).
- Include migration tasks in release checklist.

## Open Questions
- Determine final ingest pipeline (direct hardware encoder vs FFmpeg).
- Decide on authentication model for MediaMTX (basic auth, token, IP allowlist).
- Evaluate transcoding requirements for remote analysis uploads.

