# Makefile for oobe

VERSION ?= 0.1.0

BUILD_DIR := build
BIN_NAME := oobe

ROOTFS_DIR := rootfs
WWW_SRC_DIR := web
WWW_DST_DIR := usr/share/oobe/www

# Some environments (e.g. Jetson) export an LD_PRELOAD that may not exist inside
# reproducible build shells, which causes noisy "cannot be preloaded" warnings.
ENV_CLEAN := env -u LD_PRELOAD

.PHONY: all
all: build-riscv

.PHONY: configure
configure:
	@mkdir -p $(BUILD_DIR)
	@$(ENV_CLEAN) cmake -S . -B $(BUILD_DIR) -DCMAKE_BUILD_TYPE=Release

.PHONY: build-riscv
build-riscv: configure
	@$(ENV_CLEAN) cmake --build $(BUILD_DIR) -j
	@echo "Build complete: $(BUILD_DIR)/$(BIN_NAME)"

.PHONY: opkg
opkg: build-riscv
	@echo "Creating opkg package..."
	@rm -rf $(BUILD_DIR)/opkg-stage
	@mkdir -p $(BUILD_DIR)/opkg-stage/CONTROL
	@mkdir -p $(BUILD_DIR)/opkg-stage/usr/local/bin
	@mkdir -p $(BUILD_DIR)/opkg-stage/$(WWW_DST_DIR)
	@# Copy CONTROL files
	@cp opkg/CONTROL/control $(BUILD_DIR)/opkg-stage/CONTROL/
	@cp opkg/CONTROL/postinst $(BUILD_DIR)/opkg-stage/CONTROL/
	@chmod 755 $(BUILD_DIR)/opkg-stage/CONTROL/postinst
	@# Install the binary
	@cp $(BUILD_DIR)/$(BIN_NAME) $(BUILD_DIR)/opkg-stage/usr/local/bin/$(BIN_NAME)
	@chmod 755 $(BUILD_DIR)/opkg-stage/usr/local/bin/$(BIN_NAME)
	@# Copy rootfs files (init script, etc.)
	@if [ -d "$(ROOTFS_DIR)" ]; then cp -r $(ROOTFS_DIR)/* $(BUILD_DIR)/opkg-stage/; fi
	@# Copy web assets
	@cp -r $(WWW_SRC_DIR)/* $(BUILD_DIR)/opkg-stage/$(WWW_DST_DIR)/
	@# Create the opkg package
	@cd $(BUILD_DIR)/opkg-stage && \
		$(ENV_CLEAN) tar -czf ../data.tar.gz --owner=0 --group=0 ./usr ./etc && \
		$(ENV_CLEAN) tar -czf ../control.tar.gz --owner=0 --group=0 -C CONTROL . && \
		echo "2.0" > ../debian-binary && \
		cd .. && $(ENV_CLEAN) ar r $(BIN_NAME)_$(VERSION)_riscv64.ipk debian-binary control.tar.gz data.tar.gz
	@rm -f $(BUILD_DIR)/debian-binary $(BUILD_DIR)/control.tar.gz $(BUILD_DIR)/data.tar.gz
	@echo "opkg package created: $(BUILD_DIR)/$(BIN_NAME)_$(VERSION)_riscv64.ipk"

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
