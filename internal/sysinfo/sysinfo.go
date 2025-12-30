// Package sysinfo provides comprehensive system detection for wizado
// This collects hardware, software, and environment information for diagnostics,
// optimal configuration, and future analytics.
package sysinfo

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// SystemInfo contains all detected system information
type SystemInfo struct {
	CollectedAt time.Time `json:"collected_at"`

	// Hardware
	CPU     CPUInfo     `json:"cpu"`
	GPU     GPUInfo     `json:"gpu"`
	Memory  MemoryInfo  `json:"memory"`
	Display DisplayInfo `json:"display"`

	// Input Devices
	Input InputInfo `json:"input"`

	// Network
	Network NetworkInfo `json:"network"`

	// Software
	OS           OSInfo           `json:"os"`
	Desktop      DesktopInfo      `json:"desktop"`
	Dependencies DependenciesInfo `json:"dependencies"`

	// Wizado-specific
	WizadoVersion string `json:"wizado_version"`
}

// CPUInfo contains CPU details
type CPUInfo struct {
	Model     string `json:"model"`
	Vendor    string `json:"vendor"`
	Cores     int    `json:"cores"`
	Threads   int    `json:"threads"`
	Frequency string `json:"frequency"`
}

// GPUInfo contains GPU details
type GPUInfo struct {
	HasNVIDIA    bool   `json:"has_nvidia"`
	HasAMD       bool   `json:"has_amd"`
	HasIntel     bool   `json:"has_intel"`
	Primary      string `json:"primary"`
	PrimaryID    string `json:"primary_id"` // Vulkan device ID
	DriverVersion string `json:"driver_version"`
	VRAMMiB      int    `json:"vram_mib"`
}

// MemoryInfo contains RAM details
type MemoryInfo struct {
	TotalMiB     int `json:"total_mib"`
	AvailableMiB int `json:"available_mib"`
	SwapTotalMiB int `json:"swap_total_mib"`
}

// DisplayInfo contains monitor details
type DisplayInfo struct {
	Count      int       `json:"count"`
	Primary    Monitor   `json:"primary"`
	All        []Monitor `json:"all,omitempty"`
}

// Monitor represents a single display
type Monitor struct {
	Name       string  `json:"name"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	RefreshHz  float64 `json:"refresh_hz"`
	Scale      float64 `json:"scale"`
	IsPrimary  bool    `json:"is_primary"`
}

// InputInfo contains detected input devices
type InputInfo struct {
	Keyboards   []InputDevice `json:"keyboards"`
	Mice        []InputDevice `json:"mice"`
	Controllers []InputDevice `json:"controllers"`
	HasKeyboard bool          `json:"has_keyboard"`
	HasMouse    bool          `json:"has_mouse"`
	HasController bool        `json:"has_controller"`
}

// InputDevice represents an input device
type InputDevice struct {
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Type   string `json:"type"`
	Vendor string `json:"vendor,omitempty"`
}

// NetworkInfo contains network status
type NetworkInfo struct {
	HasInternet    bool   `json:"has_internet"`
	PrimaryIF      string `json:"primary_interface"`
	ConnectionType string `json:"connection_type"` // "ethernet", "wifi", "unknown"
	SSID           string `json:"ssid,omitempty"`  // WiFi network name if applicable
}

// OSInfo contains operating system details
type OSInfo struct {
	Name         string `json:"name"`
	ID           string `json:"id"`
	Version      string `json:"version"`
	Kernel       string `json:"kernel"`
	Architecture string `json:"architecture"`
}

// DesktopInfo contains desktop environment details
type DesktopInfo struct {
	Session       string `json:"session"`       // "hyprland", "sway", etc.
	Compositor    string `json:"compositor"`
	Version       string `json:"version"`
	WaylandDisplay string `json:"wayland_display"`
	IsWayland     bool   `json:"is_wayland"`
}

// DependenciesInfo contains versions of required software
type DependenciesInfo struct {
	Steam      PackageInfo `json:"steam"`
	Gamescope  PackageInfo `json:"gamescope"`
	GameMode   PackageInfo `json:"gamemode"`
	MangoHUD   PackageInfo `json:"mangohud"`
	Hyprland   PackageInfo `json:"hyprland"`
}

// PackageInfo represents a software package
type PackageInfo struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Path      string `json:"path,omitempty"`
}

// Collect gathers all system information
func Collect(wizadoVersion string) *SystemInfo {
	info := &SystemInfo{
		CollectedAt:   time.Now().UTC(),
		WizadoVersion: wizadoVersion,
	}

	info.CPU = collectCPU()
	info.GPU = collectGPU()
	info.Memory = collectMemory()
	info.Display = collectDisplay()
	info.Input = collectInput()
	info.Network = collectNetwork()
	info.OS = collectOS()
	info.Desktop = collectDesktop()
	info.Dependencies = collectDependencies()

	return info
}

// collectCPU gathers CPU information
func collectCPU() CPUInfo {
	info := CPUInfo{
		Cores:   runtime.NumCPU(),
		Threads: runtime.NumCPU(),
	}

	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return info
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "model name") {
			if idx := strings.Index(line, ":"); idx != -1 {
				info.Model = strings.TrimSpace(line[idx+1:])
			}
		} else if strings.HasPrefix(line, "vendor_id") {
			if idx := strings.Index(line, ":"); idx != -1 {
				info.Vendor = strings.TrimSpace(line[idx+1:])
			}
		} else if strings.HasPrefix(line, "cpu MHz") {
			if idx := strings.Index(line, ":"); idx != -1 {
				info.Frequency = strings.TrimSpace(line[idx+1:]) + " MHz"
			}
		}

		if info.Model != "" && info.Vendor != "" && info.Frequency != "" {
			break
		}
	}

	// Count physical cores vs threads
	coreIDs := make(map[string]bool)
	for _, line := range lines {
		if strings.HasPrefix(line, "core id") {
			if idx := strings.Index(line, ":"); idx != -1 {
				coreIDs[strings.TrimSpace(line[idx+1:])] = true
			}
		}
	}
	if len(coreIDs) > 0 {
		info.Cores = len(coreIDs)
	}

	return info
}

// collectGPU gathers GPU information
func collectGPU() GPUInfo {
	info := GPUInfo{}

	out, err := exec.Command("lspci", "-nn").Output()
	if err != nil {
		return info
	}

	lspciOutput := string(out)
	lines := strings.Split(lspciOutput, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Check for NVIDIA
		if strings.Contains(lower, "nvidia") && (strings.Contains(lower, "vga") || strings.Contains(lower, "3d")) {
			info.HasNVIDIA = true
			if info.Primary == "" {
				info.Primary = extractGPUName(line)
				// Extract Vulkan device ID [10de:XXXX]
				if idx := strings.Index(line, "[10de:"); idx != -1 {
					end := strings.Index(line[idx:], "]")
					if end != -1 {
						info.PrimaryID = strings.Trim(line[idx:idx+end+1], "[]")
					}
				}
			}
		}

		// Check for AMD
		if (strings.Contains(lower, "amd") || strings.Contains(lower, "radeon")) &&
			(strings.Contains(lower, "vga") || strings.Contains(lower, "3d")) {
			info.HasAMD = true
			if info.Primary == "" {
				info.Primary = extractGPUName(line)
			}
		}

		// Check for Intel
		if strings.Contains(lower, "intel") && strings.Contains(lower, "vga") {
			info.HasIntel = true
			// Only set as primary if no dedicated GPU found
			if info.Primary == "" && !info.HasNVIDIA && !info.HasAMD {
				info.Primary = extractGPUName(line)
			}
		}
	}

	// Get NVIDIA driver version
	if info.HasNVIDIA {
		if out, err := exec.Command("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader").Output(); err == nil {
			info.DriverVersion = strings.TrimSpace(string(out))
		}

		// Get VRAM
		if out, err := exec.Command("nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits").Output(); err == nil {
			var vram int
			if _, err := strings.NewReader(strings.TrimSpace(string(out))).Read([]byte{}); err == nil {
				if n, _ := strings.NewReader(strings.TrimSpace(string(out))).Read([]byte{}); n > 0 {
					// Parse MiB value
					vramStr := strings.TrimSpace(string(out))
					var v int
					if _, err := exec.Command("echo", vramStr).Output(); err == nil {
						// Simple parse
						for _, c := range vramStr {
							if c >= '0' && c <= '9' {
								v = v*10 + int(c-'0')
							}
						}
						vram = v
					}
				}
			}
			info.VRAMMiB = vram
		}
	}

	return info
}

// extractGPUName extracts the GPU name from lspci output
func extractGPUName(line string) string {
	// Format: "XX:XX.X VGA compatible controller: NVIDIA Corporation GeForce RTX 4090 [10de:2684]"
	parts := strings.SplitN(line, ":", 3)
	if len(parts) >= 3 {
		name := strings.TrimSpace(parts[2])
		// Remove the PCI ID at the end
		if idx := strings.LastIndex(name, "["); idx != -1 {
			name = strings.TrimSpace(name[:idx])
		}
		return name
	}
	return ""
}

// collectMemory gathers memory information
func collectMemory() MemoryInfo {
	info := MemoryInfo{}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return info
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			info.TotalMiB = parseMemValue(line) / 1024
		} else if strings.HasPrefix(line, "MemAvailable:") {
			info.AvailableMiB = parseMemValue(line) / 1024
		} else if strings.HasPrefix(line, "SwapTotal:") {
			info.SwapTotalMiB = parseMemValue(line) / 1024
		}
	}

	return info
}

func parseMemValue(line string) int {
	// Format: "MemTotal:       32456789 kB"
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		var value int
		for _, c := range fields[1] {
			if c >= '0' && c <= '9' {
				value = value*10 + int(c-'0')
			}
		}
		return value
	}
	return 0
}

// collectDisplay gathers monitor information from Hyprland
func collectDisplay() DisplayInfo {
	info := DisplayInfo{}

	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err != nil {
		return info
	}

	var monitors []struct {
		Name       string  `json:"name"`
		Width      int     `json:"width"`
		Height     int     `json:"height"`
		RefreshRate float64 `json:"refreshRate"`
		Scale      float64 `json:"scale"`
		Focused    bool    `json:"focused"`
	}

	if err := json.Unmarshal(out, &monitors); err != nil {
		return info
	}

	info.Count = len(monitors)

	for i, m := range monitors {
		monitor := Monitor{
			Name:      m.Name,
			Width:     m.Width,
			Height:    m.Height,
			RefreshHz: m.RefreshRate,
			Scale:     m.Scale,
			IsPrimary: m.Focused || i == 0,
		}
		info.All = append(info.All, monitor)

		if monitor.IsPrimary {
			info.Primary = monitor
		}
	}

	return info
}

// collectInput gathers input device information
func collectInput() InputInfo {
	info := InputInfo{}

	// Use libinput to list devices
	out, err := exec.Command("libinput", "list-devices").Output()
	if err == nil {
		parseLibinputDevices(string(out), &info)
	}

	// Fallback: check /dev/input for devices
	if len(info.Keyboards) == 0 && len(info.Mice) == 0 {
		checkDevInputDevices(&info)
	}

	// Check for game controllers specifically
	checkGameControllers(&info)

	info.HasKeyboard = len(info.Keyboards) > 0
	info.HasMouse = len(info.Mice) > 0
	info.HasController = len(info.Controllers) > 0

	return info
}

func parseLibinputDevices(output string, info *InputInfo) {
	blocks := strings.Split(output, "Device:")
	for _, block := range blocks {
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		device := InputDevice{}

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Name:") || (len(lines) > 0 && strings.TrimSpace(lines[0]) != "" && !strings.Contains(lines[0], ":")) {
				// First line after Device: is the name
				if device.Name == "" {
					device.Name = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "Name:"))
				}
			}
			if strings.HasPrefix(line, "Kernel:") {
				device.Path = strings.TrimSpace(strings.TrimPrefix(line, "Kernel:"))
			}
			if strings.HasPrefix(line, "Capabilities:") {
				caps := strings.ToLower(line)
				if strings.Contains(caps, "keyboard") {
					device.Type = "keyboard"
					info.Keyboards = append(info.Keyboards, device)
				} else if strings.Contains(caps, "pointer") {
					device.Type = "mouse"
					info.Mice = append(info.Mice, device)
				}
			}
		}
	}
}

func checkDevInputDevices(info *InputInfo) {
	// Read /proc/bus/input/devices for input device info
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return
	}

	blocks := strings.Split(string(data), "\n\n")
	for _, block := range blocks {
		if block == "" {
			continue
		}

		device := InputDevice{}
		isKeyboard := false
		isMouse := false

		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "N: Name=") {
				device.Name = strings.Trim(strings.TrimPrefix(line, "N: Name="), "\"")
			}
			if strings.HasPrefix(line, "H: Handlers=") {
				handlers := strings.ToLower(line)
				if strings.Contains(handlers, "kbd") {
					isKeyboard = true
				}
				if strings.Contains(handlers, "mouse") {
					isMouse = true
				}
			}
		}

		if device.Name != "" {
			lower := strings.ToLower(device.Name)
			// Skip virtual/internal devices
			if strings.Contains(lower, "power button") ||
				strings.Contains(lower, "sleep button") ||
				strings.Contains(lower, "video bus") ||
				strings.Contains(lower, "pc speaker") {
				continue
			}

			if isKeyboard {
				device.Type = "keyboard"
				info.Keyboards = append(info.Keyboards, device)
			} else if isMouse {
				device.Type = "mouse"
				info.Mice = append(info.Mice, device)
			}
		}
	}
}

func checkGameControllers(info *InputInfo) {
	// Check /dev/input/js* for joysticks
	matches, _ := filepath.Glob("/dev/input/js*")
	for _, path := range matches {
		device := InputDevice{
			Path: path,
			Type: "controller",
			Name: "Game Controller",
		}

		// Try to get the name from sysfs
		jsNum := strings.TrimPrefix(filepath.Base(path), "js")
		namePath := "/sys/class/input/js" + jsNum + "/device/name"
		if data, err := os.ReadFile(namePath); err == nil {
			device.Name = strings.TrimSpace(string(data))
		}

		info.Controllers = append(info.Controllers, device)
	}

	// Also check for Steam Controller and other devices via /proc/bus/input/devices
	data, _ := os.ReadFile("/proc/bus/input/devices")
	for _, block := range strings.Split(string(data), "\n\n") {
		lower := strings.ToLower(block)
		if strings.Contains(lower, "gamepad") ||
			strings.Contains(lower, "controller") ||
			strings.Contains(lower, "xbox") ||
			strings.Contains(lower, "playstation") ||
			strings.Contains(lower, "dualsense") ||
			strings.Contains(lower, "dualshock") ||
			strings.Contains(lower, "steam") {

			for _, line := range strings.Split(block, "\n") {
				if strings.HasPrefix(line, "N: Name=") {
					name := strings.Trim(strings.TrimPrefix(line, "N: Name="), "\"")
					// Check if we already have this controller
					found := false
					for _, c := range info.Controllers {
						if c.Name == name {
							found = true
							break
						}
					}
					if !found {
						info.Controllers = append(info.Controllers, InputDevice{
							Name: name,
							Type: "controller",
						})
					}
				}
			}
		}
	}
}

// collectNetwork gathers network information
func collectNetwork() NetworkInfo {
	info := NetworkInfo{}

	// Check internet connectivity
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://connectivitycheck.gstatic.com/generate_204")
	if err == nil {
		resp.Body.Close()
		info.HasInternet = resp.StatusCode == 204
	}

	// Find primary interface
	out, err := exec.Command("ip", "route", "get", "1.1.1.1").Output()
	if err == nil {
		// Parse "dev ethX" from output
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				info.PrimaryIF = fields[i+1]
				break
			}
		}
	}

	// Determine connection type
	if info.PrimaryIF != "" {
		lower := strings.ToLower(info.PrimaryIF)
		if strings.HasPrefix(lower, "wl") || strings.HasPrefix(lower, "wifi") {
			info.ConnectionType = "wifi"
			// Try to get SSID
			out, err := exec.Command("iwgetid", "-r").Output()
			if err == nil {
				info.SSID = strings.TrimSpace(string(out))
			}
		} else if strings.HasPrefix(lower, "eth") || strings.HasPrefix(lower, "en") {
			info.ConnectionType = "ethernet"
		} else {
			info.ConnectionType = "unknown"
		}
	}

	return info
}

// collectOS gathers operating system information
func collectOS() OSInfo {
	info := OSInfo{
		Architecture: runtime.GOARCH,
	}

	// Get kernel version
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(out))
	}

	// Parse /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "NAME=") {
				info.Name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
			} else if strings.HasPrefix(line, "ID=") {
				info.ID = strings.TrimPrefix(line, "ID=")
			} else if strings.HasPrefix(line, "VERSION_ID=") {
				info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	return info
}

// collectDesktop gathers desktop environment information
func collectDesktop() DesktopInfo {
	info := DesktopInfo{
		WaylandDisplay: os.Getenv("WAYLAND_DISPLAY"),
		IsWayland:      os.Getenv("WAYLAND_DISPLAY") != "",
	}

	// Detect Hyprland
	if out, err := exec.Command("hyprctl", "version", "-j").Output(); err == nil {
		info.Session = "hyprland"
		info.Compositor = "Hyprland"

		var version struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(out, &version); err == nil {
			info.Version = version.Version
		} else {
			// Try plain text parsing
			text := string(out)
			if idx := strings.Index(text, "Hyprland "); idx != -1 {
				rest := text[idx+9:]
				if end := strings.IndexAny(rest, " \n"); end != -1 {
					info.Version = rest[:end]
				} else {
					info.Version = rest
				}
			}
		}
	} else if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		info.Session = "hyprland"
		info.Compositor = "Hyprland"
	}

	return info
}

// collectDependencies gathers information about required software
func collectDependencies() DependenciesInfo {
	info := DependenciesInfo{}

	// Steam - don't use --version as it launches the full client!
	// Use pacman to get version instead
	info.Steam = getPackageInfo("steam", nil)

	// Gamescope
	info.Gamescope = getPackageInfo("gamescope", []string{"--version"})

	// GameMode
	info.GameMode = getPackageInfo("gamemoded", []string{"-v"})
	if !info.GameMode.Installed {
		// Try alternative check
		info.GameMode = getPackageInfo("gamemoderun", nil)
	}

	// MangoHUD
	info.MangoHUD = getPackageInfo("mangohud", []string{"--version"})

	// Hyprland
	info.Hyprland = getHyprlandInfo()

	return info
}

func getPackageInfo(name string, versionArgs []string) PackageInfo {
	info := PackageInfo{}

	// Check if binary exists
	path, err := exec.LookPath(name)
	if err != nil {
		return info
	}

	info.Installed = true
	info.Path = path

	// Try to get version
	if versionArgs != nil {
		out, err := exec.Command(name, versionArgs...).CombinedOutput()
		if err == nil {
			// Extract version number from output
			info.Version = extractVersion(string(out))
		}
	}

	// Fallback: try pacman
	if info.Version == "" {
		out, err := exec.Command("pacman", "-Qi", name).Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "Version") {
					if idx := strings.Index(line, ":"); idx != -1 {
						info.Version = strings.TrimSpace(line[idx+1:])
						break
					}
				}
			}
		}
	}

	return info
}

func getHyprlandInfo() PackageInfo {
	info := PackageInfo{}

	path, err := exec.LookPath("Hyprland")
	if err != nil {
		path, err = exec.LookPath("hyprland")
	}

	if err != nil {
		return info
	}

	info.Installed = true
	info.Path = path

	// Get version from hyprctl
	out, err := exec.Command("hyprctl", "version").Output()
	if err == nil {
		info.Version = extractVersion(string(out))
	}

	return info
}

// extractVersion extracts a version number from command output
func extractVersion(output string) string {
	// Common patterns: "vX.Y.Z", "X.Y.Z", "version X.Y.Z"
	patterns := []string{
		`v?(\d+\.\d+(?:\.\d+)?(?:-[a-zA-Z0-9]+)?)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(output); len(match) > 1 {
			return match[1]
		}
	}

	// Just return first line if no pattern matches
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return ""
}

// CheckInternet performs a quick internet connectivity check
func CheckInternet() bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://connectivitycheck.gstatic.com/generate_204")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 204
}

// GetPrimaryMAC returns the MAC address of the primary network interface
func GetPrimaryMAC() string {
	out, err := exec.Command("ip", "route", "get", "1.1.1.1").Output()
	if err != nil {
		return ""
	}

	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			ifaceName := fields[i+1]
			iface, err := net.InterfaceByName(ifaceName)
			if err == nil && len(iface.HardwareAddr) > 0 {
				return iface.HardwareAddr.String()
			}
		}
	}

	return ""
}

// ToJSON serializes the system info to JSON
func (s *SystemInfo) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// Summary returns a human-readable summary of the system
func (s *SystemInfo) Summary() string {
	var b strings.Builder

	b.WriteString("System Information\n")
	b.WriteString("══════════════════\n\n")

	// Hardware
	b.WriteString("Hardware:\n")
	b.WriteString("  CPU: " + s.CPU.Model + "\n")
	b.WriteString("  GPU: " + s.GPU.Primary)
	if s.GPU.DriverVersion != "" {
		b.WriteString(" (Driver: " + s.GPU.DriverVersion + ")")
	}
	b.WriteString("\n")
	b.WriteString("  RAM: " + formatMiB(s.Memory.TotalMiB) + "\n")

	// Display
	if s.Display.Primary.Width > 0 {
		b.WriteString("  Display: " + s.Display.Primary.Name + " @ ")
		b.WriteString(strings.Repeat(" ", 0))
		b.WriteString(formatResolution(s.Display.Primary.Width, s.Display.Primary.Height, s.Display.Primary.RefreshHz))
		b.WriteString("\n")
	}

	// Input
	b.WriteString("\nInput Devices:\n")
	if s.Input.HasKeyboard {
		b.WriteString("  ✓ Keyboard")
		if len(s.Input.Keyboards) > 0 {
			b.WriteString(" (" + s.Input.Keyboards[0].Name + ")")
		}
		b.WriteString("\n")
	}
	if s.Input.HasMouse {
		b.WriteString("  ✓ Mouse")
		if len(s.Input.Mice) > 0 {
			b.WriteString(" (" + s.Input.Mice[0].Name + ")")
		}
		b.WriteString("\n")
	}
	if s.Input.HasController {
		b.WriteString("  ✓ Controller")
		if len(s.Input.Controllers) > 0 {
			b.WriteString(" (" + s.Input.Controllers[0].Name + ")")
		}
		b.WriteString("\n")
	}

	// Network
	b.WriteString("\nNetwork:\n")
	if s.Network.HasInternet {
		b.WriteString("  ✓ Internet connected")
	} else {
		b.WriteString("  ✗ No internet")
	}
	if s.Network.ConnectionType != "" {
		b.WriteString(" via " + s.Network.ConnectionType)
	}
	if s.Network.SSID != "" {
		b.WriteString(" (" + s.Network.SSID + ")")
	}
	b.WriteString("\n")

	// Software
	b.WriteString("\nSoftware:\n")
	b.WriteString("  OS: " + s.OS.Name + " " + s.OS.Version + "\n")
	b.WriteString("  Kernel: " + s.OS.Kernel + "\n")
	b.WriteString("  Desktop: " + s.Desktop.Compositor + " " + s.Desktop.Version + "\n")

	// Dependencies
	b.WriteString("\nDependencies:\n")
	printDep(&b, "Steam", s.Dependencies.Steam)
	printDep(&b, "Gamescope", s.Dependencies.Gamescope)
	printDep(&b, "GameMode", s.Dependencies.GameMode)
	printDep(&b, "MangoHUD", s.Dependencies.MangoHUD)
	printDep(&b, "Hyprland", s.Dependencies.Hyprland)

	return b.String()
}

func printDep(b *strings.Builder, name string, pkg PackageInfo) {
	if pkg.Installed {
		b.WriteString("  ✓ " + name)
		if pkg.Version != "" {
			b.WriteString(" " + pkg.Version)
		}
	} else {
		b.WriteString("  ✗ " + name + " (not installed)")
	}
	b.WriteString("\n")
}

func formatMiB(mib int) string {
	if mib >= 1024 {
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(
				string([]byte{byte(mib/1024/10+'0'), '.', byte(mib/1024%10+'0')}),
				".0", "", 1),
			"0"), ".") + " GiB"
	}
	return string(rune(mib)) + " MiB"
}

func formatResolution(w, h int, hz float64) string {
	return strings.Join([]string{
		itoa(w), "x", itoa(h), " @ ", itoa(int(hz)), "Hz",
	}, "")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte(i%10) + '0'
		i /= 10
	}
	return string(b[pos:])
}

