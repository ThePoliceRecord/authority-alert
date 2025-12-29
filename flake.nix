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
            echo "OTA server workflow:"
            echo "  nix run .#ota-stage      # Stage latest build"
            echo "  nix run .#ota-serve      # Start server"
            echo "  nix run .#ota-status     # Check status"
            echo "  nix run .#ota-stop       # Stop server"
            echo "  nix run .#ota-clean      # Remove all releases"
            echo ""
            echo "Submodule Nix flakes (use independently):"
            echo "  cd reCamera-OS && nix develop          # SDK build environment"
            echo "  cd sscma-example-sg200x && nix develop # SSCMA development"
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
                git submodule update --init --recursive --depth 1
                make \$TARGET
              '
              "
            '');
          };

          # Initialize git submodules
          init = {
            type = "app";
            program = toString (pkgs.writeShellScript "init-submodules" ''
              set -e
              cd reCamera-OS
              echo "Initializing git submodules..."
              git submodule update --init --recursive --depth 1
              echo "Done!"
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

          # Check submodule status
          sub-status = {
            type = "app";
            program = toString (pkgs.writeShellScript "sub-status" ''
              echo "============================================="
              echo "Submodule Status"
              echo "============================================="
              echo ""
              
              # Check if reCamera-OS exists
              if [ ! -d "reCamera-OS" ]; then
                echo "Error: reCamera-OS directory not found"
                exit 1
              fi
              
              cd reCamera-OS
              
              # Show submodule summary
              git submodule summary
              echo ""
              
              # Show status of submodules
              echo "Detailed status:"
              git status --short
              echo ""
              
              # Check for uncommitted changes
              if git status --porcelain | grep -q .; then
                echo "Uncommitted changes detected in reCamera-OS submodule"
                echo "Run 'nix run .#sub-diff' to see details"
              else
                echo "reCamera-OS submodule is clean"
              fi
            '');
          };

          # Show submodule diff
          sub-diff = {
            type = "app";
            program = toString (pkgs.writeShellScript "sub-diff" ''
              cd reCamera-OS
              echo "============================================="
              echo "Submodule Changes (git diff)"
              echo "============================================="
              echo ""
              git diff
            '');
          };

          # Update submodules to latest commit
          sub-update = {
            type = "app";
            program = toString (pkgs.writeShellScript "sub-update" ''
              set -e
              cd reCamera-OS
              echo "============================================="
              echo "Updating submodules to latest commits..."
              echo "============================================="
              echo ""
              
              # Update all submodules recursively
              git submodule update --remote --recursive --depth 1
              
              echo ""
              echo "Submodules updated!"
              echo ""
              echo "New submodule commits:"
              git submodule status
            '');
          };

          # Reset submodules to clean state
          sub-reset = {
            type = "app";
            program = toString (pkgs.writeShellScript "sub-reset" ''
              set -e
              cd reCamera-OS
              echo "============================================="
              echo "Resetting submodules to clean state..."
              echo "============================================="
              echo ""
              
              # Reset all submodules
              git submodule foreach --recursive git reset --hard
              git submodule foreach --recursive git clean -fdx
              
              # Update to the committed state
              git submodule update --init --recursive --depth 1
              
              echo ""
              echo "Submodules reset to clean state!"
              git submodule status
            '');
          };

          # Commit submodule changes
          sub-commit = {
            type = "app";
            program = toString (pkgs.writeShellScript "sub-commit" ''
              set -e
              cd reCamera-OS
              
              # Check if there are changes
              if ! git status --porcelain | grep -q .; then
                echo "No changes to commit in reCamera-OS submodule"
                exit 0
              fi
              
              echo "============================================="
              echo "Committing submodule changes..."
              echo "============================================="
              echo ""
              
              # Show what will be committed
              echo "Changes to be committed:"
              git status --short
              echo ""
              
              # Prompt for commit message if not provided
              MESSAGE="''${1:-"Update reCamera-OS submodule"}"
              
              # Add all changes
              git add -A
              
              # Commit
              git commit -m "$MESSAGE"
              
              echo ""
              echo "Committed! New commit:"
              git log -1 --oneline
              echo ""
              echo "Remember to commit the parent repo to track this submodule update:"
              echo "  cd .."
              echo "  git add reCamera-OS"
              echo "  git commit -m 'Update reCamera-OS submodule'"
            '');
          };

          # Pull latest changes for submodules
          sub-pull = {
            type = "app";
            program = toString (pkgs.writeShellScript "sub-pull" ''
              set -e
              cd reCamera-OS
              echo "============================================="
              echo "Pulling latest changes for submodules..."
              echo "============================================="
              echo ""
              
              # Pull latest for each submodule
              git submodule foreach --recursive git pull origin $(git rev-parse --abbrev-ref HEAD)
              
              echo ""
              echo "Pull complete!"
              git submodule status
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
