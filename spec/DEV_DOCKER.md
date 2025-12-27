# Dev Docker Container (Build Environment)

Owner: YOU
Last updated: YYYY-MM-DD

## Purpose
- Provide a reproducible containerized toolchain for building reCamera OS images and OTA artifacts.
- Match the setup used by `./docker_build.sh` so local builds mirror CI/official releases.
- Serve as the base environment for compiling custom packages, regenerating rootfs, or producing OTA zips before publishing to the OTA server.

## Host Requirements
- Docker Engine >= 20.10 (root or rootless with proper permissions).
- At least 30 GB free disk space (per README recommendation).
- Suggested host OS: Linux (Ubuntu 20.04+).
- Optional: add user to `docker` group for passwordless builds (`sudo usermod -aG docker $USER`).

## Entry Points
- `./docker_build.sh <target>` — wraps Docker run, mounts workspace, and kicks off Buildroot build.
- Example target: `sg2002_recamera_emmc`.
- Script location: `reCamera-OS/docker_build.sh`.

## docker_build.sh Flow (high level)
1. Pull/build the base Docker image specified inside the script (typically a Buildroot toolchain image).
2. Mount project directory into the container.
3. Sync required submodules (`git submodule update --init --recursive --depth 1`).
4. Invoke build script inside container (`make <target>` or vendor build pipeline).
5. Outputs land in `reCamera-OS/output/<target>/...` on host.

## Custom Build Steps
- To build: `cd reCamera-OS && ./docker_build.sh sg2002_recamera_emmc`.
- To rebuild after changes: rerun the same command; Docker layer caching speeds up tool downloads.
- To open an interactive shell inside the build container:
  ```bash
  ./docker_build.sh sg2002_recamera_emmc --shell
  ```
  (if script supports `--shell`; otherwise use `docker run -it` with the same image/mounts — inspect script for exact image name and volume options.)

## Volume Layout (typical)
- Host repo mounted at `/workspace/reCamera-OS` inside container.
- Build outputs written under `output/<target>/` and `buildroot-2021.05/output/...` (persist on host).
- User-specific artifacts (e.g., `output/sg2002_recamera_emmc/install/...`) available after build completes.

## Environment Variables
- `TARGET` — passed to build pipeline (e.g., `sg2002_recamera_emmc`).
- `PROJECT_OUT`, `OUTPUT_DIR`, etc., set by `external/setenv.sh` once inside container.
- Adjusting build flags: edit Buildroot defconfigs under `external/build/boards/...` and rerun build.

## Adding Packages / Modifying Build
1. Edit Buildroot defconfig (`external/build/boards/.../sg2002_recamera_emmc_defconfig`).
2. Modify overlays or package recipes as needed.
3. Run `./docker_build.sh <target>` to rebuild.
4. Generated OTA zip: `output/<target>/install/soc_<target>/*_ota.zip`.

## Cleaning Builds
- Inside container (or via script): run `make <target> clean` or `rm -rf output/<target>` on host.
- To nuke Buildroot output: remove `buildroot-2021.05/output/...` (forces toolchain rebuild).

## Troubleshooting
- Permission denied on Docker socket: ensure user is in `docker` group or run script with `sudo`.
- Disk space: prune Docker images (`docker system prune`) and remove old `output/*` directories.
- Submodule errors: run `git submodule update --init --recursive --depth 1` manually on host.
- Build failures: drop into container shell to inspect logs (`./docker_build.sh <target> --shell`).

## Integration with OTA Workflow
- After successful build, copy OTA artifacts into OTA server content directory (`spec/OTA_SERVER_DOCKER.md`).
- Use `spec/version_audit.sh` to verify versions before publishing.
- Update manifests (`sg2002_recamera_emmc_sha256sum.txt`) with `sha256sum <zip> > manifest`.

## Next Steps
- Confirm `docker_build.sh` image tag matches internal requirements (document inside script if needed).
- Automate build via CI (GitHub Actions/Runner) calling the same script.
- Extend documentation if additional targets (`sg2002_recamera_sd`, `sg2002_xiao_sd`) are required.
