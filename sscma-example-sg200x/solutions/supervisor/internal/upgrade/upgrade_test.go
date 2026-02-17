package upgrade

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeManager_parseVersionInfo_Valid(t *testing.T) {
	t.Parallel()

	m := &UpgradeManager{}
	checksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 64
	content := checksum + "  sg2002_reCamera_1.0.0_ota.zip\n"

	p := filepath.Join(t.TempDir(), "sha256sum.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	info, err := m.parseVersionInfo(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Checksum != checksum {
		t.Fatalf("checksum mismatch: want=%q got=%q", checksum, info.Checksum)
	}
	if info.FileName != "sg2002_reCamera_1.0.0_ota.zip" {
		t.Fatalf("filename mismatch: %q", info.FileName)
	}
	if info.OSName != "reCamera" {
		t.Fatalf("os name mismatch: %q", info.OSName)
	}
	if info.Version != "1.0.0" {
		t.Fatalf("version mismatch: %q", info.Version)
	}
}

func TestUpgradeManager_parseVersionInfo_Invalid(t *testing.T) {
	t.Parallel()

	m := &UpgradeManager{}
	p := filepath.Join(t.TempDir(), "sha256sum.txt")
	if err := os.WriteFile(p, []byte("not a checksum file\n"), 0o644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}
	if _, err := m.parseVersionInfo(p); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpgradeManager_readZipChecksum(t *testing.T) {
	t.Parallel()

	m := &UpgradeManager{}
	zipPath := filepath.Join(t.TempDir(), "ota.zip")

	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("sha256sum.txt")
	if err != nil {
		t.Fatalf("create sha256sum.txt: %v", err)
	}
	// The parser just stores the first two fields; it doesn't validate checksum length.
	if _, err := w.Write([]byte("deadbeef  boot.emmc\n0123  rootfs_ext4.emmc\n")); err != nil {
		t.Fatalf("write sha256sum.txt: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := zf.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	mmap, err := m.readZipChecksum(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mmap["boot.emmc"] != "deadbeef" {
		t.Fatalf("unexpected boot.emmc checksum: %q", mmap["boot.emmc"])
	}
	if mmap["rootfs_ext4.emmc"] != "0123" {
		t.Fatalf("unexpected rootfs checksum: %q", mmap["rootfs_ext4.emmc"])
	}
}

func TestUpgradeManager_verifyChecksum(t *testing.T) {
	t.Parallel()

	m := &UpgradeManager{}
	p := filepath.Join(t.TempDir(), "file.bin")
	data := []byte("hello")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	h := sha256.Sum256(data)
	expected := hex.EncodeToString(h[:])
	if err := m.verifyChecksum(p, expected); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if err := m.verifyChecksum(p, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}
