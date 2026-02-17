################################################################################
#
# sscma-camera-streamer
#
################################################################################

SSCMA_CAMERA_STREAMER_VERSION = 1.0
SSCMA_CAMERA_STREAMER_SITE = $(TOPDIR)/../../sscma-example-sg200x
SSCMA_CAMERA_STREAMER_SITE_METHOD = local
SSCMA_CAMERA_STREAMER_LICENSE = Apache-2.0
SSCMA_CAMERA_STREAMER_DEPENDENCIES = openssl

# Configure step: prepare the build environment and run CMake to configure the build
define SSCMA_CAMERA_STREAMER_CONFIGURE_CMDS
	mkdir -p $(@D)/solutions/camera-streamer/build && \
	cd $(@D)/solutions/camera-streamer/build && \
	$(BR2_CMAKE) \
		-DSG200X_SDK_PATH=$(O)/../../../ \
		-DSYSROOT=$(STAGING_DIR) \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX=$(TARGET_DIR) \
		..
endef

# Build step: compile the package using the Makefile in the build directory
define SSCMA_CAMERA_STREAMER_BUILD_CMDS
	$(MAKE) -C $(@D)/solutions/camera-streamer/build
endef

# Install step: copy the built binary and rootfs files to the target directory
define SSCMA_CAMERA_STREAMER_INSTALL_TARGET_CMDS
	# Install the executable file
	$(INSTALL) -D -m 0755 $(@D)/solutions/camera-streamer/build/camera-streamer \
		$(TARGET_DIR)/usr/local/bin/camera-streamer
	
	# Copy rootfs files if they exist
	if [ -d $(@D)/solutions/camera-streamer/rootfs ]; then \
		cp -r $(@D)/solutions/camera-streamer/rootfs/* $(TARGET_DIR)/; \
	fi
endef

$(eval $(generic-package))
