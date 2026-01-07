# Nix Flake Improvements for reCamera-OS Build

This document contains planned improvements for `flake.nix` to better support
cross-compilation and developer workflow for the reCamera-OS project.

## Project Context

- **Target**: SG2002 RISC-V SoC
- **Defconfig**: `sg2002_recamera_emmc_defconfig`
- **Toolchain**: `riscv64-unknown-linux-musl-` (pre-built in `host-tools/gcc/riscv64-linux-musl-x86_64/`)
- **Build System**: Custom wrapper around Buildroot 2021.05
- **Current Version**: 0.2.1

### Build Flow
```
make sg2002_recamera_emmc
    → external/build.sh
        → external/setenv.sh (rsync sources, apply patches)
        → build/envsetup_soc.sh (set variables)
        → defconfig, build_all
        → gen_*_zip (package outputs)
```

---

## High Priority - Cross-Compilation Isolation

### 1. Unset Additional NIX Variables

Add to `profile` section after existing `unset` statements:

```nix
# Additional NIX variables to unset for cross-compilation isolation
unset NIX_HARDENING_ENABLE   # Prevents -fstack-protector pollution in target builds
unset NIX_CC                 # Prevent host compiler wrapper interference
unset NIX_BINTOOLS           # Prevent host bintools wrapper interference
unset PKG_CONFIG_PATH        # Host pkg-config can break target builds
unset PKG_CONFIG_LIBDIR      # Same as above
unset NIX_PKG_CONFIG_WRAPPER_TARGET_HOST_x86_64_unknown_linux_gnu
```

### 2. Pre-configure RISC-V Toolchain Path

The build uses pre-built toolchains in `host-tools/gcc/`. Add to `profile`:

```nix
# Configure pre-built RISC-V toolchain (used by Buildroot external toolchain)
if [ -d "reCamera-OS/host-tools/gcc/riscv64-linux-musl-x86_64" ]; then
  export RISCV_LINUX_MUSL_PATH="$(pwd)/reCamera-OS/host-tools/gcc/riscv64-linux-musl-x86_64"
  # Optionally add to PATH for manual testing
  # export PATH="$RISCV_LINUX_MUSL_PATH/bin:$PATH"
fi
```

---

## Medium Priority - Developer Convenience

### 3. Add Build Convenience Apps

Add to the `apps` section:

```nix
# Clean build directory
apps.clean = {
  type = "app";
  program = toString (pkgs.writeShellScript "clean-recamera" ''
    set -e
    if [ -d "reCamera-OS/output/sg2002_recamera_emmc" ]; then
      rm -rf reCamera-OS/output/sg2002_recamera_emmc
      echo "Cleaned build directory: reCamera-OS/output/sg2002_recamera_emmc"
    else
      echo "Build directory already clean"
    fi
  '');
};

# Run Buildroot menuconfig
apps.menuconfig = {
  type = "app";
  program = toString (pkgs.writeShellScript "menuconfig" ''
    BR_OUTPUT="reCamera-OS/output/sg2002_recamera_emmc/buildroot-2021.05/output/cvitek_CV181X_musl_riscv64"
    if [ ! -d "$BR_OUTPUT" ]; then
      echo "Error: Build directory not found. Run build first."
      exit 1
    fi
    ${fhsEnv}/bin/recamera-os-fhs -c "make -C reCamera-OS/output/sg2002_recamera_emmc/buildroot-2021.05 menuconfig O=output/cvitek_CV181X_musl_riscv64"
  '');
};

# Linux kernel menuconfig
apps.linux-menuconfig = {
  type = "app";
  program = toString (pkgs.writeShellScript "linux-menuconfig" ''
    ${fhsEnv}/bin/recamera-os-fhs -c "make -C reCamera-OS/output/sg2002_recamera_emmc/linux_5.10 menuconfig ARCH=riscv"
  '');
};

# Initialize git submodules
apps.init = {
  type = "app";
  program = toString (pkgs.writeShellScript "init-recamera" ''
    set -e
    cd reCamera-OS
    echo "Initializing git submodules..."
    git submodule update --init --recursive --depth 1
    echo "Done!"
  '');
};
```

### 4. Add ccache for Faster Rebuilds

Add to `targetPkgs`:

```nix
ccache
```

Add to `profile`:

```nix
# Enable ccache for faster rebuilds
export USE_CCACHE=1
export CCACHE_DIR="''${CCACHE_DIR:-$HOME/.ccache}"
export BR2_CCACHE=y
export BR2_CCACHE_DIR="$CCACHE_DIR"

# Create ccache directory if it doesn't exist
mkdir -p "$CCACHE_DIR" 2>/dev/null || true
```

### 5. Parallel Build Settings

Add to `profile`:

```nix
# Optimize parallel builds
export MAKEFLAGS="-j$(nproc)"
export BR2_JLEVEL="$(nproc)"

# Show build parallelism info
echo " Parallel jobs: $(nproc)"
```

---

## Low Priority - Additional Tools

### 6. Add Missing Build Dependencies

Add to `targetPkgs` (found from `.devcontainer/Dockerfile`):

```nix
# Additional tools for image generation
# Note: android-tools provides simg2img, img2simg for sparse images
# This may require: nixpkgs with android-tools or manual alternative

# ELF utilities (for debugging cross-compiled binaries)
libelf
libelf.dev
elfutils  # provides eu-strip, eu-readelf, etc.

# GCC cross-compilation dependencies (if building toolchain from source)
gmp
gmp.dev
mpfr
mpfr.dev
libmpc
```

### 7. Add QEMU for Testing RISC-V Binaries

Add to `targetPkgs`:

```nix
# QEMU for running/testing RISC-V binaries on x86_64 host
qemu
```

Add helper function to `profile`:

```nix
# Helper to run RISC-V binaries via QEMU
run_riscv() {
  qemu-riscv64 -L reCamera-OS/host-tools/gcc/riscv64-linux-musl-x86_64/sysroot "$@"
}
```

### 8. Add SDK Extraction Helper

Add to `apps`:

```nix
# Extract SDK from build output
apps.sdk = {
  type = "app";
  program = toString (pkgs.writeShellScript "extract-sdk" ''
    set -e
    SDK_DIR="reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc"
    if [ ! -d "$SDK_DIR" ]; then
      echo "Error: Build not complete. SDK directory not found."
      exit 1
    fi
    SDK_TAR=$(find "$SDK_DIR" -name "*_sdk.tar.gz" | head -1)
    if [ -z "$SDK_TAR" ]; then
      echo "Error: SDK tarball not found in $SDK_DIR"
      exit 1
    fi
    echo "Found SDK: $SDK_TAR"
    echo "Extract with: tar -xzf $SDK_TAR"
  '');
};
```

---

## Additional Improvements

### 9. Better Shell Banner

Update the banner in `profile` to show more useful info:

```nix
echo "==============================================="
echo " reCamera-OS / Authority Alert FHS Dev Shell"
echo "==============================================="
echo " Python:      $(python --version 2>&1)"
echo " GCC:         $(gcc --version | head -1)"
echo " Make:        $(make --version | head -1)"
echo " Parallel:    $(nproc) cores"
echo " ccache:      ''${CCACHE_DIR:-disabled}"
echo " /bin/bash:   available"
echo "==============================================="
echo ""
echo "Quick commands:"
echo "  nix run .#init    - Initialize git submodules"
echo "  nix run .#build   - Build sg2002_recamera_emmc"
echo "  nix run .#clean   - Clean build directory"
echo "  nix run .#sdk     - Show SDK location"
echo ""
echo "Manual build:"
echo "  cd reCamera-OS && make sg2002_recamera_emmc"
echo ""
```

### 10. Environment Variable Reference

Add to `profile` for debugging:

```nix
# Debug: show key environment variables
show_env() {
  echo "=== Build Environment ==="
  echo "TOPDIR:              ''${TOPDIR:-not set}"
  echo "PROJECT_OUT:         ''${PROJECT_OUT:-not set}"
  echo "BUILDROOT_DIR:       ''${BUILDROOT_DIR:-not set}"
  echo "LINUX_DIR:           ''${LINUX_DIR:-not set}"
  echo "RISCV_TOOLCHAIN_PATH: ''${RISCV_TOOLCHAIN_PATH:-not set}"
  echo "CCACHE_DIR:          ''${CCACHE_DIR:-not set}"
  echo "========================="
}
```

---

## Files Modified

When implementing these changes, update:

1. `flake.nix` - Main configuration file
2. This file (`FLAKE_IMPROVEMENTS.md`) - Remove implemented items

## Testing

After implementing changes, test with:

```bash
# Test shell entry
nix develop

# Test apps
nix run .#init
nix run .#build
nix run .#clean

# Test cross-compilation isolation
nix develop -c env | grep -E '^(NIX_|PKG_CONFIG)'  # Should be empty
```
