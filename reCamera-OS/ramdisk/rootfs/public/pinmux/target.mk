ifeq ($(CONFIG_TARGET_PACKAGE_PINMUX),y)
TARGET_PACKAGES += pinmux

ifeq ($(CHIP_ARCH),CV181X)
PINMUX_BOARD := cv181x
else ifeq ($(CHIP_ARCH),CV180X)
PINMUX_BOARD := cv180x
else ifeq ($(CHIP_ARCH),CV183X)
PINMUX_BOARD := cv183x
else ifeq ($(CHIP_ARCH),CV182X)
PINMUX_BOARD := cv182x
else
PINMUX_BOARD := cv181x
endif

PINMUX_TOOLS_DIR := $(TOP_DIR)/ramdisk/tools/cvi_pinmux
PINMUX_SRC_DIR   := $(PINMUX_TOOLS_DIR)/$(PINMUX_BOARD)
PINMUX_BIN       := $(PINMUX_SRC_DIR)/cvi_pinmux
PINMUX_DST_DIR   = $(TOP_DIR)/ramdisk/rootfs/public/pinmux/$(packages_arch)/usr/sbin
PINMUX_DST       := $(PINMUX_DST_DIR)/cvi_pinmux

.PHONY: pinmux pinmux_clean pinmux_build_install

all: pinmux_build_install

pinmux:
	$(MAKE) -C $(PINMUX_SRC_DIR) SDK_VER=$(SDK_VER)
	mkdir -p $(PINMUX_DST_DIR)
	install -m 0755 $(PINMUX_BIN) $(PINMUX_DST)

rootfs-prepare: pinmux_build_install

pinmux_build_install:
	@echo "[pinmux] BOARD=$(PINMUX_BOARD) SDK_VER=$(SDK_VER) packages_arch=$(packages_arch)"
	$(MAKE) -C $(PINMUX_SRC_DIR) SDK_VER=$(SDK_VER)
	mkdir -p $(PINMUX_DST_DIR)
	install -m 0755 $(PINMUX_BIN) $(PINMUX_DST)

pinmux_clean:
	$(MAKE) -C $(PINMUX_SRC_DIR) clean || true
	rm -f $(PINMUX_DST)

endif
