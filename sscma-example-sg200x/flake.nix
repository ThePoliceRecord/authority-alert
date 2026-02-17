{
  description = "SSCMA Example for SG200X - Development Environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # Fetch host-tools from GitHub archive zip
        # Note: If toolchains are in Git LFS, this won't include them!
        # We'll test and see...
        host-tools = pkgs.fetchzip {
          url = "https://github.com/sophgo/host-tools/archive/refs/heads/master.zip";
          sha256 = "sha256-OARUHjWRIcsKo0LVm1T4/CBaf2Lis3YKO9ZXfC5KD8E=";
          stripRoot = true;
        };

        # Toolchain path inside host-tools repo
        toolchainSubdir = "gcc/riscv64-linux-musl-x86_64";

        # Build target for the OS SDK
        buildTarget = "sg2002_recamera_emmc";
        
        # SDK path will be set at runtime from the actual filesystem
        # We can't use relative paths in Nix because they get copied to the store
        # Instead, we'll check for the SDK at runtime in the shell profile
        sdkPath = null;  # Will be detected at runtime

        # FHS environment for running the toolchain
        fhsEnv = pkgs.buildFHSEnv {
          name = "sscma-sg200x-fhs";

          targetPkgs = pkgs: with pkgs; [
            # Build tools
            cmake
            ninja
            gnumake
            pkg-config
            go
            nodejs

            # Version control
            git

            # Deploy tools
            openssh
            rsync

            # Debugging/analysis
            file
            binutils

            # Required for toolchain
            stdenv.cc.cc.lib
            zlib
            ncurses5
            python3
            glibc
          ];

          profile = ''
            # Project root
            export PROJECT_ROOT="$(pwd)"

            # Toolchain from Nix store (fetched from sophgo/host-tools)
            export TOOLCHAIN_PATH="${host-tools}/${toolchainSubdir}"
            export PATH="$TOOLCHAIN_PATH/bin:$PATH"
            
            echo ""
            echo "════════════════════════════════════════════════════════════"
            echo "  SSCMA SG200X Development Environment"
            echo "════════════════════════════════════════════════════════════"
            echo ""
            echo "✓ Toolchain: ${host-tools}/${toolchainSubdir}"
            
            # Detect SDK at runtime from the actual filesystem.
            # Support both the historical repo name (reCamera-OS) and the newer one (authority-alert-OS).
            OS_REPO_PATH=""
            for candidate in \
              "$PROJECT_ROOT/../authority-alert-OS" \
              "$PROJECT_ROOT/../authority-alert-os" \
              "$PROJECT_ROOT/../reCamera-OS" \
              "$PROJECT_ROOT/../reCamera-os"
            do
              if [ -d "$candidate" ]; then
                OS_REPO_PATH="$candidate"
                break
              fi
            done

            SDK_INSTALL_PATH="$OS_REPO_PATH/output/${buildTarget}/install/soc_${buildTarget}"
            
            if [ -n "$OS_REPO_PATH" ] && [ -d "$SDK_INSTALL_PATH" ]; then
              # Check if SDK needs to be extracted
              SDK_TARBALL="$(ls -1t "$SDK_INSTALL_PATH"/*_sdk.tar.gz 2>/dev/null | head -n 1)"
              SDK_EXTRACTED_MARKER="$SDK_INSTALL_PATH/${buildTarget}"
              
              if [ -f "$SDK_TARBALL" ] && [ ! -d "$SDK_EXTRACTED_MARKER" ]; then
                echo ""
                echo "📦 Extracting SDK (first time setup)..."
                tar -xzf "$SDK_TARBALL" -C "$SDK_INSTALL_PATH" 2>/dev/null
                if [ $? -eq 0 ]; then
                  echo "✓ SDK extracted successfully"
                else
                  echo "⚠ Failed to extract SDK"
                fi
              fi
              
              # Create symlinks if needed (for backwards compatibility with CMakeLists.txt)
              if [ -d "$SDK_EXTRACTED_MARKER" ]; then
                if [ ! -L "$SDK_INSTALL_PATH/cvi_mpi" ]; then
                  echo "🔗 Creating SDK symlinks..."
                  ln -sf "${buildTarget}/cvi_mpi" "$SDK_INSTALL_PATH/cvi_mpi" 2>/dev/null
                  ln -sf "${buildTarget}/osdrv" "$SDK_INSTALL_PATH/osdrv" 2>/dev/null
                  echo "✓ SDK symlinks created"
                fi
              fi
              
              export SG200X_SDK_PATH="$SDK_INSTALL_PATH"
              export TPU_SDK_PATH="$SDK_INSTALL_PATH/tpu_musl_riscv64/cvitek_tpu_sdk"
              
              echo "✓ SDK (manual build): $SG200X_SDK_PATH"
              
              if [ -d "$TPU_SDK_PATH" ]; then
                echo "✓ TPU SDK: $TPU_SDK_PATH"
                export PKG_CONFIG_PATH="$TPU_SDK_PATH/lib/pkgconfig:$PKG_CONFIG_PATH"
                export LD_LIBRARY_PATH="$TPU_SDK_PATH/lib:$LD_LIBRARY_PATH"
              fi
            else
              echo "⚠ No SDK found"
              echo ""
              echo "Build SDK in the OS repo (sibling directory):"
              echo "  cd ../authority-alert-OS  # or ../reCamera-OS"
              echo "  nix develop  # Then run docker_build.sh"
            fi

            echo ""
            # Verify toolchain
            if command -v riscv64-unknown-linux-musl-gcc &> /dev/null; then
              echo "✓ Compiler: $(riscv64-unknown-linux-musl-gcc --version | head -1)"
            else
              echo "⚠ Toolchain compiler not found in PATH"
            fi

            echo ""
            echo "════════════════════════════════════════════════════════════"
            echo ""
            echo "Quick Start:"
            echo "  cd solutions/helloworld"
            echo "  cmake -B build -DCMAKE_BUILD_TYPE=Release ."
            echo "  cmake --build build"
            echo ""
            echo "════════════════════════════════════════════════════════════"
            echo ""
          '';

          runScript = "bash";
        };

      in
      {
        devShells.default = pkgs.mkShell {
          name = "sscma-sg200x";

          buildInputs = [ fhsEnv ];

          shellHook = ''
            # Clear NoMachine's LD_PRELOAD to avoid warnings in Nix shell
            unset LD_PRELOAD

            echo "════════════════════════════════════════════════════════════"
            echo " Starting SSCMA SG200X Development Environment"
            echo "════════════════════════════════════════════════════════════"
            exec ${fhsEnv}/bin/sscma-sg200x-fhs
          '';
        };
      });
}
