{
  description = "reCamera-OS - Sophgo SG200X Camera Operating System";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # Available build targets
        supportedTargets = [
          "sg2002_recamera_emmc"
          "sg2002_recamera_sd"
          "sg2002_xiao_sd"
        ];

        # Default target
        defaultTarget = "sg2002_recamera_emmc";

        # Function to build SDK for a specific target
        mkSDKBuilder = buildTarget: pkgs.stdenv.mkDerivation {
          name = "recamera-sdk-${buildTarget}";
          src = ./.;

          nativeBuildInputs = with pkgs; [
            docker
            git
            python3
            python3Packages.jinja2
            python3Packages.pyyaml
          ];

          # Docker build requires host daemon access
          __impure = true;

          buildPhase = ''
            echo "════════════════════════════════════════════════════════════"
            echo " Building reCamera-OS SDK"
            echo "════════════════════════════════════════════════════════════"
            echo " Target: ${buildTarget}"
            echo " Architecture: RISC-V (musl)"
            echo " This build may take 1-2 hours..."
            echo "════════════════════════════════════════════════════════════"
            echo ""

            # Initialize git (required by build system)
            git config --global user.email "nix@builder"
            git config --global user.name "Nix Builder"
            git config --global  init.defaultBranch main
            git config --global safe.directory $(pwd)

            # Initialize repo if not already a git repo
            if [ ! -d ".git" ]; then
              git init
              git add .
              git commit -m "Initial commit for Nix build"
            fi

            # Update submodules (with depth limit for faster builds)
            echo "Updating submodules..."
            git submodule update --init --recursive --depth 1 || true

            # Run the Docker build script
            echo "Starting Docker build..."
            bash ./docker_build.sh ${buildTarget}
            
            echo ""
            echo "✓ Build completed successfully"
          '';

          installPhase = ''
            echo "Installing SDK to Nix store..."
            mkdir -p $out/${buildTarget}
            
            # Path to SDK tarball
            SDK_TARBALL="output/${buildTarget}/install/soc_${buildTarget}/${buildTarget}_sdk.tar.gz"
            
            if [ -f "$SDK_TARBALL" ]; then
              echo "✓ Found SDK tarball"
              tar -xzf "$SDK_TARBALL" -C $out/${buildTarget} --strip-components=1
              echo "✓ SDK extracted to $out/${buildTarget}"
              
              # Also copy the full output for reference
              mkdir -p $out/${buildTarget}/build_output
              cp -r output/${buildTarget}/* $out/${buildTarget}/build_output/ || true
              
              echo "✓ Complete build output preserved"
            else
              echo "✗ ERROR: SDK tarball not found"
              echo "Expected at: $SDK_TARBALL"
              echo ""
              echo "Available files:"
              find output -type f -name "*.tar.gz" || true
              exit 1
            fi

            # Create metadata file
            cat > $out/${buildTarget}/.nix-build-info <<EOF
Build Target: ${buildTarget}
Build Date: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
Nix System: ${system}
Builder: Nix Flake
Source: reCamera-OS (local)
EOF
            
            echo ""
            echo "════════════════════════════════════════════════════════════"
            echo " SDK Installation Complete"
            echo "════════════════════════════════════════════════════════════"
            echo " Location: $out/${buildTarget}"
            echo " TPU SDK: $out/${buildTarget}/tpu_musl_riscv64/cvitek_tpu_sdk"
            echo "════════════════════════════════════════════════════════════"
          '';

          meta = with pkgs.lib; {
            description = "reCamera-OS SDK for Sophgo SG200X (${buildTarget})";
            homepage = "https://github.com/Seeed-Studio/reCamera-OS";
            license = licenses.asl20;
            platforms = platforms.linux;
            maintainers = with maintainers; [ ];
          };
        };

        # Build all supported targets as packages
        sdkPackages = builtins.listToAttrs (
          map (target: {
            name = "sdk-${target}";
            value = mkSDKBuilder target;
          }) supportedTargets
        );

      in
      {
        # Export SDK packages
        packages = sdkPackages // {
          # Default to emmc target
          default = sdkPackages."sdk-${defaultTarget}";
          
          # Convenience alias
          sdk = sdkPackages."sdk-${defaultTarget}";
        };

        # Development shell for building reCamera-OS
        devShells.default = pkgs.mkShell {
          name = "recamera-os-dev";

          buildInputs = with pkgs; [
            # Build tools
            docker
            git
            cmake
            gnumake
           
            # Python tools
            python3
            python3Packages.jinja2
            python3Packages.pyyaml
            
            # Development tools
            vim
            tree
          ];

          shellHook = ''
            echo "════════════════════════════════════════════════════════════"
            echo " reCamera-OS Development Environment"
            echo "════════════════════════════════════════════════════════════"
            echo ""
            echo "Available build targets:"
            ${builtins.concatStringsSep "\n" (map (t: "  echo \"  - ${t}\"") supportedTargets)}
            echo ""
            echo "Build SDK with Nix:"
            echo "  nix build .#sdk-sg2002_recamera_emmc"
            echo ""
            echo "Or from main project:"
            echo "  cd .. && nix run .#build"
            echo ""
            echo "Output will be in:"
            echo "  ./output/<target>/"
            echo "════════════════════════════════════════════════════════════"
          '';
        };
      });
}
