{
  description = "reCamera-OS / Authority Alert build environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        # Use overlay to ensure libxcrypt has all hash algorithms (SHA-256, SHA-512, etc.)
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        # PCRE with additional soname symlinks for compatibility with prebuild binaries
        # Some prebuild tools (like make_ext4fs) are compiled on Ubuntu/Debian which uses
        # libpcre.so.3 soname, while NixOS PCRE uses libpcre.so.1
        pcreCompat = pkgs.pcre.overrideAttrs (oldAttrs: {
          postInstall = (oldAttrs.postInstall or "") + ''
            # Create .so.3 symlinks for Ubuntu/Debian binary compatibility
            ln -sf libpcre.so.1 $out/lib/libpcre.so.3
            ln -sf libpcreposix.so.0 $out/lib/libpcreposix.so.3
          '';
        });

        # Python with required packages
        pythonEnv = pkgs.python3.withPackages (ps: with ps; [
          jinja2
          pyyaml
          pip
          kconfiglib  # for menuconfig/defconfig
        ]);

        # FHS environment for build compatibility
        fhsEnv = pkgs.buildFHSEnv {
          name = "recamera-os-fhs";
          
          targetPkgs = _pkgs: with pkgs; [
            # Build essentials
            gnumake
            gcc
            glibc
            glibc.static
            binutils
            cmake
            ninja
            automake
            autoconf
            libtool
            pkg-config
            patchelf

            # SCons / parallel build
            scons
            parallel

            # Compression & filesystem tools
            squashfsTools
            cpio
            fakeroot
            mtools
            bzip2
            bzip2.dev  # for bzlib.h
            zlib
            zlib.dev
            xz
            xz.dev  # for lzma.h
            zip
            unzip
            e2fsprogs  # resize2fs, mke2fs, etc.

            # Kernel / bootloader build
            flex
            bison
            bc
            dtc  # device-tree-compiler
            openssl
            openssl.dev
            ncurses
            ncurses.dev  # for menuconfig

            # Crypt libraries (needed for host-mkpasswd and host-python in Buildroot)
            # libxcrypt provides libcrypt.so.2 (needed by pre-compiled host binaries)
            # libxcrypt-legacy provides libcrypt.so.1 with full hash support (SHA-256/SHA-512)
            libxcrypt
            libxcrypt-legacy

            # Misc build tools
            wget
            curl
            git
            jq
            tcl
            tree
            rsync
            xxd
            file
            which
            patch
            diffutils

            # Python environment
            pythonEnv

            # Node.js (for potential JS tooling)
            nodejs_18

            # SSH client
            openssh

            # PCRE for regex (pcreCompat has .so.3 symlinks for prebuild tool compat)
            pcreCompat
            pcreCompat.dev

            # CA certificates
            cacert

            # GNU coreutils
            coreutils
            findutils
            gnugrep
            gnused
            gawk
            gnutar
            gzip

            # Bash (provides /bin/bash in FHS)
            bash
            bashInteractive

            # Additional tools
            lsb-release
            gnupg
            perl

            # Docker (optional, for docker_build.sh)
            docker

            # 32-bit libraries for cross compilation
            pkgsi686Linux.glibc
          ];

          multiPkgs = _pkgs: with pkgs; [
            zlib
            glibc
            libxcrypt
            libxcrypt-legacy
            pcreCompat
          ];

          runScript = "bash";

          profile = ''
            export NIX_SHELL_NAME="recamera-os-fhs"
            
            # CRITICAL: Sanitize LD_LIBRARY_PATH for Buildroot compatibility
            # Buildroot fails if LD_LIBRARY_PATH contains empty paths (::) or current dir (.)
            if [ -n "$LD_LIBRARY_PATH" ]; then
              # Remove empty path components (::), trailing/leading colons, and current dir
              LD_LIBRARY_PATH=$(echo "$LD_LIBRARY_PATH" | sed -e 's/::/:/g' -e 's/^://' -e 's/:$//' -e 's/:\.:/:/g' -e 's/^\.://' -e 's/:\.$//')
              export LD_LIBRARY_PATH
            fi
            
            # Add FHS library paths for prebuild binaries (make_ext4fs, etc.)
            # These binaries expect libraries in standard FHS paths like /usr/lib
            export LD_LIBRARY_PATH="/usr/lib:/lib''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
            
            # Git safe directory
            git config --global --add safe.directory "$(pwd)" 2>/dev/null || true
            
            # Locate host toolchains if present
            if [ -d "reCamera-OS/host-tools/gcc" ]; then
              export RISCV_TOOLCHAIN_PATH="$(pwd)/reCamera-OS/host-tools/gcc"
            fi

            # Enable bash completion features for interactive use
            shopt -s progcomp 2>/dev/null || true

            echo "==============================================="
            echo " reCamera-OS / Authority Alert FHS Dev Shell"
            echo " Python: $(python --version 2>&1)"
            echo " GCC: $(gcc --version | head -1)"
            echo " Make: $(make --version | head -1)"
            echo " /bin/bash: available"
            echo "==============================================="
            echo ""
            echo "Build commands:"
            echo "  cd reCamera-OS && make sg2002_recamera_emmc"
            echo ""
            echo "Or run a single build command:"
            echo "  nix run .#default -- -c 'make -C reCamera-OS sg2002_recamera_emmc'"
            echo ""
          '';
        };

      in {
        # The FHS dev shell for interactive use
        devShells.default = fhsEnv.env;

        # Package for nix run commands
        packages.default = fhsEnv;

        # Convenience app for building
        apps.build = {
          type = "app";
          program = toString (pkgs.writeShellScript "build-recamera" ''
            ${fhsEnv}/bin/recamera-os-fhs -c "make -C reCamera-OS ''${1:-sg2002_recamera_emmc}"
          '');
        };
      });
}
