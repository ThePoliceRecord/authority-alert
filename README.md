# Authority Alert - reCamera-OS Build Environment

Custom firmware build environment for the reCamera platform, targeting the SG2002 RISC-V SoC.

## Project Overview

This project provides:

- **reCamera-OS**: Custom firmware based on Buildroot 2021.05 for the SG2002 SoC (included as a git submodule)
- **OTA Server**: Local OTA update server for testing firmware deployments
- **Nix Development Shell**: Reproducible build environment with Docker integration

### Target Hardware

- **SoC**: SG2002 RISC-V
- **Configuration**: `sg2002_recamera_emmc`
- **Toolchain**: `riscv64-unknown-linux-musl-` (pre-built)

## Prerequisites

- [Nix](https://nixos.org/download.html) with flakes enabled
- Docker (for builds)
- Git

### Enable Nix Flakes

Add to `~/.config/nix/nix.conf`:

```
experimental-features = nix-command flakes
```

## Quick Start

### 1. Clone with Submodules

```bash
git clone --recurse-submodules <repo-url>
cd authority_alert_planning
```

Or if already cloned:

```bash
git submodule update --init --recursive
```

### 2. Initialize Submodules

```bash
nix run .#init
```

### 3. Enter Development Shell

```bash
nix develop
```

This provides:
- Docker and docker-compose
- Python 3 with build dependencies
- Git, Make, and standard utilities
- Build helper commands

### 4. Build Firmware

**Recommended: Use Nix app wrapper**

```bash
nix run .#build
```

This automatically:
- Builds the Docker image (Ubuntu 20.04 based)
- Initializes git submodules
- Runs the build with correct permissions
- Outputs to `reCamera-OS/output/`

**Alternative: Manual Docker build**

```bash
cd reCamera-OS
bash docker_build.sh sg2002_recamera_emmc
```

## Available Nix Commands

| Command | Description |
|---------|-------------|
| `nix develop` | Enter the development shell |
| `nix run .#build` | Build firmware using Docker |
| `nix run .#init` | Initialize git submodules |
| `nix run .#clean` | Clean build output directory |
| `nix run .#ota-stage` | Stage latest build for OTA testing |
| `nix run .#ota-serve` | Start local OTA server (port 8080) |
| `nix run .#ota-status` | Show OTA server status and device commands |
| `nix run .#ota-stop` | Stop the OTA server |
| `nix run .#sub-status` | **Show submodule status and changes** |
| `nix run .#sub-diff` | **Show detailed diff of submodule changes** |
| `nix run .#sub-update` | **Update submodules to latest commits** |
| `nix run .#sub-reset` | **Reset submodules to clean state** |
| `nix run .#sub-commit` | **Commit submodule changes** |
| `nix run .#sub-pull` | **Pull latest changes from submodule remotes** |

## Build Output

After a successful build, artifacts are located at:

```
reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc/
├── *_emmc_ota.zip      # OTA update package
├── *_sdk.tar.gz        # SDK for application development
└── sg2002_recamera_emmc_md5sum.txt
```

## Development Workflow

### Daily Development

1. Enter dev shell: `nix develop`
2. Make changes to customizations
3. Build: `nix run .#build`
4. Test OTA update with local server (see below)

### Testing OTA Updates

After building, stage and serve your firmware for OTA testing:

```bash
nix run .#ota-stage           # Stages as "latest"
nix run .#ota-serve           # Starts server and shows device commands
```

The `ota-serve` command will display the exact commands to run on your reCamera:

```
On the reCamera, run:
  echo '1,http://192.168.1.x:8080/releases/latest/sg2002_recamera_emmc_md5sum.txt' | sudo tee /etc/upgrade
  sudo /mnt/system/upgrade.sh latest
  sudo /mnt/system/upgrade.sh download
  sudo /mnt/system/upgrade.sh start
```

Check status anytime with:

```bash
nix run .#ota-status
```

Stop the server when done:

```bash
nix run .#ota-stop
```

See [`ota_server/README.md`](ota_server/README.md) for advanced configuration.

### Clean Build

```bash
nix run .#clean
```

Or manually:

```bash
rm -rf reCamera-OS/output
```

## Submodule Management

The reCamera-OS directory is a git submodule. Use these commands to manage it:

### Check Submodule Status

```bash
nix run .#sub-status
```

Shows:
- Submodule commit status
- Uncommitted changes
- Modified files

### View Submodule Changes

```bash
nix run .#sub-diff
```

Shows detailed diff of all changes in the submodule.

### Update Submodules

```bash
nix run .#sub-update
```

Updates all submodules to their latest commits from remote.

### Reset Submodules

```bash
nix run .#sub-reset
```

Resets submodules to clean state (discards all changes).

### Commit Submodule Changes

```bash
nix run .#sub-commit "Your commit message"
```

Commits changes in the reCamera-OS submodule. After this, you must also commit the parent repo:

```bash
git add reCamera-OS
git commit -m "Update reCamera-OS submodule"
```

### Pull Submodule Changes

```bash
nix run .#sub-pull
```

Pulls latest changes from submodule remotes.

## Project Structure

```
authority_alert_planning/
├── flake.nix              # Nix flake configuration
├── flake.lock             # Locked dependencies
├── README.md              # This file
├── FLAKE_IMPROVEMENTS.md  # Planned improvements
├── reCamera-OS/           # Firmware source (git submodule)
│   ├── docker_build.sh    # Docker build script
│   ├── .devcontainer/     # Docker image definition
│   └── output/            # Build artifacts (generated)
└── ota_server/            # Local OTA update server
    ├── docker-compose.yml
    ├── prepare_release.sh
    └── nginx/
```

## Troubleshooting

### Docker Permission Issues

If you see permission errors, ensure Docker is running and your user is in the `docker` group:

```bash
sudo usermod -aG docker $USER
# Log out and back in
```

### Submodule Issues

If submodules are missing or corrupted:

```bash
git submodule deinit -f --all
git submodule update --init --recursive
```

Or use the Nix command:

```bash
nix run .#sub-reset
nix run .#init
```

### NixOS `/bin/bash` Warning

On NixOS, `/bin/bash` may not exist. The Docker-based build handles this automatically. For manual builds, use:

```bash
bash docker_build.sh sg2002_recamera_emmc
```

### Build Fails with Nix Environment Variables

The Docker build isolates from Nix environment variables. If building outside Docker (not recommended), you may need to unset:

```bash
unset NIX_LDFLAGS NIX_CFLAGS_COMPILE NIX_CC NIX_BINTOOLS
```

## Contributing

1. Create a feature branch
2. Make changes
3. Test with `nix run .#build`
4. Submit merge request

## License

See individual component licenses in their respective directories.
