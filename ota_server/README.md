# Local OTA Server

Lightweight Dockerized HTTP server for hosting OTA artifacts compatible with `/mnt/system/upgrade.sh`.

What it serves
- `releases/<version>/sg2002_recamera_emmc_md5sum.txt`
- `releases/<version>/*_emmc_ota.zip`

Quick start
1) Build firmware artifact (from repo root):
   - `./reCamera-OS/docker_build.sh sg2002_recamera_emmc`
   - Artifact will be under `reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc/*_emmc_ota.zip`.

2) Stage a release (from this folder):
   - `chmod +x prepare_release.sh`
   - `./prepare_release.sh 0.1.0`

3) Run the server:
   - `docker compose up -d`

4) Verify locally:
   - `curl -I http://localhost:8080/releases/0.1.0/sg2002_recamera_emmc_md5sum.txt`
   - `curl -I http://localhost:8080/releases/0.1.0/` (directory listing enabled)

5) Point device to this server and upgrade:
   - `echo "1,http://<host>:8080/releases/0.1.0/sg2002_recamera_emmc_md5sum.txt" | sudo tee /etc/upgrade`
   - `sudo /mnt/system/upgrade.sh latest`
   - `sudo /mnt/system/upgrade.sh download`
   - `sudo /mnt/system/upgrade.sh start`

Notes
- This server is static-file only. Publishing is by copying files into `ota_content/releases/<version>/` (the helper script does this for local builds).
- If you need remote publishing from CI via HTTP PUT, use a server that supports PUT (e.g., Caddy with `respond`/`file_server` + `route` handling, nginx with `dav` module configured, S3, or a simple artifact store). The included nginx config is read-only.
- For internet exposure, add TLS and access control; for local/LAN testing, plain HTTP is fine.

