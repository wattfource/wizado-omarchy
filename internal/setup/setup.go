// Package setup handles system configuration for wizado
package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wattfource/wizado/internal/logging"
	"github.com/wattfource/wizado/internal/sysinfo"
)

// GPUInfo holds detected GPU information
type GPUInfo struct {
	HasNVIDIA  bool
	HasAMD     bool
	HasIntel   bool
	NVIDIAVkID string
}

// Options for setup
type Options struct {
	NonInteractive bool
	DryRun         bool
}

var log *logging.Logger

func init() {
	log = logging.WithComponent("setup")
}

// Run performs the full setup
func Run(opts Options) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("  WIZADO - Steam Gaming Launcher Setup")
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()
	
	log.Info("Starting wizado setup")
	
	// Validate environment first
	if err := validateEnvironment(); err != nil {
		log.Errorf("Environment validation failed: %v", err)
		return err
	}
	
	// Check internet connectivity
	if !checkInternet() {
		fmt.Println("⚠ Warning: No internet connection detected")
		fmt.Println("  Some features may not work correctly without internet.")
		fmt.Println()
		log.Warn("No internet connection detected")
		
		if !opts.NonInteractive {
			if !confirm("Continue anyway?") {
				return fmt.Errorf("setup cancelled - internet required for package installation")
			}
		}
	} else {
		fmt.Println("✓ Internet connection: OK")
		log.Info("Internet connection verified")
	}
	
	// Collect and display system information
	fmt.Println()
	fmt.Println("Detecting system configuration...")
	sysInfo := sysinfo.Collect("setup")
	
	// Display detected info
	printSystemInfo(sysInfo)
	
	// Convert to our GPUInfo type
	gpu := GPUInfo{
		HasNVIDIA:  sysInfo.GPU.HasNVIDIA,
		HasAMD:     sysInfo.GPU.HasAMD,
		HasIntel:   sysInfo.GPU.HasIntel,
		NVIDIAVkID: sysInfo.GPU.PrimaryID,
	}
	
	// Confirm installation
	if !opts.NonInteractive {
		if !confirm("Install wizado Steam gaming launcher?") {
			return fmt.Errorf("installation cancelled")
		}
	}
	
	// Enable multilib
	if err := ensureMultilib(opts); err != nil {
		return err
	}
	
	// Install dependencies
	if err := installDependencies(gpu, opts); err != nil {
		return err
	}
	
	// Install optional packages
	if err := installOptionalPackages(opts); err != nil {
		// Non-fatal
		fmt.Printf("Warning: some optional packages failed to install: %v\n", err)
		log.Warnf("Optional packages failed: %v", err)
	}
	
	// Check user groups
	if err := checkUserGroups(opts); err != nil {
		return err
	}
	
	// Grant gamescope capabilities
	if err := grantGamescopeCap(opts); err != nil {
		// Non-fatal
		fmt.Printf("Warning: could not grant gamescope cap_sys_nice: %v\n", err)
		log.Warnf("Gamescope cap_sys_nice failed: %v", err)
	}
	
	// Configure Hyprland keybindings
	if err := configureKeybindings(opts); err != nil {
		return err
	}
	
	// Configure Waybar
	if err := configureWaybar(opts); err != nil {
		// Non-fatal
		fmt.Printf("Warning: could not configure waybar: %v\n", err)
		log.Warnf("Waybar config failed: %v", err)
	}
	
	// Print success
	printSuccess(gpu, sysInfo)
	
	log.Info("Setup completed successfully")
	
	return nil
}

func checkInternet() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Try multiple endpoints
	endpoints := []string{
		"http://connectivitycheck.gstatic.com/generate_204",
		"http://www.google.com",
		"http://archlinux.org",
	}
	
	for _, url := range endpoints {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
	}
	
	return false
}

func printSystemInfo(info *sysinfo.SystemInfo) {
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│  SYSTEM INFORMATION                                         │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	
	// Hardware
	fmt.Printf("│  CPU: %-53s │\n", truncate(info.CPU.Model, 53))
	
	gpuStr := info.GPU.Primary
	if info.GPU.DriverVersion != "" {
		gpuStr += " (v" + info.GPU.DriverVersion + ")"
	}
	fmt.Printf("│  GPU: %-53s │\n", truncate(gpuStr, 53))
	
	fmt.Printf("│  RAM: %d GiB                                                 │\n", info.Memory.TotalMiB/1024)
	
	if info.Display.Primary.Width > 0 {
		fmt.Printf("│  Display: %dx%d @ %.0f Hz                                  │\n",
			info.Display.Primary.Width, info.Display.Primary.Height, info.Display.Primary.RefreshHz)
	}
	
	// Input devices
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Println("│  INPUT DEVICES                                              │")
	
	if info.Input.HasKeyboard {
		kbName := "detected"
		if len(info.Input.Keyboards) > 0 {
			kbName = info.Input.Keyboards[0].Name
		}
		fmt.Printf("│  ✓ Keyboard: %-46s │\n", truncate(kbName, 46))
	} else {
		fmt.Println("│  ✗ Keyboard: not detected                                   │")
	}
	
	if info.Input.HasMouse {
		mouseName := "detected"
		if len(info.Input.Mice) > 0 {
			mouseName = info.Input.Mice[0].Name
		}
		fmt.Printf("│  ✓ Mouse: %-49s │\n", truncate(mouseName, 49))
	} else {
		fmt.Println("│  ✗ Mouse: not detected                                      │")
	}
	
	if info.Input.HasController {
		ctrlName := "detected"
		if len(info.Input.Controllers) > 0 {
			ctrlName = info.Input.Controllers[0].Name
		}
		fmt.Printf("│  ✓ Controller: %-44s │\n", truncate(ctrlName, 44))
	} else {
		fmt.Println("│  ○ Controller: not connected (optional)                     │")
	}
	
	// Software/Dependencies
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Println("│  DEPENDENCIES                                               │")
	
	printDepStatus("Steam", info.Dependencies.Steam)
	printDepStatus("Gamescope", info.Dependencies.Gamescope)
	printDepStatus("GameMode", info.Dependencies.GameMode)
	printDepStatus("MangoHUD", info.Dependencies.MangoHUD)
	printDepStatus("Hyprland", info.Dependencies.Hyprland)
	
	// Network
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Println("│  NETWORK                                                    │")
	
	if info.Network.HasInternet {
		connType := info.Network.ConnectionType
		if connType == "" {
			connType = "connected"
		}
		if info.Network.SSID != "" {
			connType = "WiFi: " + info.Network.SSID
		}
		fmt.Printf("│  ✓ Internet: %-46s │\n", connType)
	} else {
		fmt.Println("│  ✗ Internet: not connected                                  │")
	}
	
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()
}

func printDepStatus(name string, pkg sysinfo.PackageInfo) {
	if pkg.Installed {
		version := pkg.Version
		if version == "" {
			version = "installed"
		}
		fmt.Printf("│  ✓ %-10s %-47s │\n", name+":", truncate(version, 47))
	} else {
		fmt.Printf("│  ✗ %-10s %-47s │\n", name+":", "not installed")
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func validateEnvironment() error {
	// Check for pacman
	if _, err := exec.LookPath("pacman"); err != nil {
		return fmt.Errorf("pacman not found - this tool is for Arch Linux")
	}
	
	// Check for hyprctl
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return fmt.Errorf("hyprctl not found - is Hyprland installed?")
	}
	
	// Check for Hyprland config
	home, _ := os.UserHomeDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	if _, err := os.Stat(hyprDir); os.IsNotExist(err) {
		return fmt.Errorf("Hyprland config directory not found: %s", hyprDir)
	}
	
	// Verify we're on Wayland (if in a session)
	if os.Getenv("DISPLAY") != "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Println("⚠ Warning: X11 session detected. Wizado requires Wayland/Hyprland.")
	}
	
	return nil
}

func detectGPUs() GPUInfo {
	var info GPUInfo
	
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return info
	}
	
	lspciOutput := strings.ToLower(string(out))
	
	if strings.Contains(lspciOutput, "nvidia") {
		info.HasNVIDIA = true
		// Get Vulkan device ID
		out, err := exec.Command("lspci", "-nn").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(strings.ToLower(line), "nvidia") {
					if idx := strings.Index(line, "[10de:"); idx != -1 {
						end := strings.Index(line[idx:], "]")
						if end != -1 {
							info.NVIDIAVkID = strings.Trim(line[idx:idx+end+1], "[]")
							break
						}
					}
				}
			}
		}
	}
	
	if strings.Contains(lspciOutput, "amd") || strings.Contains(lspciOutput, "radeon") {
		info.HasAMD = true
	}
	
	if strings.Contains(lspciOutput, "intel") {
		info.HasIntel = true
	}
	
	return info
}

func ensureMultilib(opts Options) error {
	// Check if multilib is enabled
	data, err := os.ReadFile("/etc/pacman.conf")
	if err != nil {
		return err
	}
	
	if strings.Contains(string(data), "[multilib]") && !strings.Contains(string(data), "#[multilib]") {
		fmt.Println("✓ Multilib repository: enabled")
		return nil
	}
	
	fmt.Println("⚠ Multilib repository NOT enabled (required for Steam 32-bit libraries)")
	log.Warn("Multilib repository not enabled")
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would enable multilib in /etc/pacman.conf")
		return nil
	}
	
	if !opts.NonInteractive {
		if !confirm("Enable multilib in /etc/pacman.conf?") {
			return fmt.Errorf("multilib required for Steam")
		}
	}
	
	// Enable multilib using sed
	cmd := exec.Command("sudo", "sed", "-i", "/^#\\[multilib\\]/,/^#Include/ s/^#//", "/etc/pacman.conf")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable multilib: %v", err)
	}
	
	// Refresh package database
	fmt.Println("Refreshing package database...")
	cmd = exec.Command("sudo", "pacman", "-Syy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to refresh package database: %v", err)
	}
	
	log.Info("Multilib repository enabled")
	return nil
}

func installDependencies(gpu GPUInfo, opts Options) error {
	// Core dependencies
	deps := []string{
		"steam",
		"gamescope",
		"jq",
		"bc",
		"lib32-vulkan-icd-loader",
		"vulkan-icd-loader",
		"lib32-mesa",
		"mesa",
		"pciutils",
		"libinput", // For input device detection
	}
	
	// GPU-specific drivers
	if gpu.HasNVIDIA {
		deps = append(deps, "nvidia-utils", "lib32-nvidia-utils")
	}
	if gpu.HasAMD {
		deps = append(deps, "vulkan-radeon", "lib32-vulkan-radeon")
	}
	
	// Check which are missing
	var missing []string
	for _, dep := range deps {
		if !packageInstalled(dep) {
			missing = append(missing, dep)
		}
	}
	
	if len(missing) == 0 {
		fmt.Println("✓ All required dependencies installed")
		return nil
	}
	
	fmt.Printf("\nMissing required packages (%d):\n", len(missing))
	for _, dep := range missing {
		fmt.Printf("  • %s\n", dep)
	}
	
	log.Infof("Missing %d required packages", len(missing))
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would install missing packages")
		return nil
	}
	
	if !opts.NonInteractive {
		if !confirm("Install missing packages?") {
			return fmt.Errorf("dependencies required")
		}
	}
	
	args := append([]string{"pacman", "-S", "--needed", "--noconfirm"}, missing...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installOptionalPackages(opts Options) error {
	optional := []string{
		"gamemode",
		"lib32-gamemode",
		"mangohud",
		"lib32-mangohud",
	}
	
	var missing []string
	for _, pkg := range optional {
		if !packageInstalled(pkg) {
			missing = append(missing, pkg)
		}
	}
	
	if len(missing) == 0 {
		fmt.Println("✓ Optional packages already installed")
		return nil
	}
	
	fmt.Println("\nOptional packages (recommended for best performance):")
	for _, pkg := range missing {
		desc := getPackageDescription(pkg)
		fmt.Printf("  • %s - %s\n", pkg, desc)
	}
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would install optional packages")
		return nil
	}
	
	if !opts.NonInteractive {
		if !confirm("Install optional packages?") {
			return nil // Not an error to skip
		}
	}
	
	args := append([]string{"pacman", "-S", "--needed", "--noconfirm"}, missing...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getPackageDescription(pkg string) string {
	switch pkg {
	case "gamemode":
		return "CPU/GPU performance optimizations"
	case "lib32-gamemode":
		return "32-bit gamemode support"
	case "mangohud":
		return "Performance overlay (FPS, temps)"
	case "lib32-mangohud":
		return "32-bit mangohud support"
	default:
		return ""
	}
}

func checkUserGroups(opts Options) error {
	// Get current groups
	out, err := exec.Command("groups").Output()
	if err != nil {
		return err
	}
	
	groups := string(out)
	var missing []string
	
	if !strings.Contains(groups, "video") {
		missing = append(missing, "video")
	}
	if !strings.Contains(groups, "input") {
		missing = append(missing, "input")
	}
	
	if len(missing) == 0 {
		fmt.Println("✓ User groups: OK")
		return nil
	}
	
	fmt.Printf("⚠ User missing from groups: %s\n", strings.Join(missing, ", "))
	log.Warnf("User missing from groups: %v", missing)
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would add user to groups")
		return nil
	}
	
	if !opts.NonInteractive {
		if !confirm("Add user to groups?") {
			return nil
		}
	}
	
	user := os.Getenv("USER")
	groupsCSV := strings.Join(missing, ",")
	
	cmd := exec.Command("sudo", "usermod", "-aG", groupsCSV, user)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add user to groups: %v", err)
	}
	
	fmt.Println("⚠ Log out and back in for group changes to take effect")
	log.Info("User added to groups, logout required")
	return nil
}

func grantGamescopeCap(opts Options) error {
	gamescopePath, err := exec.LookPath("gamescope")
	if err != nil {
		return nil // Not an error if gamescope not found
	}
	
	// Check if already has capability
	out, err := exec.Command("getcap", gamescopePath).Output()
	if err == nil && strings.Contains(string(out), "cap_sys_nice") {
		fmt.Println("✓ Gamescope has cap_sys_nice capability")
		return nil
	}
	
	fmt.Println("Gamescope can run with real-time priority if granted cap_sys_nice")
	fmt.Println("  This improves frame pacing and reduces input latency")
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would grant cap_sys_nice to gamescope")
		return nil
	}
	
	if !opts.NonInteractive {
		if !confirm("Grant cap_sys_nice to gamescope?") {
			return nil
		}
	}
	
	cmd := exec.Command("sudo", "setcap", "cap_sys_nice+ep", gamescopePath)
	return cmd.Run()
}

func configureKeybindings(opts Options) error {
	home, _ := os.UserHomeDir()
	
	// Find bindings config
	bindingsPaths := []string{
		filepath.Join(home, ".config", "hypr", "bindings.conf"),
		filepath.Join(home, ".config", "hypr", "keybinds.conf"),
		filepath.Join(home, ".config", "hypr", "hyprland.conf"),
	}
	
	var bindingsFile string
	for _, path := range bindingsPaths {
		if _, err := os.Stat(path); err == nil {
			bindingsFile = path
			break
		}
	}
	
	if bindingsFile == "" {
		return fmt.Errorf("could not find Hyprland bindings config")
	}
	
	fmt.Printf("Using bindings config: %s\n", bindingsFile)
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would add keybindings to config")
		return nil
	}
	
	// Read current config
	data, err := os.ReadFile(bindingsFile)
	if err != nil {
		return err
	}
	
	content := string(data)
	
	// Remove old wizado bindings
	if strings.Contains(content, "# Wizado - added by wizado") {
		// Find and remove the block
		startMarker := "# Wizado - added by wizado"
		endMarker := "# End Wizado bindings"
		
		startIdx := strings.Index(content, startMarker)
		endIdx := strings.Index(content, endMarker)
		
		if startIdx != -1 && endIdx != -1 {
			content = content[:startIdx] + content[endIdx+len(endMarker):]
		}
	}
	
	// Detect bind style (bind vs bindd)
	bindStyle := "bindd"
	if !strings.Contains(content, "bindd") && strings.Contains(content, "bind =") {
		bindStyle = "bind"
	}
	
	// Add new bindings
	bindings := fmt.Sprintf(`

# Wizado - added by wizado
# Opens Wizado TUI menu on workspace 10
`)
	
	if bindStyle == "bindd" {
		bindings += `bindd = SUPER SHIFT, S, Wizado Menu, exec, wizado-menu-float
bindd = SUPER SHIFT, Q, Kill Steam, exec, pkill -9 steam; pkill -9 gamescope
`
	} else {
		bindings += `bind = SUPER SHIFT, S, exec, wizado-menu-float
bind = SUPER SHIFT, Q, exec, pkill -9 steam; pkill -9 gamescope
`
	}
	bindings += "# End Wizado bindings\n"
	
	content += bindings
	
	// Write back
	if err := os.WriteFile(bindingsFile, []byte(content), 0644); err != nil {
		return err
	}
	
	// Reload Hyprland
	exec.Command("hyprctl", "reload").Run()
	
	fmt.Println("✓ Keybindings added: Super+Shift+S (menu), Super+Shift+Q (kill)")
	log.Info("Keybindings configured")
	return nil
}

func configureWaybar(opts Options) error {
	home, _ := os.UserHomeDir()
	waybarDir := filepath.Join(home, ".config", "waybar")
	
	// Find waybar config
	configPath := filepath.Join(waybarDir, "config.jsonc")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(waybarDir, "config")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("waybar config not found")
		}
	}
	
	fmt.Printf("Using waybar config: %s\n", configPath)
	
	if opts.DryRun {
		fmt.Println("[DRY RUN] Would add wizado module to waybar")
		return nil
	}
	
	// Read config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	
	content := string(data)
	
	// Check if module already exists
	if strings.Contains(content, `"custom/wizado"`) {
		fmt.Println("✓ Wizado module already exists in waybar config")
		return nil
	}
	
	// Try to add module using jq
	// Note: on-click uses wizado-menu-float to spawn a terminal for the TUI
	moduleJSON := `{
    "custom/wizado": {
        "format": "{}",
        "return-type": "json",
        "exec": "wizado status",
        "on-click": "wizado-menu-float",
        "on-click-right": "wizado-menu-float",
        "interval": 60,
        "tooltip": true
    }
}`
	
	// Try jq approach
	if _, err := exec.LookPath("jq"); err == nil {
		// First add to modules-right
		cmd := exec.Command("jq", `if .["modules-right"] then .["modules-right"] = ["custom/wizado"] + .["modules-right"] else . end`, configPath)
		out, err := cmd.Output()
		if err == nil {
			// Then add the module definition
			var config map[string]interface{}
			if err := json.Unmarshal(out, &config); err == nil {
				config["custom/wizado"] = map[string]interface{}{
					"format":         "{}",
					"return-type":    "json",
					"exec":           "wizado status",
					"on-click":       "wizado-menu-float",
					"on-click-right": "wizado-menu-float",
					"interval":       60,
					"tooltip":        true,
				}
				
				newData, err := json.MarshalIndent(config, "", "  ")
				if err == nil {
					os.WriteFile(configPath, newData, 0644)
					fmt.Println("✓ Added wizado module to waybar config")
					
					// Restart waybar
					exec.Command("pkill", "waybar").Run()
					go exec.Command("waybar").Start()
					
					log.Info("Waybar module configured")
					return nil
				}
			}
		}
	}
	
	// Fallback: print instructions
	fmt.Println("Could not automatically add waybar module.")
	fmt.Println("Add the following to your waybar config:")
	fmt.Println(moduleJSON)
	
	return nil
}

func printSuccess(gpu GPUInfo, sysInfo *sysinfo.SystemInfo) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("  INSTALLATION COMPLETE")
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  Hardware detected:")
	if gpu.HasNVIDIA {
		fmt.Printf("    • NVIDIA GPU: %s\n", gpu.NVIDIAVkID)
	}
	if gpu.HasAMD {
		fmt.Println("    • AMD GPU")
	}
	if gpu.HasIntel {
		fmt.Println("    • Intel GPU")
	}
	
	fmt.Println()
	fmt.Println("  Performance features:")
	if sysInfo.Dependencies.GameMode.Installed {
		fmt.Println("    ✓ GameMode - CPU/GPU optimizations")
	}
	if sysInfo.Dependencies.MangoHUD.Installed {
		fmt.Println("    ✓ MangoHUD - Performance overlay")
	}
	if sysInfo.Dependencies.Gamescope.Installed {
		fmt.Println("    ✓ Gamescope - Gaming compositor")
	}
	
	fmt.Println()
	fmt.Println("  Input devices:")
	if sysInfo.Input.HasKeyboard {
		fmt.Println("    ✓ Keyboard")
	}
	if sysInfo.Input.HasMouse {
		fmt.Println("    ✓ Mouse")
	}
	if sysInfo.Input.HasController {
		fmt.Println("    ✓ Controller")
	}
	
	fmt.Println()
	fmt.Println("  Keybindings:")
	fmt.Println("    Super + Shift + S    Open Wizado Menu")
	fmt.Println("    Super + Shift + Q    Force-quit Steam")
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    wizado               Open TUI menu (launch Steam from there)")
	fmt.Println("    wizado config        Configure settings & license via TUI")
	fmt.Println("    wizado setup         Run this setup again")
	fmt.Println("    wizado info          Display system information")
	fmt.Println()
	fmt.Println("  License:")
	fmt.Println("    A valid license is required to use wizado.")
	fmt.Println("    Get one at: https://wizado.app ($10 for 5 machines)")
	fmt.Println()
}

func packageInstalled(name string) bool {
	cmd := exec.Command("pacman", "-Qi", name)
	return cmd.Run() == nil
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
