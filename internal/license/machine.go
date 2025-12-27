package license

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// GenerateMachineID creates a unique, hardware-based machine identifier
// Uses multiple sources that are difficult to spoof
func GenerateMachineID() string {
	var parts []string
	
	// 1. System machine-id (standard)
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		parts = append(parts, strings.TrimSpace(string(data)))
	}
	
	// 2. DMI Product UUID (hardware-based, harder to fake)
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		parts = append(parts, strings.TrimSpace(string(data)))
	} else {
		// Try dmidecode as fallback (requires root typically)
		if out, err := exec.Command("dmidecode", "-s", "system-uuid").Output(); err == nil {
			parts = append(parts, strings.TrimSpace(string(out)))
		}
	}
	
	// 3. Root disk serial number
	rootDisk := getRootDiskSerial()
	if rootDisk != "" {
		parts = append(parts, rootDisk)
	}
	
	// 4. Primary network interface MAC address
	mac := getPrimaryMAC()
	if mac != "" {
		parts = append(parts, mac)
	}
	
	// 5. CPU info
	cpuInfo := getCPUInfo()
	if cpuInfo != "" {
		parts = append(parts, cpuInfo)
	}
	
	// 6. GPU identifiers
	gpuInfo := getGPUInfo()
	if gpuInfo != "" {
		parts = append(parts, gpuInfo)
	}
	
	// 7. Hostname + username
	if hostname, err := os.Hostname(); err == nil {
		parts = append(parts, hostname)
	}
	
	if u, err := user.Current(); err == nil {
		parts = append(parts, u.Username)
	}
	
	// Hash everything with SHA-256
	combined := strings.Join(parts, "")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// getRootDiskSerial gets the serial number of the root disk
func getRootDiskSerial() string {
	// Find root disk device
	out, err := exec.Command("df", "/").Output()
	if err != nil {
		return ""
	}
	
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return ""
	}
	
	fields := strings.Fields(lines[1])
	if len(fields) == 0 {
		return ""
	}
	
	device := fields[0]
	// Strip partition number (e.g., /dev/sda1 -> sda)
	device = filepath.Base(device)
	for len(device) > 0 && device[len(device)-1] >= '0' && device[len(device)-1] <= '9' {
		device = device[:len(device)-1]
	}
	
	// Try to read serial from sysfs
	serialPath := filepath.Join("/sys/block", device, "device/serial")
	if data, err := os.ReadFile(serialPath); err == nil {
		return strings.TrimSpace(string(data))
	}
	
	// Try udevadm
	out, err = exec.Command("udevadm", "info", "--query=property", "--name=/dev/"+device).Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "ID_SERIAL=") {
				return strings.TrimPrefix(line, "ID_SERIAL=")
			}
		}
	}
	
	return ""
}

// getPrimaryMAC gets the MAC address of the primary network interface
func getPrimaryMAC() string {
	// Find primary interface by looking at default route
	out, err := exec.Command("ip", "route", "get", "1.1.1.1").Output()
	if err != nil {
		return ""
	}
	
	// Parse "dev ethX" from output
	parts := strings.Fields(string(out))
	for i, part := range parts {
		if part == "dev" && i+1 < len(parts) {
			ifaceName := parts[i+1]
			
			// Get MAC address
			iface, err := net.InterfaceByName(ifaceName)
			if err == nil && len(iface.HardwareAddr) > 0 {
				return iface.HardwareAddr.String()
			}
		}
	}
	
	return ""
}

// getCPUInfo gets CPU model and family information
func getCPUInfo() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	
	var parts []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "cpu family") {
			if idx := strings.Index(line, ":"); idx != -1 {
				parts = append(parts, strings.TrimSpace(line[idx+1:]))
			}
			if len(parts) >= 2 {
				break
			}
		}
	}
	
	return strings.Join(parts, "|")
}

// getGPUInfo gets GPU identifiers
func getGPUInfo() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return ""
	}
	
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d") || strings.Contains(lower, "display") {
			return strings.TrimSpace(line)
		}
	}
	
	return ""
}

