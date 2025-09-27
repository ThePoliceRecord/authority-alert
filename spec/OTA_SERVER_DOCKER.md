# OTA Update Server (Docker Spec)

Owner: YOU
Last updated: YYYY-MM-DD

## Goal
- Provide a lightweight HTTP server (Docker-based) to host OTA artifacts (`sg2002_recamera_emmc_md5sum.txt`, `*_ota.zip`).
- Match the expectations of `/mnt/system/upgrade.sh` (direct manifest URLs or GitHub-style redirects).
- Enable reproducible local testing before publishing firmware.

## Requirements
- Docker or compatible container runtime (Compose optional but recommended).
- Static file hosting with HTTP/HTTPS support.
- Volume/bind mount where Buildroot output artifacts can be dropped.
- Optional: authentication, TLS, logging.

## Recommended Stack
- Base image: `nginx:alpine` (small, static hosting). Alternative: `caddy:alpine` if auto-TLS is desired.
- Container document root: serve files under `/usr/share/nginx/html`.
- Host directory structure:
  - `ota_content/releases/<version>/sg2002_recamera_emmc_md5sum.txt`
  - `ota_content/releases/<version>/<artifact>.zip`
- For HTTPS, front with reverse proxy (Traefik, Caddy) or terminate TLS inside the container.

## OTA File Requirements
- Manifest file *must* be named `sg2002_recamera_emmc_md5sum.txt`.
- Each line format: `<md5> <filename>` (single space).
- ZIP archive must contain `rootfs_ext4.emmc` and `md5sum.txt`; optionally `fip.bin`, `boot.emmc`.
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
      ├─ sg2002_recamera_emmc_md5sum.txt
      └─ sg2002_reCamera_0.2.1_emmc_ota.zip
```

### Sample Manifest (`sg2002_recamera_emmc_md5sum.txt`)
```
d41d8cd98f00b204e9800998ecf8427e sg2002_reCamera_0.2.1_emmc_ota.zip
```

## Test Workflow
1. Start server: `docker compose up -d` (from directory containing `docker-compose.yml`).
2. Verify locally:
   - `curl -I http://localhost:8080/releases/0.2.1/sg2002_recamera_emmc_md5sum.txt`
   - `curl http://localhost:8080/releases/0.2.1/sg2002_recamera_emmc_md5sum.txt`
3. On device:
   - `echo "1,http://<host>:8080/releases/0.2.1/sg2002_recamera_emmc_md5sum.txt" | sudo tee /etc/upgrade`
   - `sudo /mnt/system/upgrade.sh latest`
   - `sudo /mnt/system/upgrade.sh download`
   - `sudo /mnt/system/upgrade.sh start` (only after confirming idle slot).

## Optional Enhancements
- **Auth**: mount custom Nginx config enabling basic auth (htpasswd) or token headers.
- **HTTPS**: use `caddy:alpine` with ACME, or let Traefik terminate TLS.
- **Indexing**: add `index.html` with version list; `upgrade.sh` only needs manifest but humans may want UI.
- **Automation**: script to copy files from `output/sg2002_recamera_emmc/install/...` into `ota_content/releases/<version>/` and regenerate manifest.
- **swupdate support**: host `.swu` files alongside `.zip` for devices using swupdate.

## Security Notes
- Serve over HTTPS on public networks to prevent tampering.
- Restrict access (VPN/firewall) for pre-release images.
- Consider cryptographic signing (manifest signature or swupdate signed bundles) for production deployments.

## Next Steps
- Create `ota_content/` scaffold and commit placeholder README (optional).
- Automate manifest generation (md5sum command in build scripts).
- Integrate into CI/CD (e.g., publish image to registry, deploy with Ansible/K8s).
- Update this spec if you switch to swupdate `.swu` flow or add staging channels.
