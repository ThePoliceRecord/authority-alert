package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// UBootEnv represents a U-Boot environment variable
type UBootEnv struct {
	Key   string
	Value string
}

// ReadUBootEnv reads a U-Boot environment variable using fw_printenv
func ReadUBootEnv(key string) (string, error) {
	cmd := exec.Command("fw_printenv", key)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("fw_printenv failed: %w", err)
	}

	// Parse output format: "key=value"
	line := strings.TrimSpace(string(output))
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected fw_printenv output format: %s", line)
	}

	return parts[1], nil
}

// SetUBootEnv sets a U-Boot environment variable using fw_setenv
func SetUBootEnv(key, value string) error {
	cmd := exec.Command("fw_setenv", key, value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fw_setenv failed: %w", err)
	}
	return nil
}

// ReadAllUBootEnv reads all U-Boot environment variables
func ReadAllUBootEnv() (map[string]string, error) {
	cmd := exec.Command("fw_printenv")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fw_printenv failed: %w", err)
	}

	envMap := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading fw_printenv output: %w", err)
	}

	return envMap, nil
}

// GetDeviceInfo retrieves common device identification fields
type DeviceInfo struct {
	SerialNumber string
	MACAddress   string
	Hostname     string
	BoardName    string
}

func GetDeviceInfo() (*DeviceInfo, error) {
	info := &DeviceInfo{}

	// Read serial number
	sn, err := ReadUBootEnv("sn")
	if err != nil {
		log.Printf("Warning: Could not read serial number: %v", err)
	}
	info.SerialNumber = sn

	// Read MAC address
	mac, err := ReadUBootEnv("ethaddr")
	if err != nil {
		log.Printf("Warning: Could not read MAC address: %v", err)
	}
	info.MACAddress = mac

	// Read hostname (if set)
	hostname, err := ReadUBootEnv("hostname")
	if err != nil {
		log.Printf("Warning: Could not read hostname: %v", err)
	}
	info.Hostname = hostname

	// Read board name (if set)
	board, err := ReadUBootEnv("board_name")
	if err != nil {
		log.Printf("Warning: Could not read board name: %v", err)
	}
	info.BoardName = board

	return info, nil
}

func main() {
	// Example 1: Read serial number
	fmt.Println("=== Reading Serial Number ===")
	sn, err := ReadUBootEnv("sn")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Serial Number: %s\n", sn)
	}

	// Example 2: Read MAC address
	fmt.Println("\n=== Reading MAC Address ===")
	mac, err := ReadUBootEnv("ethaddr")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("MAC Address: %s\n", mac)
	}

	// Example 3: Get all device info
	fmt.Println("\n=== Device Information ===")
	info, err := GetDeviceInfo()
	if err != nil {
		log.Fatalf("Failed to get device info: %v", err)
	}
	fmt.Printf("Serial Number: %s\n", info.SerialNumber)
	fmt.Printf("MAC Address:   %s\n", info.MACAddress)
	fmt.Printf("Hostname:      %s\n", info.Hostname)
	fmt.Printf("Board Name:    %s\n", info.BoardName)

	// Example 4: Read all environment variables
	fmt.Println("\n=== All U-Boot Environment Variables ===")
	envMap, err := ReadAllUBootEnv()
	if err != nil {
		log.Fatalf("Failed to read environment: %v", err)
	}

	// Print relevant variables
	relevantKeys := []string{"sn", "ethaddr", "hostname", "board_name", "bootcmd", "bootargs"}
	for _, key := range relevantKeys {
		if value, exists := envMap[key]; exists {
			fmt.Printf("%-15s = %s\n", key, value)
		}
	}

	// Example 5: Set a custom variable (commented out for safety)
	// fmt.Println("\n=== Setting Custom Variable ===")
	// err = SetUBootEnv("custom_field", "my_value")
	// if err != nil {
	// 	log.Printf("Error setting variable: %v", err)
	// } else {
	// 	fmt.Println("Successfully set custom_field=my_value")
	// }
}
