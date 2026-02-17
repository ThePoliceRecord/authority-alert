package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// File paths for device information
const (
	IssueFile      = "/etc/issue"
	HostnameFile   = "/etc/hostname"
	DeviceNameFile = "/etc/recamera.conf/device_name"
	UptimeFile     = "/proc/uptime"
	CPUInfoFile    = "/proc/cpuinfo"
	MemInfoFile    = "/proc/meminfo"
)

// GetCPUModel returns the CPU model from /proc/cpuinfo or fw_printenv.
// For SG200x series chips, we try to detect the specific variant.
func GetCPUModel() string {
	// First, try to get from fw_printenv chip_type
	cmd := exec.Command("fw_printenv", "chip_type")
	if out, err := cmd.Output(); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(out)), "=", 2)
		if len(parts) == 2 {
			chipType := strings.TrimSpace(parts[1])
			if chipType != "" {
				return chipType
			}
		}
	}

	// Read /proc/cpuinfo
	data, err := os.ReadFile(CPUInfoFile)
	if err != nil {
		return "SG2002" // Default fallback
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Look for Hardware, Model name, or CPU implementer fields
		if strings.HasPrefix(line, "Hardware") || strings.HasPrefix(line, "Model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				model := strings.TrimSpace(parts[1])
				// If it contains sg200, return it as-is
				if strings.Contains(strings.ToLower(model), "sg200") {
					return model
				}
			}
		}
	}

	// Default to SG2002 if we can't determine
	return "SG2002"
}

// GetRAMSize returns the total RAM size in human-readable format (e.g., "256MB").
func GetRAMSize() string {
	data, err := os.ReadFile(MemInfoFile)
	if err != nil {
		return "256MB" // Default fallback
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			// Format: "MemTotal:        253952 kB"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					// Convert to MB and format nicely
					mb := kb / 1024
					if mb >= 1024 {
						gb := mb / 1024
						return strconv.FormatInt(gb, 10) + "GB"
					}
					return strconv.FormatInt(mb, 10) + "MB"
				}
			}
		}
	}

	return "256MB" // Default fallback
}

// GetNPUInfo returns the NPU (Neural Processing Unit) specs.
// For SG200x series, this is typically 1 TOPS.
func GetNPUInfo() string {
	// For SG2002/SG2000 chips, the NPU is 1 TOPS
	// We could potentially read this from device tree or other sources
	// but for now we return the known spec
	return "1 TOPS"
}

// GetSerialNumber returns the device serial number from u-boot environment.
func GetSerialNumber() string {
	cmd := exec.Command("fw_printenv", "sn")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Output format: sn=XXXX
	parts := strings.SplitN(strings.TrimSpace(string(out)), "=", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// GetDeviceName returns the device hostname.
func GetDeviceName() string {
	// Prefer the persisted device-name file written by the boot identity script.
	// This avoids getting reset on reboot when /etc/hostname is regenerated.
	if data, err := os.ReadFile(DeviceNameFile); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			return name
		}
	}

	data, err := os.ReadFile(HostnameFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// GetOSName returns the operating system name from /etc/issue.
func GetOSName() string {
	data, err := os.ReadFile(IssueFile)
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		return fields[0]
	}
	return ""
}

// GetOSVersion returns the operating system version from /etc/issue.
func GetOSVersion() string {
	data, err := os.ReadFile(IssueFile)
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 2 {
		return fields[1]
	}
	return ""
}

// GetUptime returns the system uptime in milliseconds.
func GetUptime() int64 {
	data, err := os.ReadFile(UptimeFile)
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(seconds * 1000)
}

// GetBootRollback checks if boot rollback is enabled.
func GetBootRollback() bool {
	cmd := exec.Command("fw_printenv", "boot_rollback")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "boot_rollback=1"
}

// GetEMMCSize returns the eMMC storage size in bytes.
func GetEMMCSize() int64 {
	cmd := exec.Command("lsblk", "-b")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == "mmcblk0" {
			size, _ := strconv.ParseInt(fields[3], 10, 64)
			return size
		}
	}
	return 0
}

// HasCANBus checks if CAN bus interface is available.
func HasCANBus() bool {
	_, err := exec.Command("ifconfig", "can0").Output()
	if err != nil {
		return false
	}
	return true
}

// DetectSensor checks for I2C sensor presence and returns sensor type.
// Returns: 0=none, 1=sensor at 0x24, 2=sensor at 0x3f
func DetectSensor() int {
	// Check for sensor at address 0x24 (36)
	cmd := exec.Command("i2cdetect", "-y", "-r", "2", "36", "36")
	out, _ := cmd.Output()
	if strings.Contains(string(out), "36") {
		return 1
	}

	// Check for sensor at address 0x3f (63)
	cmd = exec.Command("i2cdetect", "-y", "-r", "2", "3f", "3f")
	out, _ = cmd.Output()
	if strings.Contains(string(out), "3f") {
		return 2
	}

	return 0
}

// SetDeviceName updates the device hostname and restarts avahi.
func SetDeviceName(name string) error {
	// Persist for the boot identity script (so it survives reboot).
	if err := os.MkdirAll(filepath.Dir(DeviceNameFile), 0755); err == nil {
		_ = os.WriteFile(DeviceNameFile, []byte(name+"\n"), 0644)
	}

	// Write to hostname file
	if err := os.WriteFile(HostnameFile, []byte(name+"\n"), 0644); err != nil {
		return err
	}

	// Set hostname
	if err := exec.Command("hostname", "-F", HostnameFile).Run(); err != nil {
		return err
	}

	// Update avahi config
	avahiConf := "/etc/avahi/avahi-daemon.conf"
	data, err := os.ReadFile(avahiConf)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "host-name=") {
				lines[i] = "host-name=" + name
			}
		}
		os.WriteFile(avahiConf, []byte(strings.Join(lines, "\n")), 0644)
	}

	// Restart avahi
	exec.Command("/etc/init.d/S50avahi-daemon", "stop").Run()
	exec.Command("/etc/init.d/S50avahi-daemon", "start").Run()

	return nil
}
