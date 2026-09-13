# Authority Alert — reCamera-OS Build Environment

Custom firmware (AuthorityOS) for the reCamera platform, built on Buildroot 2021.05 for the
Sophgo SG2002 RISC-V SoC. Nix provides the host-side tooling; the firmware itself is compiled
inside a Docker container so the build never sees your host toolchain.

## Repository Layout

```
authority-alert/
├── flake.nix                  # Dev shell + `nix run` helper apps
├── flake.lock                 # Pinned nixpkgs (nixos-24.05) + flake-utils
├── .gitattributes             # Git LFS rules for reCamera-OS/host-tools
├── reCamera-OS/               # Firmware source tree (inline, not a submodule)
│   ├── Makefile               # `make <target>` -> external/build.sh
│   ├── docker_build.sh        # Upstream Docker build script
│   ├── .devcontainer/         # Dockerfile for the build image
│   ├── external/configs/      # Target defconfigs
│   ├── external/br2-external/ # Custom Buildroot packages (oobe, supervisor, ...)
│   ├── host-tools/            # Prebuilt cross toolchains (Git LFS)
│   ├── linux_5.10/ u-boot-*/  # Kernel and bootloader sources
│   └── output/                # Build artifacts (generated, gitignored)
├── sscma-example-sg200x/      # SSCMA solutions (inline, has its own flake)
│   └── solutions/             # Camera apps, supervisor, OOBE
├── ota_server/                # Local nginx OTA server for update testing
├── spec/                      # Project specifications
└── plans/                     # Design notes
```

## Prerequisites

| Requirement | Notes |
|---|---|
| Nix with flakes | `experimental-features = nix-command flakes` in `~/.config/nix/nix.conf` |
| Docker | Daemon running, and your user in the `docker` group |
| Git + Git LFS | **LFS is required** — the RISC-V toolchains are LFS objects |
| Disk space | ~40 GB free for a full build tree |
| Time | 1–2 hours for a first build; incremental builds are much faster |

Docker must be usable without `sudo`, because `nix run .#build` invokes `docker` directly:

```bash
sudo usermod -aG docker "$USER"
newgrp docker          # or log out and back in
docker info | grep -i 'server version'
```

## One-Time Setup

### 1. Clone with LFS

```bash
git lfs install
git clone git@github.com:ThePoliceRecord/authority-alert.git
cd authority-alert
git lfs pull
```

If you cloned before running `git lfs install`, run `git lfs pull` from inside the repo.

### 2. Verify the checkout is complete

An incomplete checkout is the single most common cause of a failed build, and it fails
*late* and *confusingly* — `external/setenv.sh` skips missing source trees with a printed
warning instead of aborting, so the build runs on for a while before dying somewhere
unrelated. Check both halves before building.

**Source trees** — every one of these must exist:

```bash
for d in build buildroot-2021.05 cnpy cvi_mpi cvi_rtsp cvibuilder cvikernel cvimath \
         cviruntime external flatbuffers freertos fsbl host-tools isp_tuning ive \
         linux_5.10 opensbi osdrv oss ramdisk tdl_sdk u-boot-2021.10; do
  [ -d "reCamera-OS/$d" ] || echo "MISSING: reCamera-OS/$d"
done
```

**Toolchains** — these are Git LFS objects. `reCamera-OS/host-tools/gcc/` must contain
**ten** directories, including `riscv64-linux-musl-x86_64` (the one this target actually
uses) and `riscv64-linux-x86_64`. Confirm the compilers are real ELF binaries and not
~130-byte LFS pointers:

```bash
ls -l reCamera-OS/host-tools/gcc/riscv64-linux-musl-x86_64/bin/riscv64-unknown-linux-musl-gcc
head -c 40 reCamera-OS/host-tools/gcc/riscv64-linux-musl-x86_64/libexec/gcc/riscv64-unknown-linux-musl/10.2.0/cc1
```

`cc1` should be ~62 MB. If it is missing or starts with `version https://git-lfs.github.com/...`,
run `git lfs pull` before building. A complete `.git/lfs` cache is ~1.2 GB.

> **`git lfs pull` only restores LFS-tracked files** — the 19 paths listed in
> `.gitattributes`, all under `reCamera-OS/host-tools/gcc/`. It will *not* bring back
> `linux_5.10/`, `u-boot-2021.10/`, `opensbi/` or any other ordinary source directory.
> For those, see [Missing source directories](#missing-source-directories) below.

### 3. Enter the dev shell

```bash
nix develop
```

Provides Docker, docker-compose, git, **git-lfs**, GNU make, Python 3 (with `jinja2` and
`pyyaml`), `gh`, `rsync`, `jq`, `unzip`, and `util-linux`. The shell prints a command
cheat-sheet on entry.

You do not strictly need the dev shell to build — `nix run .#build` works from any shell —
but it is the supported environment for the helper scripts.

## Building the Firmware

**Run from the repository root.** The build app resolves `reCamera-OS/` relative to your
current directory and mounts `$(pwd)` into the container, so it will not work from a subdirectory.

```bash
nix run .#build
```

That defaults to the `sg2002_recamera_emmc` target. To pick a different one, pass it after `--`:

```bash
nix run .#build -- sg2002_xiao_sd
```

Valid targets are the defconfigs in `reCamera-OS/external/configs/`:

```bash
ls reCamera-OS/external/configs/
# sg2002_recamera_emmc_defconfig  sg2002_xiao_sd_defconfig
```

### What the build does

1. Builds the Docker image `recamera-os-builder` from `reCamera-OS/.devcontainer/Dockerfile`
   (Ubuntu 24.04 "noble" base, plus Go 1.25.5 for the supervisor) — only if that image does
   not already exist locally.
2. Runs the container as your UID/GID, with the repo bind-mounted at `/work` and a persistent
   container home at `reCamera-OS/output/.docker_home`.
3. Runs `make <target>` inside `/work/reCamera-OS`, which dispatches to
   `external/build.sh <target>`.

The container is started with `-it`, so **the build needs a TTY** — it will fail if you run it
from a context without one (a bare CI runner, a cron job, a piped shell). Use
`reCamera-OS/docker_build.sh` directly, or drop `-it`, in those cases.

The Docker image is cached after the first run. If you change the Dockerfile, force a rebuild:

```bash
docker rmi recamera-os-builder
```

### Alternative: the upstream script

```bash
cd reCamera-OS
./docker_build.sh sg2002_recamera_emmc
```

This is the script the Nix app wraps. It must be run from inside `reCamera-OS/` (its
`Dockerfile` path is relative), and unlike the Nix app it also mounts a `.ccache` directory
and runs `git submodule update --init --recursive` — a no-op in this monorepo.

Building on the host without Docker is not supported here. The Nix environment sets
`NIX_CFLAGS_COMPILE`, `NIX_LDFLAGS`, and friends, which leak into Buildroot's host-tool
compilation and break it.

## Build Outputs

Artifacts land in:

```
reCamera-OS/output/<target>/install/soc_<target>/
```

for example `reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc/`.

Names are `<chip>_<issue>_<version>_<storage>*`, where `<issue>` comes from
`reCamera-OS/external/build/boards/cv181x/<target>/rootfs/etc/issue` (currently `AuthorityOS`)
and `<version>` is the top `## X.Y.Z` heading in `reCamera-OS/CHANGELOG.md`:

| File | Purpose |
|---|---|
| `sg2002_AuthorityOS_<ver>_emmc.zip` | Full image for USB burn (`usb_dl.exe`, Windows) |
| `sg2002_AuthorityOS_<ver>_emmc_ota.zip` | OTA update package (raw images + checksums) |
| `sg2002_AuthorityOS_<ver>_emmc_recovery.zip` | SD card recovery image; reflashes eMMC on boot |
| `sg2002_AuthorityOS_<ver>_emmc_sd_compat.zip` | SD card image that boots directly from SD |
| `sg2002_AuthorityOS_<ver>_emmc.swu` | Raw swupdate bundle (cpio of `sw-description` + rootfs) |
| `sg2002_AuthorityOS_<ver>_emmc_swu.zip` | The above, zipped |
| `sg2002_AuthorityOS_<ver>_emmc_sdk.tar.gz` | SDK for application development (incl. the TPU SDK) |
| `upgrade.zip` | Intermediate that `gen_emmc_zip` copies to `*_emmc.zip`; not checksummed |
| `sg2002_recamera_emmc_sha256sum.txt` | SHA256 manifest, named after the *target*, not the image |

The manifest covers only files matching `<target_name>*.zip` — so the five zips, but not
`upgrade.zip`, the raw `.swu`, or the SDK tarball. Verify with
`sha256sum -c sg2002_recamera_emmc_sha256sum.txt`.

A verified 0.2.11 build produces roughly: the four `*_emmc*.zip` images at ~102 MB each,
`*.swu` at ~509 MB, and the SDK tarball at ~357 MB.

## Nix Commands

All commands are run from the repository root.

| Command | Description |
|---|---|
| `nix develop` | Enter the development shell |
| `nix run .#build [-- <target>]` | Build firmware in Docker (default `sg2002_recamera_emmc`) |
| `nix run .#clean` | Delete `reCamera-OS/output` entirely |
| `nix run .#clean-external` | Delete only br2-external package build dirs, forcing their rebuild |
| `nix run .#ota-stage [-- <ver>]` | Stage the newest OTA zip with a SHA256 manifest |
| `nix run .#ota-stage-md5 [-- <ver>]` | Stage with an MD5 manifest (older `upgrade.sh`) |
| `nix run .#ota-stage-both [-- <ver>]` | Stage with both manifests |
| `nix run .#ota-serve` | Start the local OTA server on port 8080 |
| `nix run .#ota-status` | Show server state, staged releases, and device commands |
| `nix run .#ota-stop` | Stop the OTA server |
| `nix run .#ota-clean` | Delete all staged releases (prompts for confirmation) |
| `nix run .#flash-recovery [-- <ver>]` | Write a recovery image to an SD card (interactive) |
| `nix run .#release [-- --version X.Y.Z] [--force] [--dry-run]` | Tag and push a release |

`reCamera-OS/` and `sscma-example-sg200x/` each carry their own flake, giving standalone
shells for working on those components alone. The root flake is the one you want for
normal firmware work.

### Rebuilding just a custom package

Editing something under `reCamera-OS/external/br2-external/` (`oobe`, `sscma-supervisor`,
`reCamera`, …) does not always retrigger a Buildroot rebuild. Clear those package build
directories and rebuild:

```bash
nix run .#clean-external
nix run .#build
```

That is far cheaper than `nix run .#clean`, which throws away the whole tree and costs you
another full build.

## Testing OTA Updates

```bash
nix run .#ota-stage     # copy the newest *_emmc_ota.zip into ota_server/ota_content/releases/
nix run .#ota-serve     # start nginx on :8080 and print the device commands
```

`ota-stage` stages under `releases/latest/` unless you pass an explicit version
(`nix run .#ota-stage -- 0.2.11`). Then, on the reCamera over SSH:

```bash
echo '1,http://<host-ip>:8080/releases/latest/sg2002_recamera_emmc_sha256sum.txt' | sudo tee /etc/upgrade
sudo /mnt/system/upgrade.sh latest
sudo /mnt/system/upgrade.sh download
sudo /mnt/system/upgrade.sh start
```

`nix run .#ota-serve` and `nix run .#ota-status` print these with your actual LAN IP filled in.
Stop the server with `nix run .#ota-stop`. See [`ota_server/README.md`](ota_server/README.md)
for nginx configuration details.

## Flashing

- **SD recovery (easiest):** `nix run .#flash-recovery` lists removable block devices, asks
  you to pick one, requires typing `yes`, then `dd`s the image. It refuses to touch `nvme*`
  and `loop*` devices. Insert the card into the board and power on; it reflashes eMMC
  automatically.
- **USB burn (Windows):** unzip `*_emmc.zip` and run
  `usb_dl.exe -c cv181x -s linux -i ..\sg2002_AuthorityOS_<ver>_emmc [-m <mac>]`.
- **Boot from SD:** write `*_emmc_sd_compat.zip` to an SD card with balenaEtcher.
- **OTA:** see above.

## Cutting a Release

```bash
nix run .#release -- --dry-run     # preview
nix run .#release                  # tag + push
```

The version is the top `## X.Y.Z` heading in `reCamera-OS/CHANGELOG.md`, overridable with
`--version`. The working tree must be clean. Re-running is idempotent if the tag already
points at `HEAD`; otherwise it fails and tells you to bump the changelog or pass `--force`
(which deletes the *local* tag only — remote tags are never rewritten). This tags this
repository only.

## Troubleshooting

### Missing source directories

Symptom — `setenv.sh` cannot `pushd` into a source tree, and the kernel patch step then runs
in the wrong directory and prompts interactively for a file to patch:

```
setenv.sh: line 54: pushd: /work/reCamera-OS/output/<target>/linux*: No such file or directory
Applying patch: 001_net_phy_cvitek_revert_read_status.patch
can't find file to patch at input line 5
File to patch:
```

The literal `linux*` in that path is the tell: `build.sh` resolves the kernel directory with
`basename $(realpath $TOPDIR/linux*)`, and an unmatched glob stays literal. It means
`reCamera-OS/linux_5.10/` is absent from your working tree. `rsync_dir` then printed
`./linux* not exist` and *returned rather than failing*, so the build continued regardless.

This is **not** an LFS problem — those directories are ordinary tracked files. Check what is
actually missing versus `HEAD`:

```bash
git ls-files -d | head          # tracked files deleted from the working tree
git ls-files | wc -l            # should be ~207,000, not a handful
```

If the index itself has been emptied (a stray `git rm --cached -r .`, an interrupted LFS
migration), rebuild it from `HEAD` and restore only the deleted files — this preserves any
uncommitted edits you have:

```bash
git reset                                        # index only; working tree untouched
nix develop --command bash -c \
  'git ls-files -d -z | xargs -0 -n 2000 git checkout --'
```

Run the restore **inside `nix develop`**: the repo has `filter.lfs` configured in
`.git/config`, so `git checkout` invokes `git-lfs`, and it will abort with
`smudge filter lfs failed / git-lfs: command not found` if the binary isn't on `PATH`.
Afterwards, run `nix run .#clean` — a build that died this way leaves a partially rsynced
output tree, including a literal `u-boot*` directory.

### Files dropped by a nested .gitignore ("No rule to make target")

Symptom — make demands an object file that has no corresponding source:

```
make[5]: *** No rule to make target '.../isp/cv181x/isp_algo/obj/dpcm_api.riscv64-unknown-linux-musl.cv181x.o',
         needed by '.../lib/libisp_algo.a'.  Stop.
```

The Sophgo ISP algorithms are **proprietary and ship as prebuilt `.o` files** — upstream
`cvi_mpi` has no `isp_algo/src/` at all, just `obj/`. When commit `6f05a0d9b` inlined the
submodules, `git add` honored the nested `.gitignore` files and silently skipped every
prebuilt blob they matched. Two rules were responsible:

- `cvi_mpi/modules/isp/.gitignore` — `*.o`, `*.a`, `*.so`, `bin`, plus `isp_version.h`,
  `cvi_pqtool_json.h`, `pqtool_definition.json`. Cost: **562 files**, including all 557
  prebuilt ISP objects.
- `reCamera-OS/.gitignore` — an unanchored `output*/`, which matches *any* nested directory
  named `output`, not just the build tree. Cost: cvi_rtsp's vendored
  `nlohmann_json/include/nlohmann/detail/output/*.hpp`, which `json.hpp` `#include`s.

Both `.gitignore` files now carry narrowly scoped negations / anchoring to prevent a
recurrence. If you hit this for some *other* path, recover it from the exact upstream
revision the monorepo was built from rather than guessing a branch tip:

```bash
# 1. the reCamera-OS submodule SHA recorded just before inlining
git ls-tree 6f05a0d9b^ -- reCamera-OS
# 2. that commit's own submodule SHAs + URLs (authority-alert-OS still has them)
git init -q /tmp/osprobe && git -C /tmp/osprobe fetch --depth=1 \
  git@github.com:ThePoliceRecord/authority-alert-OS.git <sha-from-step-1>
git -C /tmp/osprobe ls-tree FETCH_HEAD          # gitlink SHA per component
git -C /tmp/osprobe show FETCH_HEAD:.gitmodules # upstream URL per component
# 3. shallow-fetch that exact revision and copy the missing paths in
```

Pinning the recorded SHA matters: these are prebuilt binaries, so a branch-tip mismatch
against the inlined headers can fail at link time or misbehave at runtime. For reference,
`cvi_mpi` is `sophgo/cvi_mpi` @ `6dbbbb8bed877cd3a08f76ce4aed61645c38bbe1`, which diffs
clean against the inlined sources.

Known-absent and harmless (no build path touches them): ~100 upstream files dropped by
`*.pem` (hostap test certs, arm64 python certs), `build*` (a tdl_sdk doc), `.project`, and
`/.settings/` (FreeRTOS demos for unrelated boards).

**Build fails inside Buildroot with missing/garbled compiler**
LFS objects were not fetched. See [Verify the checkout is complete](#2-verify-the-checkout-is-complete).

**`Cannot connect to the Docker daemon`**
`sudo systemctl start docker`, and confirm `groups` lists `docker`.

**`the input device is not a TTY`**
The build container runs with `-it`. Run it from an interactive terminal.

**Permission errors on `reCamera-OS/output`**
The container chowns `output/` to your UID on each run. If a previous root-owned build left
artifacts behind: `sudo chown -R "$USER:$USER" reCamera-OS/output`.

**Changes to a br2-external package are ignored**
Run `nix run .#clean-external`, then rebuild.

**Dockerfile changes have no effect**
The image is only built when absent. `docker rmi recamera-os-builder` first.

**On NixOS, `/bin/bash` does not exist**
Harmless — everything that matters runs inside the container. The dev shell prints a note
about it on entry.

## Known Gotchas

- `nix run .#ota-stage` cannot auto-detect the version from the artifact filename: it looks
  for `_reCamera_`, but images are now named `_AuthorityOS_`. It therefore always falls back
  to staging under `latest`. Pass the version explicitly if you want a versioned directory.
- `.gitlab-ci.yml` looks like it predates the monorepo restructure: it runs
  `./docker_build.sh` and archives `output/` from the repo root, but both now live under
  `reCamera-OS/`. It also uses `docker run -it`-based tooling on a runner. Treat CI as
  unverified.
- `external/setenv.sh`'s `rsync_dir` helper `return`s on a missing source directory instead
  of exiting, so an incomplete checkout produces a late, misleading failure rather than a
  clear one. Hence the pre-build check in step 2.
- **The build is not cleanly re-runnable after a mid-build failure.** `setenv.sh`
  unconditionally re-applies `external/linux/patches/*.patch` to
  `output/<target>/linux_5.10/`, with `patch -p1 < "$patch" || exit 1`. On a second run the
  patch is already applied, `patch` asks `Assume -R?` with the patch file itself on stdin,
  and the build dies in `setenv.sh`. Either run `nix run .#clean` (full rebuild), or revert
  the patch in place first and keep everything else:
  ```bash
  cd reCamera-OS/output/<target>/linux_5.10
  patch -p1 -R < ../../../external/linux/patches/001_net_phy_cvitek_revert_read_status.patch
  ```
  Deleting just `output/<target>/linux_5.10/` also works — `rsync_dir` repopulates it
  pristine — at the cost of a kernel rebuild.
- `flake.lock` pins `nixos-24.05`, which is past end-of-life. It still evaluates and builds
  the dev shell; bump it when convenient.

## Contributing

1. Branch off `development`.
2. Make changes.
3. Verify with `nix run .#build`.
4. Update `reCamera-OS/CHANGELOG.md`.
5. Open a merge request.

## License

See individual component licenses in their respective directories.
