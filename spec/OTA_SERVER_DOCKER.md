# OTA Update Server (Docker Spec)

Owner: YOU
Last updated: 2026-01-10

## Goal
- Provide a lightweight HTTP server (Docker-based) to host OTA artifacts (`sg2002_recamera_emmc_sha256sum.txt`, `*_ota.zip`).
- Match the expectations of `/mnt/system/upgrade.sh` (direct manifest URLs or GitHub-style redirects).

## Requirements
- Docker or compatible container runtime (Compose optional but recommended).
- Static file hosting with HTTP/HTTPS support.
- Volume/bind mount where Buildroot output artifacts can be dropped.
- Optional: authentication, TLS, logging.

## Recommended Stack
- Base image: `nginx:alpine` (small, static hosting). Alternative: `caddy:alpine` if auto-TLS is desired.
- Container document root: serve files under `/usr/share/nginx/html`.
- Host directory structure:
  - `ota_content/releases/<version>/sg2002_recamera_emmc_sha256sum.txt`
  - `ota_content/releases/<version>/<artifact>.zip`
- For HTTPS, front with reverse proxy (Traefik, Caddy) or terminate TLS inside the container.

## OTA File Requirements
- Manifest file naming:
  - `sg2002_recamera_emmc_sha256sum.txt`
- Each line format: `<hash> <filename>` (single space).
  - SHA256: 64 hex characters
- ZIP archive must contain `rootfs_ext4.emmc` and `sha256sum.txt`; optionally `fip.bin`, `boot.emmc`.
- Device custom URL (`/etc/upgrade`) expects `1,<manifest-url>`.

## docker-compose Example
```yaml
version: '3.9'
services:
  ota-server:
    image: nginx:alpine
    container_name: ota-server
    ports:
      - "8080:80"
    volumes:
      - ./ota_content:/usr/share/nginx/html:ro
    restart: unless-stopped
```

### Host Directory Layout
```
ota_content/
└─ releases/
   └─ 0.2.1/
      ├─ sg2002_recamera_emmc_sha256sum.txt
      └─ sg2002_reCamera_0.2.1_emmc_ota.zip
```

### Sample Manifest (`sg2002_recamera_emmc_sha256sum.txt`)
```
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  sg2002_reCamera_0.2.1_emmc_ota.zip
```

## Test Workflow
1. Start server: `docker compose up -d` (from directory containing `docker-compose.yml`).
2. Verify locally:
   - `curl -I http://localhost:8080/releases/0.2.1/sg2002_recamera_emmc_sha256sum.txt`
   - `curl http://localhost:8080/releases/0.2.1/sg2002_recamera_emmc_sha256sum.txt`
3. On device:
   - `echo "1,http://<host>:8080/releases/0.2.1/sg2002_recamera_emmc_sha256sum.txt" | sudo tee /etc/upgrade`
   - `sudo /mnt/system/upgrade.sh latest`

## Optional Enhancements
- **Auth**: mount custom Nginx config enabling basic auth (htpasswd) or token headers.
- **HTTPS**: use `caddy:alpine` with ACME, or let Traefik terminate TLS.
- **Indexing**: add `index.html` with version list; `upgrade.sh` only needs manifest but humans may want UI.
- **Automation**: script to copy files from `output/sg2002_recamera_emmc/install/...` into `ota_content/releases/<version>/` and regenerate manifest.
- **swupdate support**: host `.swu` files alongside `.zip` for devices using swupdate.

## Alternative Workflow (no server): Supervisor UI upload

If you don't want to run a hosted OTA server for a one-off update, the Supervisor supports staging an OTA zip via upload:
- `POST /api/deviceMgr/uploadUpdatePackage` (multipart form, field `file`)
- `POST /api/deviceMgr/applyUploadedUpdatePackage`

This uses the same upgrader but skips the download step.

## Security Notes
- Serve over HTTPS on public networks to prevent tampering.
- Restrict access (VPN/firewall) for pre-release images.
- Consider cryptographic signing (manifest signature or swupdate signed bundles) for production deployments.

## Next Steps
- Create `ota_content/` scaffold and commit placeholder README (optional).
- Automate manifest generation (sha256sum command in build scripts).
- Integrate into CI/CD (e.g., publish image to registry, deploy with Ansible/K8s).
- Update this spec if you switch to swupdate `.swu` flow or add staging channels.
