################################################################################
#
# zerotier-one
#
################################################################################

ZEROTIER_ONE_VERSION = 1.16.0
ZEROTIER_ONE_SITE = $(call github,zerotier,ZeroTierOne,$(ZEROTIER_ONE_VERSION))
ZEROTIER_ONE_LICENSE = BUSL-1.1
ZEROTIER_ONE_LICENSE_FILES = LICENSE.txt
ZEROTIER_ONE_CPE_ID_VENDOR = zerotier
ZEROTIER_ONE_CPE_ID_PRODUCT = zerotierone
ZEROTIER_ONE_DEPENDENCIES = libopenssl

# Build environment
ZEROTIER_ONE_MAKE_ENV = \
	$(TARGET_MAKE_ENV) \
	CC="$(TARGET_CC)" \
	CXX="$(TARGET_CXX)" \
	AR="$(TARGET_AR)" \
	STRIP="$(TARGET_STRIP)"

# Build flags
ZEROTIER_ONE_MAKE_OPTS = \
	ZT_SSO_SUPPORTED=0 \
	ZT_CONTROLLER=0

# For musl libc compatibility
ifeq ($(BR2_TOOLCHAIN_USES_MUSL),y)
ZEROTIER_ONE_MAKE_OPTS += ZT_MUSL=1
endif

define ZEROTIER_ONE_BUILD_CMDS
	$(ZEROTIER_ONE_MAKE_ENV) $(MAKE) -C $(@D) \
		$(ZEROTIER_ONE_MAKE_OPTS) \
		one
endef

define ZEROTIER_ONE_INSTALL_TARGET_CMDS
	$(INSTALL) -D -m 755 $(@D)/zerotier-one $(TARGET_DIR)/usr/sbin/zerotier-one
	ln -sf zerotier-one $(TARGET_DIR)/usr/sbin/zerotier-cli
	ln -sf zerotier-one $(TARGET_DIR)/usr/sbin/zerotier-idtool
endef

define ZEROTIER_ONE_INSTALL_INIT_SYSV
	$(INSTALL) -D -m 755 $(ZEROTIER_ONE_PKGDIR)/S90zerotier \
		$(TARGET_DIR)/etc/init.d/S90zerotier
endef

define ZEROTIER_ONE_INSTALL_CONFIG
	$(INSTALL) -d -m 755 $(TARGET_DIR)/var/lib/zerotier-one
endef

ZEROTIER_ONE_POST_INSTALL_TARGET_HOOKS += ZEROTIER_ONE_INSTALL_CONFIG

$(eval $(generic-package))
