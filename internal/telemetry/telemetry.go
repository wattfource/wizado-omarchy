// Package telemetry provides anonymous usage data collection for wizado
// Phase 1: Local storage only - data is stored but not sent
// Phase 2: Opt-in remote reporting
package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wattfource/wizado/internal/sysinfo"
)

// Event types
const (
	EventLaunch       = "launch"
	EventExit         = "exit"
	EventError        = "error"
	EventConfigChange = "config_change"
	EventSetup        = "setup"
)

// Event represents a telemetry event
type Event struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Timestamp   time.Time      `json:"timestamp"`
	MachineHash string         `json:"machine_hash"` // Anonymized machine ID
	Version     string         `json:"version"`
	Data        map[string]any `json:"data,omitempty"`
}

// SessionData captures information about a gaming session
type SessionData struct {
	SessionID   string        `json:"session_id"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration_seconds,omitempty"`
	ExitCode    int           `json:"exit_code,omitempty"`
	ExitReason  string        `json:"exit_reason,omitempty"`
	
	// Configuration used
	Resolution  string `json:"resolution"`
	FSR         string `json:"fsr"`
	FrameLimit  int    `json:"frame_limit"`
	VRR         bool   `json:"vrr"`
	MangoHUD    bool   `json:"mangohud"`
	GameMode    bool   `json:"gamemode"`
	SteamUI     string `json:"steam_ui"`
}

// SystemSnapshot captures system info at a point in time
type SystemSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	MachineHash string    `json:"machine_hash"`
	
	// Hardware summary (anonymized)
	GPUType     string `json:"gpu_type"`    // "nvidia", "amd", "intel"
	GPUDriver   string `json:"gpu_driver,omitempty"`
	CPUCores    int    `json:"cpu_cores"`
	RAMGiB      int    `json:"ram_gib"`
	
	// Display
	ResolutionW int     `json:"resolution_w"`
	ResolutionH int     `json:"resolution_h"`
	RefreshHz   float64 `json:"refresh_hz"`
	
	// Input
	HasKeyboard   bool `json:"has_keyboard"`
	HasMouse      bool `json:"has_mouse"`
	HasController bool `json:"has_controller"`
	
	// Network
	ConnectionType string `json:"connection_type"`
	
	// Software
	OSName       string `json:"os_name"`
	HyprVersion  string `json:"hypr_version"`
	SteamVersion string `json:"steam_version,omitempty"`
	
	// Dependencies
	HasGamescope bool `json:"has_gamescope"`
	HasGamemode  bool `json:"has_gamemode"`
	HasMangohud  bool `json:"has_mangohud"`
}

// Store handles telemetry storage
type Store struct {
	mu       sync.Mutex
	dataDir  string
	enabled  bool
	version  string
	machineHash string
}

// Config for telemetry
type Config struct {
	Enabled bool   // Whether telemetry collection is enabled
	DataDir string // Directory to store telemetry data
	Version string // Wizado version
}

// DefaultConfig returns default telemetry configuration
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Enabled: true, // Collect locally by default
		DataDir: filepath.Join(home, ".local", "share", "wizado", "telemetry"),
		Version: "dev",
	}
}

var (
	defaultStore *Store
	once         sync.Once
)

// Init initializes the telemetry store
func Init(cfg Config) error {
	var initErr error
	once.Do(func() {
		defaultStore, initErr = NewStore(cfg)
	})
	return initErr
}

// Default returns the default store
func Default() *Store {
	if defaultStore == nil {
		Init(DefaultConfig())
	}
	return defaultStore
}

// NewStore creates a new telemetry store
func NewStore(cfg Config) (*Store, error) {
	s := &Store{
		dataDir: cfg.DataDir,
		enabled: cfg.Enabled,
		version: cfg.Version,
	}
	
	if cfg.Enabled {
		if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
			return nil, err
		}
	}
	
	// Generate anonymized machine hash
	s.machineHash = generateMachineHash()
	
	return s, nil
}

// generateMachineHash creates an anonymized machine identifier
// Uses the same sources as license machine ID but hashed differently
// so it cannot be correlated with the license
func generateMachineHash() string {
	// Read machine-id
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "unknown"
	}
	
	// Hash with a telemetry-specific salt
	combined := "wizado-telemetry-v1:" + string(data)
	hash := sha256.Sum256([]byte(combined))
	
	// Return first 16 chars for brevity
	return hex.EncodeToString(hash[:8])
}

// RecordEvent records a telemetry event
func (s *Store) RecordEvent(eventType string, data map[string]any) error {
	if !s.enabled {
		return nil
	}
	
	event := Event{
		ID:          generateEventID(),
		Type:        eventType,
		Timestamp:   time.Now().UTC(),
		MachineHash: s.machineHash,
		Version:     s.version,
		Data:        data,
	}
	
	return s.writeEvent(event)
}

// RecordLaunch records a Steam launch event
func (s *Store) RecordLaunch(session *SessionData) error {
	return s.RecordEvent(EventLaunch, map[string]any{
		"session":    session,
	})
}

// RecordExit records a session exit
func (s *Store) RecordExit(session *SessionData) error {
	return s.RecordEvent(EventExit, map[string]any{
		"session": session,
	})
}

// RecordError records an error event
func (s *Store) RecordError(component, message string, details map[string]any) error {
	data := map[string]any{
		"component": component,
		"message":   message,
	}
	for k, v := range details {
		data[k] = v
	}
	return s.RecordEvent(EventError, data)
}

// RecordSystemSnapshot captures and stores system information
func (s *Store) RecordSystemSnapshot(sysInfo *sysinfo.SystemInfo) error {
	if !s.enabled || sysInfo == nil {
		return nil
	}
	
	snapshot := SystemSnapshot{
		Timestamp:   time.Now().UTC(),
		MachineHash: s.machineHash,
		
		// Hardware
		CPUCores: sysInfo.CPU.Cores,
		RAMGiB:   sysInfo.Memory.TotalMiB / 1024,
		
		// GPU
		GPUDriver: sysInfo.GPU.DriverVersion,
		
		// Display
		ResolutionW: sysInfo.Display.Primary.Width,
		ResolutionH: sysInfo.Display.Primary.Height,
		RefreshHz:   sysInfo.Display.Primary.RefreshHz,
		
		// Input
		HasKeyboard:   sysInfo.Input.HasKeyboard,
		HasMouse:      sysInfo.Input.HasMouse,
		HasController: sysInfo.Input.HasController,
		
		// Network
		ConnectionType: sysInfo.Network.ConnectionType,
		
		// Software
		OSName:      sysInfo.OS.Name,
		HyprVersion: sysInfo.Desktop.Version,
		
		// Dependencies
		HasGamescope: sysInfo.Dependencies.Gamescope.Installed,
		HasGamemode:  sysInfo.Dependencies.GameMode.Installed,
		HasMangohud:  sysInfo.Dependencies.MangoHUD.Installed,
	}
	
	// Determine GPU type
	if sysInfo.GPU.HasNVIDIA {
		snapshot.GPUType = "nvidia"
	} else if sysInfo.GPU.HasAMD {
		snapshot.GPUType = "amd"
	} else if sysInfo.GPU.HasIntel {
		snapshot.GPUType = "intel"
	}
	
	if sysInfo.Dependencies.Steam.Version != "" {
		snapshot.SteamVersion = sysInfo.Dependencies.Steam.Version
	}
	
	return s.writeSnapshot(snapshot)
}

func (s *Store) writeEvent(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Store events in a daily file
	date := event.Timestamp.Format("2006-01-02")
	filename := filepath.Join(s.dataDir, "events", date+".jsonl")
	
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	_, err = f.WriteString(string(data) + "\n")
	return err
}

func (s *Store) writeSnapshot(snapshot SystemSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	filename := filepath.Join(s.dataDir, "snapshots", "system.json")
	
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filename, data, 0600)
}

func generateEventID() string {
	// Simple timestamp-based ID
	now := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(string(rune(now))))
	return hex.EncodeToString(hash[:8])
}

// GetStats returns summary statistics from collected telemetry
func (s *Store) GetStats() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	stats := map[string]any{
		"enabled":      s.enabled,
		"machine_hash": s.machineHash,
		"data_dir":     s.dataDir,
	}
	
	// Count events
	eventsDir := filepath.Join(s.dataDir, "events")
	eventCount := 0
	
	files, _ := os.ReadDir(eventsDir)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".jsonl" {
			// Count lines in file
			data, err := os.ReadFile(filepath.Join(eventsDir, f.Name()))
			if err == nil {
				for _, b := range data {
					if b == '\n' {
						eventCount++
					}
				}
			}
		}
	}
	
	stats["event_count"] = eventCount
	stats["event_files"] = len(files)
	
	// Check for snapshot
	snapshotPath := filepath.Join(s.dataDir, "snapshots", "system.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		stats["has_snapshot"] = true
	} else {
		stats["has_snapshot"] = false
	}
	
	return stats, nil
}

// Enable enables telemetry collection
func (s *Store) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = true
}

// Disable disables telemetry collection
func (s *Store) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
}

// IsEnabled returns whether telemetry is enabled
func (s *Store) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// ClearData removes all telemetry data
func (s *Store) ClearData() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return os.RemoveAll(s.dataDir)
}

// Global convenience functions

// RecordEvent records a telemetry event
func RecordEvent(eventType string, data map[string]any) error {
	return Default().RecordEvent(eventType, data)
}

// RecordLaunch records a Steam launch event
func RecordLaunch(session *SessionData) error {
	return Default().RecordLaunch(session)
}

// RecordExit records a session exit
func RecordExit(session *SessionData) error {
	return Default().RecordExit(session)
}

// RecordError records an error event
func RecordError(component, message string, details map[string]any) error {
	return Default().RecordError(component, message, details)
}

// RecordSystemSnapshot captures and stores system information
func RecordSystemSnapshot(sysInfo *sysinfo.SystemInfo) error {
	return Default().RecordSystemSnapshot(sysInfo)
}

