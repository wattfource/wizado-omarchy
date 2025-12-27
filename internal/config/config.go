// Package config handles wizado configuration
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds wizado settings
type Config struct {
	Resolution string // "auto" or "WIDTHxHEIGHT"
	FSR        string // "off", "ultra", "quality", "balanced", "performance"
	FrameLimit int    // 0 = unlimited
	VRR        bool   // Variable refresh rate
	MangoHUD   bool   // Performance overlay
	SteamUI    string // "gamepadui" or "tenfoot"
	Workspace  int    // Hyprland workspace number
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Resolution: "auto",
		FSR:        "off",
		FrameLimit: 0,
		VRR:        false,
		MangoHUD:   false,
		SteamUI:    "tenfoot",
		Workspace:  10,
	}
}

// Paths returns the config directory and file paths
func Paths() (dir string, file string) {
	home, _ := os.UserHomeDir()
	dir = filepath.Join(home, ".config", "wizado")
	file = filepath.Join(dir, "config")
	return
}

// Load reads the configuration from disk
func Load() (*Config, error) {
	cfg := Default()
	_, configFile := Paths()
	
	file, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return defaults if no config
		}
		return nil, err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		switch key {
		case "WIZADO_RESOLUTION":
			cfg.Resolution = value
		case "WIZADO_FSR":
			cfg.FSR = value
		case "WIZADO_FRAMELIMIT":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.FrameLimit = v
			}
		case "WIZADO_VRR":
			cfg.VRR = value == "on"
		case "WIZADO_MANGOHUD":
			cfg.MangoHUD = value == "on"
		case "WIZADO_STEAM_UI":
			cfg.SteamUI = value
		case "WIZADO_WORKSPACE":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.Workspace = v
			}
		}
	}
	
	return cfg, scanner.Err()
}

// Save writes the configuration to disk
func Save(cfg *Config) error {
	dir, configFile := Paths()
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	vrr := "off"
	if cfg.VRR {
		vrr = "on"
	}
	
	mangohud := "off"
	if cfg.MangoHUD {
		mangohud = "on"
	}
	
	content := fmt.Sprintf(`WIZADO_RESOLUTION=%s
WIZADO_FSR=%s
WIZADO_FRAMELIMIT=%d
WIZADO_VRR=%s
WIZADO_MANGOHUD=%s
WIZADO_STEAM_UI=%s
WIZADO_WORKSPACE=%d
`,
		cfg.Resolution,
		cfg.FSR,
		cfg.FrameLimit,
		vrr,
		mangohud,
		cfg.SteamUI,
		cfg.Workspace,
	)
	
	return os.WriteFile(configFile, []byte(content), 0644)
}

// FSRScales returns the scaling factor for each FSR mode
func FSRScales() map[string]float64 {
	return map[string]float64{
		"ultra":       0.77,
		"quality":     0.67,
		"balanced":    0.59,
		"performance": 0.5,
	}
}

// FSROptions returns available FSR options
func FSROptions() []string {
	return []string{"off", "ultra", "quality", "balanced", "performance"}
}

// FrameLimitOptions returns available frame limit options
func FrameLimitOptions() []int {
	return []int{0, 30, 60, 90, 120, 144, 165, 240}
}

// SteamUIOptions returns available Steam UI options
func SteamUIOptions() []string {
	return []string{"gamepadui", "tenfoot"}
}

// WorkspaceOptions returns available workspace options
func WorkspaceOptions() []int {
	return []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
}

