// Package launcher handles Steam and gamescope launching
package launcher

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wattfource/wizado/internal/config"
)

// GPU detection result
type GPUInfo struct {
	HasNVIDIA    bool
	HasAMD       bool
	HasIntel     bool
	NVIDIAVkID   string
}

// Launch starts Steam with gamescope
func Launch(cfg *config.Config) error {
	// Verify we're in Wayland
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("must run from Hyprland (no WAYLAND_DISPLAY)")
	}
	
	// Check requirements
	if _, err := exec.LookPath("steam"); err != nil {
		return fmt.Errorf("steam not installed")
	}
	if _, err := exec.LookPath("gamescope"); err != nil {
		return fmt.Errorf("gamescope not installed")
	}
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return fmt.Errorf("hyprctl not found")
	}
	
	// Detect GPU
	gpu := detectGPU()
	
	// Get resolution
	width, height := getResolution(cfg)
	
	// Build gamescope args
	gsArgs := buildGamescopeArgs(cfg, gpu, width, height)
	
	// Kill existing Steam
	killSteam()
	
	// Stop hypridle if running
	hypridleWasRunning := stopHypridle()
	
	// Save current workspace
	originalWorkspace := getCurrentWorkspace()
	
	// Find target workspace
	targetWorkspace := findEmptyWorkspace(cfg.Workspace)
	
	// Set up environment
	env := setupEnvironment(cfg, gpu)
	
	// Switch to gaming workspace
	switchWorkspace(targetWorkspace)
	
	// Build full command
	// Use gamepadui for Steam Deck-like experience, tenfoot for Big Picture
	steamUI := cfg.SteamUI
	if steamUI == "" {
		steamUI = "gamepadui"
	}
	steamArgs := []string{"-" + steamUI, "-steamos3", "-steampal", "-steamdeck"}
	fullArgs := append(gsArgs, "--")
	fullArgs = append(fullArgs, "steam")
	fullArgs = append(fullArgs, steamArgs...)
	
	// Log what we're launching
	logFile := createLogFile()
	if logFile != nil {
		fmt.Fprintf(logFile, "Launching: gamescope %s\n", strings.Join(fullArgs, " "))
		fmt.Fprintf(logFile, "Resolution: %dx%d\n", width, height)
		fmt.Fprintf(logFile, "GPU: NVIDIA=%v AMD=%v Intel=%v\n", gpu.HasNVIDIA, gpu.HasAMD, gpu.HasIntel)
	}
	
	// Launch gamescope + Steam
	cmd := exec.Command("gamescope", fullArgs...)
	cmd.Env = env
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	err := cmd.Run()
	
	// Cleanup
	switchWorkspace(originalWorkspace)
	if hypridleWasRunning {
		startHypridle()
	}
	
	if logFile != nil {
		logFile.Close()
	}
	
	return err
}

func detectGPU() GPUInfo {
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
					// Extract [10de:XXXX]
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

func getResolution(cfg *config.Config) (int, int) {
	if cfg.Resolution != "auto" {
		parts := strings.Split(cfg.Resolution, "x")
		if len(parts) == 2 {
			w, _ := strconv.Atoi(parts[0])
			h, _ := strconv.Atoi(parts[1])
			if w > 0 && h > 0 {
				return w, h
			}
		}
	}
	
	// Auto-detect from Hyprland
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err != nil {
		return 2560, 1440 // Default fallback
	}
	
	var monitors []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	
	if err := json.Unmarshal(out, &monitors); err != nil || len(monitors) == 0 {
		return 2560, 1440
	}
	
	return monitors[0].Width, monitors[0].Height
}

func buildGamescopeArgs(cfg *config.Config, gpu GPUInfo, width, height int) []string {
	args := []string{
		"-W", strconv.Itoa(width),
		"-H", strconv.Itoa(height),
		"-f", // Fullscreen
		"-e", // Expose wayland
		"--grab",
		"--force-windows-fullscreen",
		"--disable-color-management",
	}
	
	// FSR scaling
	if cfg.FSR != "off" {
		scales := config.FSRScales()
		if scale, ok := scales[cfg.FSR]; ok {
			innerW := int(math.Round(float64(width) * scale))
			innerH := int(math.Round(float64(height) * scale))
			args = append(args, "-w", strconv.Itoa(innerW), "-h", strconv.Itoa(innerH))
			args = append(args, "--filter", "fsr")
		} else {
			args = append(args, "-w", strconv.Itoa(width), "-h", strconv.Itoa(height))
		}
	} else {
		args = append(args, "-w", strconv.Itoa(width), "-h", strconv.Itoa(height))
	}
	
	// Frame limit
	if cfg.FrameLimit > 0 {
		args = append(args, "--framerate-limit", strconv.Itoa(cfg.FrameLimit))
	}
	
	// VRR
	if cfg.VRR {
		args = append(args, "--adaptive-sync")
	}
	
	// NVIDIA preference
	if gpu.HasNVIDIA && gpu.NVIDIAVkID != "" {
		args = append(args, "--prefer-vk-device", gpu.NVIDIAVkID)
	}
	
	return args
}

func setupEnvironment(cfg *config.Config, gpu GPUInfo) []string {
	env := os.Environ()
	
	if cfg.MangoHUD {
		env = append(env, "MANGOHUD=1")
	} else {
		env = append(env, "MANGOHUD=0")
	}
	
	env = append(env, "SDL_VIDEO_MINIMIZE_ON_FOCUS_LOSS=0")
	
	// Steam-specific environment
	env = append(env, "STEAM_RUNTIME_PREFER_HOST_LIBRARIES=0")
	env = append(env, "STEAM_FORCE_DESKTOPUI_SCALING=1")
	
	// Gamescope integration
	env = append(env, "STEAM_GAMESCOPE_NIS_SUPPORTED=1")
	env = append(env, "STEAM_GAMESCOPE_HAS_TEARING_SUPPORT=1")
	env = append(env, "STEAM_GAMESCOPE_TEARING_SUPPORTED=1")
	env = append(env, "STEAM_GAMESCOPE_VRR_SUPPORTED=1")
	env = append(env, "STEAM_DISPLAY_REFRESH_LIMITS=60,72,120,144")
	
	if gpu.HasNVIDIA {
		env = append(env, "VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json")
		env = append(env, "__GLX_VENDOR_LIBRARY_NAME=nvidia")
		env = append(env, "__GL_SHADER_DISK_CACHE=1")
		env = append(env, "WLR_NO_HARDWARE_CURSORS=1")
		env = append(env, "XCURSOR_SIZE=24")
		// NVIDIA-specific for gamescope
		env = append(env, "__GL_GSYNC_ALLOWED=1")
		env = append(env, "__GL_VRR_ALLOWED=1")
	}
	
	return env
}

func killSteam() {
	exec.Command("pkill", "-9", "steam").Run()
	exec.Command("pkill", "-9", "steamwebhelper").Run()
	// Give time for processes to fully terminate
	time.Sleep(time.Second)
}

func stopHypridle() bool {
	out, _ := exec.Command("pgrep", "-x", "hypridle").Output()
	if len(out) > 0 {
		exec.Command("pkill", "hypridle").Run()
		return true
	}
	return false
}

func startHypridle() {
	cmd := exec.Command("hypridle")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Start()
}

func getCurrentWorkspace() int {
	out, err := exec.Command("hyprctl", "activeworkspace", "-j").Output()
	if err != nil {
		return 1
	}
	
	var workspace struct {
		ID int `json:"id"`
	}
	
	if err := json.Unmarshal(out, &workspace); err != nil {
		return 1
	}
	
	return workspace.ID
}

func findEmptyWorkspace(preferred int) int {
	out, err := exec.Command("hyprctl", "workspaces", "-j").Output()
	if err != nil {
		return preferred
	}
	
	var workspaces []struct {
		ID int `json:"id"`
	}
	
	if err := json.Unmarshal(out, &workspaces); err != nil {
		return preferred
	}
	
	usedWorkspaces := make(map[int]bool)
	for _, ws := range workspaces {
		usedWorkspaces[ws.ID] = true
	}
	
	// Check if preferred is empty
	if !usedWorkspaces[preferred] {
		return preferred
	}
	
	// Find first empty between 1-10
	for i := 1; i <= 10; i++ {
		if !usedWorkspaces[i] {
			return i
		}
	}
	
	return preferred
}

func switchWorkspace(ws int) {
	exec.Command("hyprctl", "dispatch", "workspace", strconv.Itoa(ws)).Run()
}

func createLogFile() *os.File {
	home, _ := os.UserHomeDir()
	stateDir := filepath.Join(home, ".cache", "wizado")
	os.MkdirAll(stateDir, 0755)
	
	logPath := filepath.Join(stateDir, "wizado.log")
	file, _ := os.Create(logPath)
	return file
}

