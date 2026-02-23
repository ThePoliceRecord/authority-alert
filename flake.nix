{
  description = "reCamera-OS / Authority Alert build environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        # Python with required packages for local development
        pythonEnv = pkgs.python3.withPackages (ps: with ps; [
          jinja2
          pyyaml
          pip
        ]);

      in {
        # Minimal dev shell for Docker-based builds
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Docker for official build method
            docker
            docker-compose
            
            # Git for version control
            git
            
            # GitHub CLI (used by nix run .#release for PR/merge workflows)
            gh
            
            # Python for scripts
            pythonEnv
            
            # Basic utilities
            gnumake
            bash
            coreutils
            findutils
            gnugrep
            gnused
            gawk
            
            # Optional: for manual inspection
            tree
            jq
            rsync
          ];

          shellHook = ''
            # Create /bin/bash symlink if it doesn't exist (NixOS compatibility)
            if [ ! -e /bin/bash ]; then
              echo "Note: /bin/bash not found. Run docker_build.sh with: bash docker_build.sh"
            fi
            
            echo "==============================================="
            echo " reCamera-OS / Authority Alert Dev Shell"
            echo "==============================================="
            echo " Docker:  $(docker --version 2>&1 || echo 'not available')"
            echo " Python:  $(python --version 2>&1)"
            echo " Git:     $(git --version 2>&1)"
            echo " Bash:    $(bash --version | head -1)"
            echo "==============================================="
            echo ""
            echo "Build reCamera-OS SDK:"
            echo "  nix run .#build"
            echo ""
            echo "Clean build artifacts:"
            echo "  nix run .#clean           # Remove entire output directory"
            echo "  nix run .#clean-external  # Clean only br2-external packages (force rebuild)"
            echo ""
            echo "Cut a release (tags all 3 repos using the top version in reCamera-OS/CHANGELOG.md):"
            echo "  nix run .#release"
            echo ""
            echo "OTA server workflow:"
            echo "  nix run .#ota-stage      # Stage latest build (SHA256 manifest)"
            echo "  nix run .#ota-stage-md5  # Stage latest build (MD5 manifest + md5sum.txt in zip)"
            echo "  nix run .#ota-stage-both # Stage latest build (both SHA256 + MD5 manifests)"
            echo "  nix run .#ota-serve      # Start server"
            echo "  nix run .#ota-status     # Check status"
            echo "  nix run .#ota-stop       # Stop server"
            echo "  nix run .#ota-clean      # Remove all releases"
            echo ""
            echo "Flash recovery image to SD card:"
            echo "  nix run .#flash-recovery      # Flash latest recovery image"
            echo "  nix run .#flash-recovery 0.2.6 # Flash specific version"
            echo ""
            echo "Component Nix flakes (use independently):"
            echo "  cd reCamera-OS && nix develop          # SDK build environment"
            echo "  cd sscma-example-sg200x && nix develop # SSCMA development"
            echo ""
            echo "Note: Large toolchain files use Git LFS."
            echo "  git lfs pull  # Fetch toolchains if needed"
            echo ""
            echo "Docker build uses official Ubuntu 20.04 environment"
            echo "and avoids cross-compilation issues."
            echo ""
          '';
        };

        # Convenience apps for Docker builds
        apps = {
          # Build using Docker (official method)
          # Wrapper that handles git submodule setup
          build = {
            type = "app";
            program = toString (pkgs.writeShellScript "docker-build" ''
              set -e
              
              TARGET=''${1:-sg2002_recamera_emmc}
              IMAGE_NAME="recamera-os-builder"
              DOCKERFILE="reCamera-OS/.devcontainer/Dockerfile"
              PROJECT_ROOT="$(pwd)"
              
              # Build Docker image if needed
              if ! ${pkgs.docker}/bin/docker image inspect "$IMAGE_NAME" &>/dev/null; then
                echo "Building Docker image..."
                ${pkgs.docker}/bin/docker build -t "$IMAGE_NAME" -f "$DOCKERFILE" reCamera-OS/
              fi
              
              echo "============================================="
              echo "Building $TARGET with Docker..."
              echo "============================================="
              
              # Run Docker with parent directory mounted so .git/modules is accessible
              HOST_UID=$(id -u)
              HOST_GID=$(id -g)
              HOST_UNAME=''${USER:-builder}
              
              ${pkgs.docker}/bin/docker run --rm -it \
                -e HOST_UID=$HOST_UID -e HOST_GID=$HOST_GID -e HOST_UNAME=$HOST_UNAME \
                -e HOME=/home/$HOST_UNAME \
                -v "$PROJECT_ROOT":/work \
                -v "$PROJECT_ROOT/reCamera-OS/output/.docker_home":/home/$HOST_UNAME \
                --workdir /work/reCamera-OS \
                "$IMAGE_NAME" \
                /bin/bash -c "
              set -e
              if ! id \"\$HOST_UNAME\" >/dev/null 2>&1; then
                groupadd -g \$HOST_GID \$HOST_UNAME 2>/dev/null || true
                useradd -m -u \$HOST_UID -g \$HOST_GID -s /bin/bash \$HOST_UNAME 2>/dev/null || true
              fi
              chown -R \$HOST_UID:\$HOST_GID /home/\$HOST_UNAME || true
              
              # Ensure output directory exists and has correct permissions
              mkdir -p /work/reCamera-OS/output || true
              chown -R \$HOST_UID:\$HOST_GID /work/reCamera-OS/output || true
              
              # Run as user
              sudo -u \$HOST_UNAME bash -c '
                set -e
                git config --global --add safe.directory /work
                git config --global --add safe.directory /work/reCamera-OS
                cd /work/reCamera-OS
                make \$TARGET
              '
              "
            '');
          };

          # Cut a release (tag + push 3 repos, and PR/merge for protected OS repo)
          release = {
            type = "app";
            program = toString (pkgs.writeShellScript "release" ''
              set -euo pipefail

              PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

              # Flags:
              #   --version X.Y.Z  Override changelog-derived version
              #   --force          Delete existing *local* tags before re-tagging (remote tags are NOT rewritten)
              #   --dry-run        Print actions without mutating tags/branches or pushing
              FORCE_LOCAL_TAGS=0
              DRY_RUN=0
              OVERRIDE_VERSION=""
              while [ "$#" -gt 0 ]; do
                case "$1" in
                  --version)
                    if [ "$#" -lt 2 ]; then
                      echo "ERROR: --version requires an argument like 0.2.7"
                      exit 1
                    fi
                    shift
                    OVERRIDE_VERSION="$1"
                    ;;
                  --force)
                    FORCE_LOCAL_TAGS=1
                    ;;
                  --dry-run)
                    DRY_RUN=1
                    ;;
                  -h|--help)
                    echo "Usage: nix run .#release -- [--version X.Y.Z] [--force] [--dry-run]"
                    exit 0
                    ;;
                  *)
                    echo "ERROR: Unknown argument: $1"
                    echo "Usage: nix run .#release -- [--version X.Y.Z] [--force] [--dry-run]"
                    exit 1
                    ;;
                esac
                shift
              done

              cd "$PROJECT_ROOT"

              CHANGELOG="reCamera-OS/CHANGELOG.md"
              if [ ! -f "$CHANGELOG" ]; then
                echo "ERROR: $CHANGELOG not found. Run from the superproject root."
                exit 1
              fi

              if [ -n "$OVERRIDE_VERSION" ]; then
                VERSION="$OVERRIDE_VERSION"
              else
                VERSION=$(grep -m1 -E '^##[[:space:]]+[0-9]+\.[0-9]+\.[0-9]+' "$CHANGELOG" \
                  | sed -E 's/^##[[:space:]]+([0-9]+\.[0-9]+\.[0-9]+).*/\1/')
              fi

              if [ -z "$VERSION" ]; then
                echo "ERROR: Could not parse version from top of $CHANGELOG"
                echo "Expected a line like: ## 0.2.5 (YYYY-MM-DD)"
                exit 1
              fi

              echo "============================================="
              echo "Cutting release: $VERSION"
              echo "============================================="

              run() {
                if [ "$DRY_RUN" -eq 1 ]; then
                  echo "+ $*"
                else
                  "$@"
                fi
              }

              ensure_clean() {
                local dir="$1"
                if [ "$DRY_RUN" -eq 1 ]; then
                  return 0
                fi
                ( cd "$dir" && [ -z "$(git status --porcelain)" ] ) || {
                  echo "ERROR: Working tree not clean: $dir"
                  ( cd "$dir" && git status --porcelain )
                  exit 1
                }
              }

              ensure_tag_ok_or_prepare() {
                # Accept an existing tag iff it points at HEAD (idempotent reruns).
                # If --force is set, delete the local tag (but do not force-push remote tags).
                local dir="$1"
                (
                  cd "$dir"

                  if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null; then
                    if [ "$FORCE_LOCAL_TAGS" -eq 1 ]; then
                      echo "NOTE: Deleting existing local tag in $dir: $VERSION (due to --force)"
                      run git tag -d "$VERSION" >/dev/null
                      return 0
                    fi

                    local tag_commit head_commit
                    tag_commit="$(git rev-list -n1 "$VERSION")"
                    head_commit="$(git rev-parse HEAD)"
                    if [ "$tag_commit" = "$head_commit" ]; then
                      echo "NOTE: Tag already exists in $dir at HEAD; reusing: $VERSION"
                      return 0
                    fi

                    echo "ERROR: Tag already exists in $dir, but does not point at HEAD: $VERSION"
                    echo "       tag -> $tag_commit"
                    echo "       HEAD -> $head_commit"
                    echo "Fix: bump the version in reCamera-OS/CHANGELOG.md, or re-run with --force (local-only), or delete/retag manually."
                    exit 1
                  fi
                )
              }

              ensure_clean "$PROJECT_ROOT"

              ensure_tag_ok_or_prepare "$PROJECT_ROOT"

              # Tag the repo (annotated tag)
              ( cd "$PROJECT_ROOT" && git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null || run git tag -a "$VERSION" -m "$VERSION" )

              # Push tag and branch
              ( cd "$PROJECT_ROOT" && run git push origin "$VERSION" )
              ( cd "$PROJECT_ROOT" && run git push origin HEAD ) || true

              echo "Done. Release version: $VERSION"
            '');
          };

          # Clean build output
          clean = {
            type = "app";
            program = toString (pkgs.writeShellScript "clean-build" ''
              set -e
              if [ -d "reCamera-OS/output" ]; then
                echo "Cleaning reCamera-OS/output..."
                rm -rf reCamera-OS/output
                echo "Done!"
              else
                echo "Output directory already clean"
              fi
            '');
          };

          # Clean only br2-external package build artifacts
          clean-external = {
            type = "app";
            program = toString (pkgs.writeShellScript "clean-external" ''
              set -e

              BR2_EXT_DIR="reCamera-OS/external/br2-external"
              BUILD_GLOB="reCamera-OS/output/*/buildroot-*/output/*/build"

              # Discover package names from br2-external subdirectories
              PACKAGES=()
              for pkg_dir in "$BR2_EXT_DIR"/*/; do
                pkg_name=$(basename "$pkg_dir")
                if ls "$pkg_dir"/*.mk >/dev/null 2>&1; then
                  PACKAGES+=("$pkg_name")
                fi
              done

              if [ ''${#PACKAGES[@]} -eq 0 ]; then
                echo "No packages found in $BR2_EXT_DIR"
                exit 1
              fi

              BUILD_DIRS=()
              for d in $BUILD_GLOB; do
                [ -d "$d" ] && BUILD_DIRS+=("$d")
              done

              if [ ''${#BUILD_DIRS[@]} -eq 0 ]; then
                echo "No Buildroot build directories found."
                echo "Nothing to clean."
                exit 0
              fi

              echo "============================================="
              echo "Cleaning br2-external package build artifacts"
              echo "============================================="
              echo ""

              CLEANED=0
              for build_dir in "''${BUILD_DIRS[@]}"; do
                target=$(echo "$build_dir" | ${pkgs.gnused}/bin/sed 's|reCamera-OS/output/\([^/]*\)/.*|\1|')
                echo "Target: $target"
                echo "  Build dir: $build_dir"
                echo ""

                for pkg in "''${PACKAGES[@]}"; do
                  for match in "$build_dir"/''${pkg}-[0-9v]*/; do
                    if [ -d "$match" ]; then
                      dir_name=$(basename "$match")
                      echo "  Removing: $dir_name"
                      rm -rf "$match"
                      CLEANED=$((CLEANED + 1))
                    fi
                  done
                done
              done

              echo ""
              if [ "$CLEANED" -gt 0 ]; then
                echo "Cleaned $CLEANED package build director$([ "$CLEANED" -eq 1 ] && echo 'y' || echo 'ies')."
                echo ""
                echo "Next build will rebuild these packages from source."
              else
                echo "No br2-external package build directories found. Already clean."
              fi
            '');
          };

          # Stage a release for OTA testing
          ota-stage = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-stage" ''
              set -e
              
              # Find the latest OTA zip first
              OTA_ZIP=$(find reCamera-OS/output -type f -name '*_emmc_ota.zip' -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -1 | cut -d' ' -f2-)
              
              if [ -z "$OTA_ZIP" ]; then
                echo "Error: No OTA zip found. Run 'nix run .#build' first."
                exit 1
              fi
              
              echo "Found: $OTA_ZIP"
              
              # Determine version
              if [ -n "$1" ]; then
                VERSION="$1"
              else
                # Try to extract version from filename (e.g. sg2002_reCamera_0.2.2_emmc_ota.zip -> 0.2.2)
                FILENAME=$(basename "$OTA_ZIP")
                # Extract 0.2.2 from sg2002_reCamera_0.2.2_emmc_ota.zip
                # Pattern: *_reCamera_<VERSION>_emmc_ota.zip
                VERSION=$(echo "$FILENAME" | sed -n 's/.*_reCamera_\(.*\)_emmc_ota.zip/\1/p')
                
                if [ -z "$VERSION" ]; then
                  echo "Could not auto-detect version from filename. Defaulting to 'latest'."
                  VERSION="latest"
                else
                  echo "Auto-detected version: $VERSION"
                fi
              fi
              
              echo "============================================="
              echo "Staging OTA release: $VERSION"
              echo "============================================="
              
              # Create release directory
              DEST_DIR="ota_server/ota_content/releases/$VERSION"
              mkdir -p "$DEST_DIR"
              
              # Copy and generate manifest
              cp -f "$OTA_ZIP" "$DEST_DIR/"
              cd "$DEST_DIR"
              sha256sum *.zip > sg2002_recamera_emmc_sha256sum.txt
              
              echo ""
              echo "Staged to: $DEST_DIR"
              echo ""
              echo "Start server with: nix run .#ota-serve"
              echo "Test URL: http://localhost:8080/releases/$VERSION/sg2002_recamera_emmc_sha256sum.txt"
            '');
          };

          # Stage a release for OTA testing (MD5 compatibility)
          ota-stage-md5 = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-stage-md5" ''
              set -e
              
              OTA_ZIP=$(find reCamera-OS/output -type f -name '*_emmc_ota.zip' -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -1 | cut -d' ' -f2-)
              if [ -z "$OTA_ZIP" ]; then
                echo "Error: No OTA zip found. Run 'nix run .#build' first."
                exit 1
              fi
              
              echo "Found: $OTA_ZIP"
              
              if [ -n "$1" ]; then
                VERSION="$1"
              else
                FILENAME=$(basename "$OTA_ZIP")
                VERSION=$(echo "$FILENAME" | sed -n 's/.*_reCamera_\(.*\)_emmc_ota.zip/\1/p')
                if [ -z "$VERSION" ]; then
                  echo "Could not auto-detect version from filename. Defaulting to 'latest'."
                  VERSION="latest"
                else
                  echo "Auto-detected version: $VERSION"
                fi
              fi
              
              echo "============================================="
              echo "Staging OTA release (MD5): $VERSION"
              echo "============================================="
              
              bash ota_server/prepare_release.sh "$VERSION" "$OTA_ZIP" --md5
              
              echo ""
              echo "Start server with: nix run .#ota-serve"
              echo "Test URL: http://localhost:8080/releases/$VERSION/sg2002_recamera_emmc_md5sum.txt"
            '');
          };

          # Stage a release for OTA testing (both manifests)
          ota-stage-both = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-stage-both" ''
              set -e
              
              OTA_ZIP=$(find reCamera-OS/output -type f -name '*_emmc_ota.zip' -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -1 | cut -d' ' -f2-)
              if [ -z "$OTA_ZIP" ]; then
                echo "Error: No OTA zip found. Run 'nix run .#build' first."
                exit 1
              fi
              
              echo "Found: $OTA_ZIP"
              
              if [ -n "$1" ]; then
                VERSION="$1"
              else
                FILENAME=$(basename "$OTA_ZIP")
                VERSION=$(echo "$FILENAME" | sed -n 's/.*_reCamera_\(.*\)_emmc_ota.zip/\1/p')
                if [ -z "$VERSION" ]; then
                  echo "Could not auto-detect version from filename. Defaulting to 'latest'."
                  VERSION="latest"
                else
                  echo "Auto-detected version: $VERSION"
                fi
              fi
              
              echo "============================================="
              echo "Staging OTA release (SHA256+MD5): $VERSION"
              echo "============================================="
              
              bash ota_server/prepare_release.sh "$VERSION" "$OTA_ZIP" --both
              
              echo ""
              echo "Start server with: nix run .#ota-serve"
              echo "SHA256 URL: http://localhost:8080/releases/$VERSION/sg2002_recamera_emmc_sha256sum.txt"
              echo "MD5 URL:    http://localhost:8080/releases/$VERSION/sg2002_recamera_emmc_md5sum.txt"
            '');
          };

          # Start OTA server
          ota-serve = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-serve" ''
              set -e
              cd ota_server
              
              if [ ! -d "ota_content/releases" ]; then
                echo "Warning: No releases staged. Run 'nix run .#ota-stage' first."
                echo ""
              fi
              
              echo "Starting OTA server on http://localhost:8080..."
              ${pkgs.docker-compose}/bin/docker-compose up -d
              
              # Get local IP for device configuration
              LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
              
              echo ""
              echo "============================================="
              echo "OTA Server running!"
              echo "============================================="
              echo ""
              echo "Available releases:"
              for dir in ota_content/releases/*/; do
                VERSION=$(basename "$dir")
                echo "  - $VERSION"
                echo "    URL: http://$LOCAL_IP:8080/releases/$VERSION/sg2002_recamera_emmc_sha256sum.txt"
              done
              echo ""
              echo "On the reCamera, run:"
              echo "  echo '1,http://$LOCAL_IP:8080/releases/latest/sg2002_recamera_emmc_sha256sum.txt' | sudo tee /etc/upgrade"
              echo "  sudo /mnt/system/upgrade.sh latest"
              echo "  sudo /mnt/system/upgrade.sh download"
              echo "  sudo /mnt/system/upgrade.sh start"
              echo ""
              echo "Stop server: nix run .#ota-stop"
            '');
          };

          # Show OTA status and URLs
          ota-status = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-status" ''
              LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
              
              echo "============================================="
              echo "OTA Server Status"
              echo "============================================="
              
              # Check if running
              if ${pkgs.docker}/bin/docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^ota-server$'; then
                echo "Server: RUNNING on http://$LOCAL_IP:8080"
              else
                echo "Server: STOPPED (run: nix run .#ota-serve)"
              fi
              echo ""
              
              # List releases
              echo "Staged releases:"
              if [ -d "ota_server/ota_content/releases" ]; then
                for dir in ota_server/ota_content/releases/*/; do
                  if [ -d "$dir" ]; then
                    VERSION=$(basename "$dir")
                    echo "  - $VERSION"
                  fi
                done
              else
                echo "  (none - run: nix run .#ota-stage)"
              fi
              echo ""
              
              # Show device commands
              echo "To update reCamera, SSH in and run:"
              echo "  echo '1,http://$LOCAL_IP:8080/releases/latest/sg2002_recamera_emmc_sha256sum.txt' | sudo tee /etc/upgrade"
              echo "  sudo /mnt/system/upgrade.sh latest"
              echo "  sudo /mnt/system/upgrade.sh download"
              echo "  sudo /mnt/system/upgrade.sh start"
            '');
          };

          # Stop OTA server
          ota-stop = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-stop" ''
              cd ota_server
              ${pkgs.docker-compose}/bin/docker-compose down
              echo "OTA server stopped."
            '');
          };

          # Clean OTA releases
          ota-clean = {
            type = "app";
            program = toString (pkgs.writeShellScript "ota-clean" ''
              set -e
              
              RELEASES_DIR="ota_server/ota_content/releases"
              
              if [ ! -d "$RELEASES_DIR" ]; then
                echo "No releases directory found. Already clean."
                exit 0
              fi
              
              # Check if directory is empty
              if [ -z "$(ls -A $RELEASES_DIR)" ]; then
                echo "Releases directory is already empty."
                exit 0
              fi
              
              echo "============================================="
              echo "Cleaning OTA Releases"
              echo "============================================="
              echo ""
              echo "The following releases will be removed:"
              for dir in $RELEASES_DIR/*/; do
                if [ -d "$dir" ]; then
                  VERSION=$(basename "$dir")
                  echo "  - $VERSION"
                fi
              done
              echo ""
              
              # Confirm deletion
              read -p "Are you sure you want to delete all releases? [y/N] " -n 1 -r
              echo ""
              if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                echo "Cancelled."
                exit 0
              fi
              
              # Remove all releases
              rm -rf $RELEASES_DIR/*
              
              echo ""
              echo "All releases cleaned!"
              echo "Run 'nix run .#ota-stage' to stage a new release."
            '');
          };

          # Flash recovery image to SD card
          flash-recovery = {
            type = "app";
            program = toString (pkgs.writeShellScript "flash-recovery" ''
              set -e

              # Parse optional version argument
              VERSION="''${1:-}"

              # Find recovery zip(s)
              INSTALL_DIR="reCamera-OS/output/sg2002_recamera_emmc/install/soc_sg2002_recamera_emmc"

              if [ -n "$VERSION" ]; then
                # Find specific version
                RECOVERY_ZIP="$INSTALL_DIR/sg2002_AuthorityOS_''${VERSION}_emmc_recovery.zip"
                if [ ! -f "$RECOVERY_ZIP" ]; then
                  echo "Error: Recovery image for version $VERSION not found"
                  echo "Looking for: $RECOVERY_ZIP"
                  echo ""
                  echo "Available recovery images:"
                  ls -1 "$INSTALL_DIR"/*_emmc_recovery.zip 2>/dev/null || echo "  (none found)"
                  exit 1
                fi
              else
                # Find latest recovery zip
                RECOVERY_ZIP=$(find "$INSTALL_DIR" -name '*_emmc_recovery.zip' -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -1 | cut -d' ' -f2-)
                if [ -z "$RECOVERY_ZIP" ]; then
                  echo "Error: No recovery image found. Run 'nix run .#build' first."
                  exit 1
                fi
                # Extract version from filename
                VERSION=$(basename "$RECOVERY_ZIP" | sed -n 's/.*AuthorityOS_\(.*\)_emmc_recovery.zip/\1/p')
              fi

              echo "============================================="
              echo "Flash Recovery Image: $VERSION"
              echo "============================================="
              echo "Source: $RECOVERY_ZIP"
              echo ""

              # Extract to temp directory
              TEMP_DIR=$(mktemp -d)
              trap "rm -rf $TEMP_DIR" EXIT

              echo "Extracting recovery image..."
              unzip -q "$RECOVERY_ZIP" -d "$TEMP_DIR"

              IMG_FILE=$(find "$TEMP_DIR" -name '*.img' | head -1)
              if [ -z "$IMG_FILE" ]; then
                echo "Error: No .img file found in recovery zip"
                exit 1
              fi

              IMG_SIZE=$(stat -c%s "$IMG_FILE")
              echo "Image: $(basename "$IMG_FILE") ($(numfmt --to=iec $IMG_SIZE))"
              echo ""

              # List available block devices (filter for likely SD cards/USB drives)
              echo "Available devices:"
              echo ""

              # Build device list with descriptions
              DEVICES=()
              i=1
              while IFS= read -r line; do
                DEV_NAME=$(echo "$line" | awk '{print $1}')
                DEV_SIZE=$(echo "$line" | awk '{print $2}')
                DEV_RM=$(echo "$line" | awk '{print $3}')
                DEV_MODEL=$(echo "$line" | awk '{print $4}')
                DEV_TRAN=$(echo "$line" | awk '{print $5}')

                # Build description
                DESC="$DEV_SIZE"
                [ -n "$DEV_MODEL" ] && DESC="$DESC - $DEV_MODEL"
                [ -n "$DEV_TRAN" ] && DESC="$DESC ($DEV_TRAN)"
                [ "$DEV_RM" = "1" ] && DESC="$DESC [removable]"

                DEVICES+=("$DEV_NAME")
                printf "  %d) /dev/%-10s %s\n" "$i" "$DEV_NAME" "$DESC"
                ((i++))
              done < <(lsblk -d -n -o NAME,SIZE,RM,MODEL,TRAN 2>/dev/null | grep -E '^(sd|mmcblk)')

              if [ ''${#DEVICES[@]} -eq 0 ]; then
                echo "  No suitable devices found (SD card or USB drive)"
                echo ""
                echo "Insert an SD card and try again."
                exit 1
              fi

              echo ""
              echo "  0) Cancel"
              echo ""

              # Prompt for device selection
              read -p "Select device number: " SELECTION

              if [ -z "$SELECTION" ] || [ "$SELECTION" = "0" ]; then
                echo "Cancelled."
                exit 0
              fi

              # Validate selection is a number
              if ! [[ "$SELECTION" =~ ^[0-9]+$ ]]; then
                echo "Invalid selection."
                exit 1
              fi

              # Get device from selection (1-indexed)
              INDEX=$((SELECTION - 1))
              if [ "$INDEX" -lt 0 ] || [ "$INDEX" -ge ''${#DEVICES[@]} ]; then
                echo "Invalid selection: $SELECTION"
                exit 1
              fi

              DEVICE="''${DEVICES[$INDEX]}"

              # Validate device exists
              if [ ! -b "/dev/$DEVICE" ]; then
                echo "Error: /dev/$DEVICE is not a valid block device"
                exit 1
              fi

              # Safety check - don't flash nvme or system disk
              if echo "$DEVICE" | grep -qE '^(nvme|loop)'; then
                echo "Error: Refusing to flash $DEVICE (system disk protection)"
                exit 1
              fi

              # Final confirmation
              echo ""
              echo "WARNING: This will ERASE ALL DATA on /dev/$DEVICE"
              echo ""
              lsblk "/dev/$DEVICE"
              echo ""
              read -p "Type 'yes' to confirm: " CONFIRM

              if [ "$CONFIRM" != "yes" ]; then
                echo "Aborted."
                exit 1
              fi

              # Unmount any mounted partitions
              for part in /dev/''${DEVICE}*; do
                if mountpoint -q "$part" 2>/dev/null || mount | grep -q "$part"; then
                  echo "Unmounting $part..."
                  sudo umount "$part" 2>/dev/null || true
                fi
              done

              # Flash with dd
              echo ""
              echo "Flashing to /dev/$DEVICE..."
              echo ""

              sudo dd if="$IMG_FILE" of="/dev/$DEVICE" bs=4M status=progress conv=fsync

              echo ""
              echo "============================================="
              echo "Flash complete!"
              echo "============================================="
              echo ""
              echo "Safely eject the SD card before removing."
            '');
          };
        };

        # Default package points to Docker build
        packages.default = pkgs.writeShellScriptBin "recamera-build" ''
          cd reCamera-OS
          exec ./docker_build.sh sg2002_recamera_emmc
        '';
      });
}
