// Package launcher handles Steam and gamescope launching
package launcher

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/wattfource/wizado/internal/logging"
	"github.com/wattfource/wizado/internal/sysinfo"
	"github.com/wattfource/wizado/internal/telemetry"
)

// GPU detection result
type GPUInfo struct {
	HasNVIDIA    bool
	HasAMD       bool
	HasIntel     bool
	NVIDIAVkID   string
}

// SessionInfo tracks the current gaming session
type SessionInfo struct {
	ID             string
	StartTime      time.Time
	Config         *config.Config
	GPU            GPUInfo
	Width          int
	Height         int
	GameModeActive bool
}

var log *logging.Logger

func init() {
	log = logging.WithComponent("launcher")
}

// Launch starts Steam with gamescope
func Launch(cfg *config.Config) error {
	// Initialize session
	session := &SessionInfo{
		ID:        generateSessionID(),
		StartTime: time.Now(),
		Config:    cfg,
	}
	
	log.Infof("Starting gaming session %s", session.ID)
	
	// Verify we're in Wayland
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		err := fmt.Errorf("must run from Hyprland (no WAYLAND_DISPLAY)")
		log.Error(err.Error())
		return err
	}
	
	// Check requirements
	if err := checkRequirements(); err != nil {
		log.Errorf("Requirements check failed: %v", err)
		return err
	}
	
	// Detect GPU
	session.GPU = detectGPU()
	log.Infof("GPU detected: NVIDIA=%v AMD=%v Intel=%v", session.GPU.HasNVIDIA, session.GPU.HasAMD, session.GPU.HasIntel)
	
	// Get resolution
	session.Width, session.Height = getResolution(cfg)
	log.Infof("Using resolution: %dx%d", session.Width, session.Height)
	
	// Build gamescope args
	gsArgs := buildGamescopeArgs(cfg, session.GPU, session.Width, session.Height)
	
	// Kill existing Steam
	log.Debug("Killing any existing Steam processes")
	killSteam()
	
	// Stop hypridle if running
	hypridleWasRunning := stopHypridle()
	if hypridleWasRunning {
		log.Debug("Stopped hypridle")
	}
	
	// Save current workspace
	originalWorkspace := getCurrentWorkspace()
	
	// Find target workspace
	targetWorkspace := findEmptyWorkspace(cfg.Workspace)
	log.Infof("Using workspace %d (original: %d)", targetWorkspace, originalWorkspace)
	
	// Set up environment
	env := setupEnvironment(cfg, session.GPU)
	
	// Start GameMode if available
	session.GameModeActive = startGameMode()
	if session.GameModeActive {
		log.Info("GameMode activated")
	}
	
	// Switch to gaming workspace
	switchWorkspace(targetWorkspace)
	
	// Build full command
	steamUI := cfg.SteamUI
	if steamUI == "" {
		steamUI = "gamepadui"
	}
	steamArgs := []string{"-" + steamUI, "-steamos3", "-steampal", "-steamdeck"}
	fullArgs := append(gsArgs, "--")
	fullArgs = append(fullArgs, "steam")
	fullArgs = append(fullArgs, steamArgs...)
	
	// Create log file for this session
	logFile := createLogFile(session.ID)
	if logFile != nil {
		fmt.Fprintf(logFile, "═══════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(logFile, "  Wizado Gaming Session: %s\n", session.ID)
		fmt.Fprintf(logFile, "  Started: %s\n", session.StartTime.Format(time.RFC3339))
		fmt.Fprintf(logFile, "═══════════════════════════════════════════════════════════════\n\n")
		fmt.Fprintf(logFile, "Command: gamescope %s\n", strings.Join(fullArgs, " "))
		fmt.Fprintf(logFile, "Resolution: %dx%d\n", session.Width, session.Height)
		fmt.Fprintf(logFile, "FSR: %s\n", cfg.FSR)
		fmt.Fprintf(logFile, "Frame Limit: %d\n", cfg.FrameLimit)
		fmt.Fprintf(logFile, "VRR: %v\n", cfg.VRR)
		fmt.Fprintf(logFile, "MangoHUD: %v\n", cfg.MangoHUD)
		fmt.Fprintf(logFile, "GameMode: %v\n", session.GameModeActive)
		fmt.Fprintf(logFile, "GPU: NVIDIA=%v AMD=%v Intel=%v\n", session.GPU.HasNVIDIA, session.GPU.HasAMD, session.GPU.HasIntel)
		fmt.Fprintf(logFile, "\n═══════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(logFile, "  Steam/Gamescope Output\n")
		fmt.Fprintf(logFile, "═══════════════════════════════════════════════════════════════\n\n")
	}
	
	log.Infof("Launching gamescope with args: %s", strings.Join(fullArgs, " "))
	
	// Record launch telemetry
	telemetrySession := &telemetry.SessionData{
		SessionID:  session.ID,
		StartTime:  session.StartTime,
		Resolution: fmt.Sprintf("%dx%d", session.Width, session.Height),
		FSR:        cfg.FSR,
		FrameLimit: cfg.FrameLimit,
		VRR:        cfg.VRR,
		MangoHUD:   cfg.MangoHUD,
		GameMode:   session.GameModeActive,
		SteamUI:    steamUI,
	}
	telemetry.RecordLaunch(telemetrySession)
	
	// Launch gamescope + Steam
	cmd := exec.Command("gamescope", fullArgs...)
	cmd.Env = env
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	err := cmd.Run()
	
	// Session ended
	endTime := time.Now()
	duration := endTime.Sub(session.StartTime)
	
	// Log session end
	if logFile != nil {
		fmt.Fprintf(logFile, "\n═══════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(logFile, "  Session Ended: %s\n", endTime.Format(time.RFC3339))
		fmt.Fprintf(logFile, "  Duration: %s\n", duration.Round(time.Second))
		if err != nil {
			fmt.Fprintf(logFile, "  Exit Error: %v\n", err)
		}
		fmt.Fprintf(logFile, "═══════════════════════════════════════════════════════════════\n")
		logFile.Close()
	}
	
	log.Infof("Gaming session ended after %s", duration.Round(time.Second))
	
	// Record exit telemetry
	telemetrySession.EndTime = endTime
	telemetrySession.Duration = duration
	if err != nil {
		telemetrySession.ExitReason = err.Error()
		telemetrySession.ExitCode = 1
	}
	telemetry.RecordExit(telemetrySession)
	
	// Cleanup
	log.Debug("Performing cleanup")
	
	// Stop GameMode
	if session.GameModeActive {
		stopGameMode()
		log.Debug("GameMode deactivated")
	}
	
	switchWorkspace(originalWorkspace)
	if hypridleWasRunning {
		startHypridle()
		log.Debug("Restarted hypridle")
	}
	
	return err
}

func generateSessionID() string {
	now := time.Now().UnixNano()
	data := fmt.Sprintf("wizado-session-%d", now)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

func checkRequirements() error {
	if _, err := exec.LookPath("steam"); err != nil {
		return fmt.Errorf("steam not installed")
	}
	if _, err := exec.LookPath("gamescope"); err != nil {
		return fmt.Errorf("gamescope not installed")
	}
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return fmt.Errorf("hyprctl not found")
	}
	return nil
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
	
	// Input handling - ensure keyboard/mouse work in gamescope
	env = append(env, "SDL_GAMECONTROLLERCONFIG=")
	
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

// startGameMode enables GameMode for performance optimization
func startGameMode() bool {
	// Check if gamemoded is available
	if _, err := exec.LookPath("gamemoded"); err != nil {
		log.Debug("gamemoded not found, skipping GameMode")
		return false
	}
	
	// Check if already running
	out, _ := exec.Command("gamemoded", "-s").Output()
	if strings.Contains(string(out), "active") {
		log.Debug("GameMode already active")
		return true
	}
	
	// Start gamemode request
	cmd := exec.Command("gamemoded", "-r")
	if err := cmd.Run(); err != nil {
		log.Warnf("Failed to request GameMode: %v", err)
		return false
	}
	
	// Verify it started
	time.Sleep(100 * time.Millisecond)
	out, _ = exec.Command("gamemoded", "-s").Output()
	return strings.Contains(string(out), "active")
}

// stopGameMode disables GameMode
func stopGameMode() {
	exec.Command("gamemoded", "-r").Run()
}

func killSteam() {
	// Kill existing gamescope first to avoid X socket conflicts
	exec.Command("pkill", "-9", "gamescope").Run()
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

func createLogFile(sessionID string) *os.File {
	home, _ := os.UserHomeDir()
	stateDir := filepath.Join(home, ".cache", "wizado", "sessions")
	os.MkdirAll(stateDir, 0755)
	
	// Use session ID in filename for easy identification
	logPath := filepath.Join(stateDir, fmt.Sprintf("session_%s.log", sessionID))
	file, _ := os.Create(logPath)
	
	// Also create/update a symlink to the latest session
	latestLink := filepath.Join(home, ".cache", "wizado", "latest-session.log")
	os.Remove(latestLink)
	os.Symlink(logPath, latestLink)
	
	return file
}

// CollectSystemInfo gathers and records system information before launch
func CollectSystemInfo(version string) *sysinfo.SystemInfo {
	info := sysinfo.Collect(version)
	
	// Record snapshot for telemetry
	telemetry.RecordSystemSnapshot(info)
	
	return info
}
