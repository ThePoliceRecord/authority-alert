# Node-RED & UI Configuration

Owner: YOU
Last updated: YYYY-MM-DD

## Runtime Overview
- **Service script**: `/etc/init.d/S03node-red` (see `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/etc/init.d/S03node-red`).
  - Runs as the first non-root shell user (`recamera`).
  - Launches `/usr/bin/node-red-pi` with `--max-old-space-size=64 --expose-gc` and sets the userDir to `/home/<user>/.node-red`.
- **Node-RED install roots**:
  - Core runtime under `/usr/lib/node_modules/node-red/` (settings, custom CSS).
  - Project workspace under `/home/recamera/.node-red/` (flows, credentials, npm modules).
- **Start order**: Node-RED starts before hardware scripts (S70) and after Buildroot defaults. Ensure any flows that depend on hardware services check availability or delay execution.
- **Port**: 1880 (see `uiPort` in settings.js). Dashboard reachable at `http://<device>:1880/ui`.

## Key Files
- `settings.js`: `reCamera-OS/external/buildroot/board/cvitek/CV181X/overlay/usr/lib/node_modules/node-red/settings.js`
  - Flow file name: `flows.json`.
  - Static assets served from `/home/recamera/.node-red/public/` (`httpStatic`).
  - Editor theme tweaks: custom title "reCamera" and `custom.css`.
  - Diagnostics & runtime-state endpoints enabled.
- `custom.css`: `.../node-red/custom.css`
  - Overrides header color (green deploy button, white header). Adjust for branding.
- Flow storage (runtime): `/home/recamera/.node-red/flows.json` (generated at first deploy).
  - Credentials: `/home/recamera/.node-red/flows_cred.json` (encrypted if `credentialSecret` set).
- Node modules preinstalled (from `sscma-node.mk`):
  - `node-red-contrib-sscma`, `node-red-contrib-os`, `node-red-contrib-seeed-canbus`, `node-red-contrib-seeed-recamera`.
  - `@flowfuse/node-red-dashboard@1.26.0` (dashboard UI), `socketcan@4.0.5`.

## Directory Layout (after build/run)
```
/home/recamera/.node-red/
├─ flows.json                  # active flows (created at runtime)
├─ flows_cred.json             # encrypted credentials
├─ package.json / package-lock.json  # Node module metadata
├─ node_modules/               # installed nodes (from sscma-node package)
├─ public/                     # static assets served under httpStatic root
└─ settings.js (optional override if copied here)
```

## Customising Flows & UI
1. **Editor access**
   - Browse to `http://<device-ip>:1880/`
   - Admin authentication is disabled by default; enable in `settings.js` before release (see *Security* below).
2. **Editing flows**
   - Use Node-RED editor; deploy writes to `/home/recamera/.node-red/flows.json`.
   - Files persist thanks to overlay mount (userdata partition). Back up flows before firmware flashes.
3. **Bundling custom flows in firmware**
   - For immutable default flows, add them to the build overlay (e.g., create `external/ramdisk/rootfs/overlay/cv181x_musl_riscv64/home/recamera/.node-red/flows.json`).
   - Alternatively, fork `sscma-node` repo (pulled during build) and commit flows/static under its `rootfs` directory.
4. **Dashboard styling**
   - Edit `custom.css` to adjust colors/layout.
   - Static files (images, JS) go in `/home/recamera/.node-red/public/` and are served at `/` (or adjust `httpStaticRoot`).
5. **CAN / camera nodes**
   - Preinstalled custom nodes expose camera, CAN bus, and OS operations. Review their README in the `sscma-node` repository.

## Security Hardening
- Enable admin authentication:
  - Uncomment `adminAuth` in settings.js, replace bcrypt hash with new password (`node-red admin hash-pw`).
  - Optionally enable `https` (provide cert/key) or reverse proxy behind TLS.
- Lock down dashboard endpoints via `httpNodeAuth` or reverse proxy.
- Disable palette manager by setting `externalModules.palette.allowInstall = false` (prevents arbitrary npm installs).
- Ensure `/home/recamera/.node-red` permissions are owned by `recamera:recamera` (handled by init script on first run).

## Interaction with Other Services
- `sscma-node` CLI (`/usr/local/bin/sscma-node --start`) runs alongside Node-RED (see `ps` output). Flows may call its REST/WebSocket endpoints.
- `mosquitto` broker is enabled; Node-RED nodes can publish/subscribe on localhost.
- `ttyd` provides web terminal at port 9090; avoid exposing Node-RED editor publicly without authentication.

## Deployment Checklist
- Update flows and dashboard components.
- Run `spec/version_audit.sh` to confirm Node-RED version (ensure `node-red --version` works after changes).
- Export flows (`Menu > Export > Clipboard`) for backup.
- If bundling with firmware, place flows/static assets in overlay or `sscma-node` repo and rebuild.
- Verify on device: `curl http://localhost:1880/`, `curl http://localhost:1880/ui/`, check logs (`journalctl -u node-red` or `/tmp/node-red/*`).

## Open Questions
- Identify exact REST endpoints exposed by `node-red-contrib-sscma` (review upstream repo once submodule is fetched).
- Decide whether to lock dashboard behind auth or keep publicly accessible on LAN.
- Plan for version upgrades: Node-RED 4.x is current; align with Node.js LTS.

