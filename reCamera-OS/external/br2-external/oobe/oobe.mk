################################################################################
#
# oobe
#
################################################################################

OOBE_VERSION = 1.0
OOBE_SITE = $(TOPDIR)/../../../../sscma-example-sg200x
OOBE_SITE_METHOD = local
OOBE_LICENSE = Apache-2.0

# Note: We use the host system's CMake and toolchain, not Buildroot's host packages.
# The oobe solution uses CMake with a custom RISC-V toolchain.
# Ensure cmake and the RISC-V toolchain are available in your build environment PATH.

# Configure step: run CMake configuration
define OOBE_CONFIGURE_CMDS
	mkdir -p $(@D)/solutions/oobe/build && \
	cd $(@D)/solutions/oobe/build && \
		cmake -S $(@D)/solutions/oobe \
		      -B $(@D)/solutions/oobe/build \
		      -DCMAKE_BUILD_TYPE=Release
endef

# Build step: compile the C++ binary for RISC-V
define OOBE_BUILD_CMDS
	cd $(@D)/solutions/oobe/build && \
		cmake --build . -j
endef

# Install step: copy the built files to the target directory
define OOBE_INSTALL_TARGET_CMDS
	# Install the executable file
	$(INSTALL) -D -m 0755 $(@D)/solutions/oobe/build/oobe $(TARGET_DIR)/usr/local/bin/oobe

	# Install web assets
	mkdir -p $(TARGET_DIR)/usr/share/oobe/www
	cp -r $(@D)/solutions/oobe/web/* $(TARGET_DIR)/usr/share/oobe/www/

	# Copy rootfs files (init scripts, config files, etc.)
	if [ -d "$(@D)/solutions/oobe/rootfs" ]; then \
		cp -r $(@D)/solutions/oobe/rootfs/* $(TARGET_DIR)/; \
	fi
endef

$(eval $(generic-package))
