################################################################################
#
# sscma-supervisor
#
################################################################################

SSCMA_SUPERVISOR_VERSION = 1.0
SSCMA_SUPERVISOR_SITE = $(TOPDIR)/../../sscma-example-sg200x
SSCMA_SUPERVISOR_SITE_METHOD = local
SSCMA_SUPERVISOR_LICENSE = Apache-2.0

# Note: We use the host system's Go and npm/node, not Buildroot's host packages,
# because Buildroot 2021.05's host-go doesn't support RISC-V cross-compilation.
# Ensure go (>= 1.21) and npm are available in your build environment PATH.

# Configure step: download Go modules
define SSCMA_SUPERVISOR_CONFIGURE_CMDS
	cd $(@D)/solutions/supervisor && \
		go mod download
endef

# Build step: compile Go binary for RISC-V and build web frontend
define SSCMA_SUPERVISOR_BUILD_CMDS
	# Build web frontend
	cd $(@D)/solutions/supervisor/www && \
		npm install && \
		npm run build && \
		mkdir -p $(@D)/solutions/supervisor/rootfs/usr/share/supervisor/www && \
		cp -r $(@D)/solutions/supervisor/www/dist/* $(@D)/solutions/supervisor/rootfs/usr/share/supervisor/www/

	# Build Go binary for RISC-V
	cd $(@D)/solutions/supervisor && \
		mkdir -p build && \
		GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 \
		go build \
		-ldflags "-s -w" \
		-o $(@D)/solutions/supervisor/build/supervisor \
		./cmd/supervisor
endef

# Install step: copy the built files to the target directory
define SSCMA_SUPERVISOR_INSTALL_TARGET_CMDS
	# Install the executable file
	$(INSTALL) -D -m 0755 $(@D)/solutions/supervisor/build/supervisor $(TARGET_DIR)/usr/local/bin/supervisor

	# Copy other files from the source directory to the target directory
	cp -r $(@D)/solutions/supervisor/rootfs/* $(TARGET_DIR)/
endef

$(eval $(generic-package))
