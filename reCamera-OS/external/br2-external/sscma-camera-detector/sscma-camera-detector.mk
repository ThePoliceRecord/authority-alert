################################################################################
#
# sscma-camera-detector
#
################################################################################

SSCMA_CAMERA_DETECTOR_VERSION = 1.0
SSCMA_CAMERA_DETECTOR_SITE = $(TOPDIR)/../../../../sscma-example-sg200x
SSCMA_CAMERA_DETECTOR_SITE_METHOD = local
SSCMA_CAMERA_DETECTOR_LICENSE = Apache-2.0
SSCMA_CAMERA_DETECTOR_DEPENDENCIES = jpeg

# Configure step: prepare the build environment and run CMake to configure the build
define SSCMA_CAMERA_DETECTOR_CONFIGURE_CMDS
	mkdir -p $(@D)/solutions/camera-detector/build && \
	cd $(@D)/solutions/camera-detector/build && \
	$(BR2_CMAKE) \
		-DSG200X_SDK_PATH=$(O)/../../../install/soc_sg2002_recamera_emmc \
		-DSYSROOT=$(STAGING_DIR) \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX=$(TARGET_DIR) \
		..
endef

# Build step: compile the package using the Makefile in the build directory
define SSCMA_CAMERA_DETECTOR_BUILD_CMDS
	$(MAKE) -C $(@D)/solutions/camera-detector/build
endef

# Install step: copy the built binary and rootfs files to the target directory
define SSCMA_CAMERA_DETECTOR_INSTALL_TARGET_CMDS
	# Install the executable file
	$(INSTALL) -D -m 0755 $(@D)/solutions/camera-detector/build/camera-detector \
		$(TARGET_DIR)/usr/local/bin/camera-detector

	# Copy rootfs files if they exist
	if [ -d $(@D)/solutions/camera-detector/rootfs ]; then \
		cp -r $(@D)/solutions/camera-detector/rootfs/* $(TARGET_DIR)/; \
	fi
endef

$(eval $(generic-package))
