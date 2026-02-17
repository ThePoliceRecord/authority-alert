################################################################################
#
# sscma-camera-recorder
#
################################################################################

SSCMA_VIDEO_RECORDER_VERSION = 1.0
SSCMA_VIDEO_RECORDER_SITE = $(TOPDIR)/../../sscma-example-sg200x
SSCMA_VIDEO_RECORDER_SITE_METHOD = local
SSCMA_VIDEO_RECORDER_LICENSE = Apache-2.0

# Build step: compile the package using the Makefile
define SSCMA_VIDEO_RECORDER_BUILD_CMDS
	$(MAKE) -C $(@D)/solutions/camera-recorder \
		CC="$(TARGET_CC)" \
		CXX="$(TARGET_CXX)" \
		CFLAGS="$(TARGET_CFLAGS) -I$(@D)/components/sophgo/video/include -I$(STAGING_DIR)/usr/include" \
		CXXFLAGS="$(TARGET_CXXFLAGS) -I$(@D)/components/sophgo/video/include -I$(STAGING_DIR)/usr/include" \
		LDFLAGS="$(TARGET_LDFLAGS) -L$(STAGING_DIR)/usr/lib" \
		VIDEO_DIR="$(@D)/components/sophgo/video" \
		BUILD_DIR="build"
endef

# Install step: copy the built binary and rootfs files to the target directory
define SSCMA_VIDEO_RECORDER_INSTALL_TARGET_CMDS
	# Install the executable file
	$(INSTALL) -D -m 0755 $(@D)/solutions/camera-recorder/build/camera-recorder \
		$(TARGET_DIR)/usr/local/bin/camera-recorder
	
	# Copy rootfs files if they exist
	if [ -d $(@D)/solutions/camera-recorder/rootfs ]; then \
		cp -r $(@D)/solutions/camera-recorder/rootfs/* $(TARGET_DIR)/; \
	fi
endef

$(eval $(generic-package))
